#!/usr/bin/env ts-node
/**
 * MVC Mainnet End-to-End Validation Script for Native Token / Token AMM (`contracts/tokenSwap/*`)
 *
 * Steps:
 *   1) mint-musd   : Mint a second standard MetaContract FT (MUSD, 8 decimals) with the exact same
 *                    dual-whitelist codeHash (f725449d154e41a996412433cc41148e4355c7cc) as MVST.
 *   2) deploy-pool : Deploy a fresh MVST / MUSD native Token/Token pool + initial dual-FT liquidity
 *                    using production `deployTokenSwapPoolOnChain`.
 *   3) e2e         : Execute full on-chain cycle using production `TokenSwapEngine` (WalletSigner):
 *                    - Swap MVST -> MUSD (token1ToToken2)
 *                    - Swap MUSD -> MVST (token2ToToken1)
 *                    - Subsequent Dual-FT AddLiquidity (11 inputs)
 *                    - RemoveLiquidity (10 inputs, receive both MVST & MUSD back)
 *
 * Safety: defaults to DRY mode (offline `.verify()` only, zero broadcast). Pass `--live` to broadcast.
 */
import * as fs from 'fs'
import * as path from 'path'
import {
  Common,
  ProtoHeader,
  SwapEngine,
  TokenProto,
  TokenSwapEngine,
  loadSwapContractsFast,
  loadTokenSwapContractsFast,
  loadVerifiedContractDesc,
  scrypt,
} from '../src/core-bridge'
import { deployTokenSwapPoolOnChain, inspectOnChainPoolAndBuildEntry } from '../src/pool-admin'

const { mvc, Bytes, PubKey, Ripemd160, Sig, SigHashPreimage, getPreimage, signTx, toHex } = scrypt

const STATE_FILE = path.resolve(__dirname, '../data/tokenswap-mainnet-validation.json')
const DEFAULT_KEY_PATH = '/Users/tusm/Documents/MetaID_Projects/mvcswap-core-mainnet/scripts/.keys/main.json'

const MVST_TOKEN_ID = '5ec98a884f16931190f2fad564da7ac0a4a18436'
const STANDARD_CODE_HASH = 'f725449d154e41a996412433cc41148e4355c7cc'

function loadKeyWif(): string {
  if (process.env.SWAP_ADMIN_WIF) return process.env.SWAP_ADMIN_WIF.trim()
  const keyFile = process.env.SWAP_ADMIN_KEY_FILE || DEFAULT_KEY_PATH
  const raw = JSON.parse(fs.readFileSync(keyFile, 'utf8'))
  if (!raw.wif) throw new Error(`Missing wif in ${keyFile}`)
  return raw.wif
}

function loadState(): Record<string, any> {
  if (!fs.existsSync(STATE_FILE)) return {}
  return JSON.parse(fs.readFileSync(STATE_FILE, 'utf8'))
}

function saveState(patch: Record<string, any>): Record<string, any> {
  const cur = loadState()
  const next = { ...cur, ...patch, updatedAt: new Date().toISOString() }
  fs.mkdirSync(path.dirname(STATE_FILE), { recursive: true })
  fs.writeFileSync(STATE_FILE, JSON.stringify(next, null, 2))
  return next
}

async function stepMintMusd(live: boolean) {
  const wif = loadKeyWif()
  const privKey = mvc.PrivateKey.fromWIF(wif)
  const pubKey = privKey.publicKey
  const addr = privKey.toAddress(mvc.Networks.livenet)
  const chain = new SwapEngine.MetaletChainClient({ net: 'livenet' })
  const base = loadSwapContractsFast()

  const Genesis = scrypt.buildContractClass(loadVerifiedContractDesc('token/tokenGenesis'))
  const Token = base.Token

  const TOKEN_NAME = Buffer.alloc(TokenProto.TOKEN_NAME_LEN, 0)
  TOKEN_NAME.write('MVCSwapValUSD')
  const TOKEN_SYMBOL = Buffer.alloc(TokenProto.TOKEN_SYMBOL_LEN, 0)
  TOKEN_SYMBOL.write('MUSD')
  const DECIMAL_NUM = Buffer.from('08', 'hex')
  const SUPPLY = BigInt(200_000_000_000) // 2000 MUSD (8 decimals)
  const buffValue = Common.getUInt64Buf(SUPPLY)
  const tokenVersion = Common.getUInt32Buf(1)
  const tokenType = Common.getUInt32Buf(1)
  const PROTO_FLAG = ProtoHeader.PROTO_FLAG

  const inputSatoshis = 10000
  const utxos = await chain.getSpaceUtxos(addr.toString())
  const largest = [...utxos].sort((a: any, b: any) => b.satoshis - a.satoshis)[0]
  if (!largest || largest.satoshis < 100000) {
    throw new Error(`Wallet ${addr.toString()} needs >= 100,000 sats`)
  }
  console.log(`[mint-musd] addr=${addr.toString()} largest=${largest.satoshis} sats live=${live}`)

  const genesis = new Genesis()
  const genData = Buffer.concat([
    TOKEN_NAME,
    TOKEN_SYMBOL,
    DECIMAL_NUM,
    addr.hashBuffer,
    Buffer.alloc(8, 0),
    Buffer.alloc(20, 0),
    Buffer.alloc(36, 0),
    tokenVersion,
    tokenType,
    PROTO_FLAG,
  ])
  genesis.setDataPart(Common.buildScriptData(genData).toString('hex'))

  // T1: deploy genesis
  const t1 = new mvc.Transaction()
  t1.version = Common.TX_VERSION
  const p2pkh = mvc.Script.buildPublicKeyHashOut(addr)
  t1.from({
    txId: largest.txid,
    outputIndex: largest.vout,
    script: p2pkh,
    satoshis: largest.satoshis,
  })
  t1.addOutput(new mvc.Transaction.Output({ script: genesis.lockingScript, satoshis: inputSatoshis }))
  const t1Fee = 3500
  const t1Change = largest.satoshis - inputSatoshis - t1Fee
  t1.addOutput(new mvc.Transaction.Output({ script: p2pkh, satoshis: t1Change }))
  Common.signP2PKH(t1, privKey, 0)
  const t1Hex = t1.uncheckedSerialize()
  console.log(`[mint-musd] T1 (genesis) txid=${t1.id} bytes=${t1Hex.length / 2}`)

  // T2: issue MUSD
  const genesisScriptBuf: Buffer = genesis.lockingScript.toBuffer()
  const genesisTxidBuf = Buffer.from(Common.genGenesisTxid(t1.id, 0), 'hex')
  const newGenBuf = TokenProto.getNewGenesisScript(genesisScriptBuf, genesisTxidBuf)
  const genesisHash = Common.getScriptHashBuf(newGenBuf)
  const tokenID = Common.hash160(Buffer.concat([genesisHash, genesisTxidBuf]))

  const tokenInst = new Token(base.transferCheckCodeHashArray, base.unlockCheckCodeHashArray)
  const tokData = Buffer.concat([
    TOKEN_NAME,
    TOKEN_SYMBOL,
    DECIMAL_NUM,
    addr.hashBuffer,
    buffValue,
    genesisHash,
    genesisTxidBuf,
    tokenVersion,
    tokenType,
    PROTO_FLAG,
  ])
  tokenInst.setDataPart(Common.buildScriptData(tokData).toString('hex'))
  const tokenScript = tokenInst.lockingScript
  const actualCodeHash = TokenProto.getScriptCodeHash(tokenScript.toBuffer()).toString('hex')
  if (actualCodeHash !== STANDARD_CODE_HASH) {
    throw new Error(`CodeHash mismatch: got ${actualCodeHash}, expected ${STANDARD_CODE_HASH}`)
  }

  const t2 = new mvc.Transaction()
  t2.version = Common.TX_VERSION
  const prevouts: Buffer[] = []
  Common.addInput(t2, t1.id, 0, genesis.lockingScript, inputSatoshis, prevouts)
  Common.addInput(t2, t1.id, 1, p2pkh, t1Change, prevouts, true)
  Common.addOutput(t2, mvc.Script.fromBuffer(newGenBuf), inputSatoshis)
  Common.addOutput(t2, tokenScript, inputSatoshis)
  const t2Fee = 14000
  const t2Change = t1Change - inputSatoshis - t2Fee
  Common.addOutput(t2, p2pkh, t2Change)

  const preimage = getPreimage(t2, genesis.lockingScript, inputSatoshis, 0, Common.SIG_HASH_ALL)
  const sig = signTx(t2, privKey, genesis.lockingScript, inputSatoshis, 0)
  const [inputProofInfo, txHeader] = Common.getTxInputProof(t1, 0)
  const inputProof = new base.TxInputProof(inputProofInfo)
  const prevOutputProof = Common.getTxOutputProof(t1, 0)

  const res = genesis.unlock(
    new SigHashPreimage(toHex(preimage)),
    new PubKey(toHex(pubKey)),
    new Sig(toHex(sig)),
    new Bytes(tokenScript.toHex()),
    txHeader,
    0,
    inputProof,
    prevOutputProof.txHeader,
    prevOutputProof.hashProof,
    prevOutputProof.satoshiBytes,
    inputSatoshis,
    inputSatoshis,
    new Ripemd160(addr.hashBuffer.toString('hex')),
    t2Change,
    new Bytes('')
  )
  const v = res.verify({ tx: t2, inputIndex: 0, inputSatoshis })
  if (!v.success) throw new Error(`T2 genesis unlock verify failed: ${v.error}`)
  t2.inputs[0].setScript(res.unlockingScript)
  Common.signP2PKH(t2, privKey, 1)
  const t2Hex = t2.uncheckedSerialize()
  console.log(`[mint-musd] T2 (issue) VERIFIED! txid=${t2.id} bytes=${t2Hex.length / 2} tokenID=${tokenID.toString('hex')}`)

  if (!live) {
    console.log('[mint-musd] DRY mode: not broadcasting.')
    return
  }
  const b1 = await chain.broadcast(t1Hex)
  console.log(`[mint-musd] Broadcasted T1: ${b1}`)
  await new Promise((r) => setTimeout(r, 1500))
  const b2 = await chain.broadcast(t2Hex)
  console.log(`[mint-musd] Broadcasted T2: ${b2}`)

  saveState({
    musd: {
      symbol: 'MUSD',
      name: 'MVCSwapValUSD',
      decimals: 8,
      tokenID: tokenID.toString('hex'),
      codeHash: actualCodeHash,
      genesisTxid: b1,
      issueTxid: b2,
      supply: SUPPLY.toString(),
    },
  })
}

const PROD_CODE_HASH = 'c9cc7bbd1010b44873959a8b1a2bcedeb62302b7'

async function mintSingleProdToken(params: {
  symbol: string
  name: string
  supply: bigint
  live: boolean
}) {
  const { symbol, name, supply, live } = params
  const wif = loadKeyWif()
  const privKey = mvc.PrivateKey.fromWIF(wif)
  const pubKey = privKey.publicKey
  const addr = privKey.toAddress(mvc.Networks.livenet)
  const chain = new SwapEngine.MetaletChainClient({ net: 'livenet' })
  const base = loadSwapContractsFast()

  const Genesis = scrypt.buildContractClass(loadVerifiedContractDesc('token/tokenGenesis'))
  const Token = base.Token
  const tcArr = SwapEngine.PROD_TRANSFER_CHECK_HASHES.map((h: string) => new Bytes(h))
  const ucArr = SwapEngine.PROD_UNLOCK_CHECK_HASHES.map((h: string) => new Bytes(h))

  const TOKEN_NAME = Buffer.alloc(TokenProto.TOKEN_NAME_LEN, 0)
  TOKEN_NAME.write(name)
  const TOKEN_SYMBOL = Buffer.alloc(TokenProto.TOKEN_SYMBOL_LEN, 0)
  TOKEN_SYMBOL.write(symbol)
  const DECIMAL_NUM = Buffer.from('08', 'hex')
  const buffValue = Common.getUInt64Buf(supply)
  const tokenVersion = Common.getUInt32Buf(1)
  const tokenType = Common.getUInt32Buf(1)
  const PROTO_FLAG = ProtoHeader.PROTO_FLAG

  const inputSatoshis = 10000
  const utxos = await chain.getSpaceUtxos(addr.toString())
  const largest = [...utxos].sort((a: any, b: any) => b.satoshis - a.satoshis)[0]
  if (!largest || largest.satoshis < 100000) {
    throw new Error(`Wallet ${addr.toString()} needs >= 100,000 sats`)
  }
  console.log(`[mint-${symbol}] addr=${addr.toString()} largest=${largest.satoshis} sats live=${live}`)

  const genesis = new Genesis()
  const genData = Buffer.concat([
    TOKEN_NAME,
    TOKEN_SYMBOL,
    DECIMAL_NUM,
    addr.hashBuffer,
    Buffer.alloc(8, 0),
    Buffer.alloc(20, 0),
    Buffer.alloc(36, 0),
    tokenVersion,
    tokenType,
    PROTO_FLAG,
  ])
  genesis.setDataPart(Common.buildScriptData(genData).toString('hex'))

  const t1 = new mvc.Transaction()
  t1.version = Common.TX_VERSION
  const p2pkh = mvc.Script.buildPublicKeyHashOut(addr)
  t1.from({ txId: largest.txid, outputIndex: largest.vout, script: p2pkh, satoshis: largest.satoshis })
  t1.addOutput(new mvc.Transaction.Output({ script: genesis.lockingScript, satoshis: inputSatoshis }))
  const t1Fee = 3500
  const t1Change = largest.satoshis - inputSatoshis - t1Fee
  t1.addOutput(new mvc.Transaction.Output({ script: p2pkh, satoshis: t1Change }))
  Common.signP2PKH(t1, privKey, 0)
  const t1Hex = t1.uncheckedSerialize()

  const genesisScriptBuf: Buffer = genesis.lockingScript.toBuffer()
  const genesisTxidBuf = Buffer.from(Common.genGenesisTxid(t1.id, 0), 'hex')
  const newGenBuf = TokenProto.getNewGenesisScript(genesisScriptBuf, genesisTxidBuf)
  const genesisHash = Common.getScriptHashBuf(newGenBuf)
  const tokenID = Common.hash160(Buffer.concat([genesisHash, genesisTxidBuf]))

  const tokenInst = new Token(tcArr, ucArr)
  const tokData = Buffer.concat([
    TOKEN_NAME,
    TOKEN_SYMBOL,
    DECIMAL_NUM,
    addr.hashBuffer,
    buffValue,
    genesisHash,
    genesisTxidBuf,
    tokenVersion,
    tokenType,
    PROTO_FLAG,
  ])
  tokenInst.setDataPart(Common.buildScriptData(tokData).toString('hex'))
  const tokenScript = tokenInst.lockingScript
  const actualCodeHash = TokenProto.getScriptCodeHash(tokenScript.toBuffer()).toString('hex')
  if (actualCodeHash !== PROD_CODE_HASH) {
    throw new Error(`CodeHash mismatch: got ${actualCodeHash}, expected prod ${PROD_CODE_HASH}`)
  }

  const t2 = new mvc.Transaction()
  t2.version = Common.TX_VERSION
  const prevouts: Buffer[] = []
  Common.addInput(t2, t1.id, 0, genesis.lockingScript, inputSatoshis, prevouts)
  Common.addInput(t2, t1.id, 1, p2pkh, t1Change, prevouts, true)
  Common.addOutput(t2, mvc.Script.fromBuffer(newGenBuf), inputSatoshis)
  Common.addOutput(t2, tokenScript, inputSatoshis)
  const t2Fee = 14000
  const t2Change = t1Change - inputSatoshis - t2Fee
  Common.addOutput(t2, p2pkh, t2Change)

  const preimage = getPreimage(t2, genesis.lockingScript, inputSatoshis, 0, Common.SIG_HASH_ALL)
  const sig = signTx(t2, privKey, genesis.lockingScript, inputSatoshis, 0)
  const [inputProofInfo, txHeader] = Common.getTxInputProof(t1, 0)
  const inputProof = new base.TxInputProof(inputProofInfo)
  const prevOutputProof = Common.getTxOutputProof(t1, 0)

  const res = genesis.unlock(
    new SigHashPreimage(toHex(preimage)),
    new PubKey(toHex(pubKey)),
    new Sig(toHex(sig)),
    new Bytes(tokenScript.toHex()),
    txHeader,
    0,
    inputProof,
    prevOutputProof.txHeader,
    prevOutputProof.hashProof,
    prevOutputProof.satoshiBytes,
    inputSatoshis,
    inputSatoshis,
    new Ripemd160(addr.hashBuffer.toString('hex')),
    t2Change,
    new Bytes('')
  )
  const v = res.verify({ tx: t2, inputIndex: 0, inputSatoshis })
  if (!v.success) throw new Error(`[mint-${symbol}] T2 genesis unlock verify failed: ${v.error}`)
  t2.inputs[0].setScript(res.unlockingScript)
  Common.signP2PKH(t2, privKey, 1)
  const t2Hex = t2.uncheckedSerialize()
  console.log(`[mint-${symbol}] VERIFIED! T1=${t1.id} T2=${t2.id} tokenID=${tokenID.toString('hex')} codeHash=${actualCodeHash}`)

  if (!live) return null
  const b1 = await chain.broadcast(t1Hex)
  await new Promise((r) => setTimeout(r, 1500))
  const b2 = await chain.broadcast(t2Hex)
  console.log(`[mint-${symbol}] Broadcasted T1=${b1} T2=${b2}`)
  return {
    symbol,
    name,
    decimals: 8,
    tokenID: tokenID.toString('hex'),
    codeHash: actualCodeHash,
    genesisTxid: b1,
    issueTxid: b2,
    supply: supply.toString(),
  }
}

async function stepMintProdPair(live: boolean) {
  const tmva = await mintSingleProdToken({
    symbol: 'TMVA',
    name: 'TestMVCTokenA',
    supply: BigInt(200_000_000_000), // 2000 TMVA
    live,
  })
  if (live) {
    saveState({ tmva })
    await new Promise((r) => setTimeout(r, 2500))
  }
  const tmvb = await mintSingleProdToken({
    symbol: 'TMVB',
    name: 'TestMVCTokenB',
    supply: BigInt(1_000_000_000_000), // 10000 TMVB
    live,
  })
  if (live) {
    saveState({ tmvb })
  }
}

async function stepDeployPool(live: boolean) {
  const state = loadState()
  if (!state.tmva?.tokenID || !state.tmvb?.tokenID) {
    throw new Error('TMVA/TMVB not minted yet; run --step mint-prod-pair --live first')
  }
  const wif = loadKeyWif()
  const chain = new SwapEngine.MetaletChainClient({ net: 'livenet' })
  const tsContracts = loadTokenSwapContractsFast()

  const result = await deployTokenSwapPoolOnChain({
    chain,
    tokenSwapContracts: tsContracts,
    adminWif: wif,
    input: {
      poolId: 'tmva-tmvb',
      pairName: 'TMVA / TMVB',
      token1ID: state.tmva.tokenID,
      token1CodeHash: state.tmva.codeHash,
      token2ID: state.tmvb.tokenID,
      token2CodeHash: state.tmvb.codeHash,
      initToken1Amount: BigInt(100_000_000), // 1.0 TMVA
      initToken2Amount: BigInt(500_000_000), // 5.0 TMVB
      feeRate: 25,
      projFeeRate: 5,
      minProjFee: BigInt(0),
      dryRun: !live,
    },
    onProgress: (m) => console.log(`[deploy-pool] ${m}`),
  })

  console.log(`[deploy-pool] SUCCESS (dryRun=${result.dryRun}):`, JSON.stringify({ txids: result.txids, fees: result.fees, entry: result.entry }, null, 2))

  if (live) {
    // Also verify inspectOnChainPoolAndBuildEntry auto-detection immediately
    await new Promise((r) => setTimeout(r, 2000))
    const inspected = await inspectOnChainPoolAndBuildEntry({
      chain,
      contracts: loadSwapContractsFast(),
      whitelistSwapCodeHashes: [result.entry.contract.swapCodeHash],
      input: {
        poolId: 'tmva-tmvb',
        poolTxid: result.entry.bootstrapHead.poolTxid,
        lpGenesisTxid: result.entry.bootstrapHead.lpGenesisTxid,
        lpGenesisVout: result.entry.bootstrapHead.lpGenesisVout,
      },
    })
    console.log('[deploy-pool] inspectOnChainPoolAndBuildEntry VERIFIED:', JSON.stringify(inspected.onChain))
    saveState({
      poolEntry: result.entry,
      deployTxids: result.txids,
      currentHead: {
        poolTxid: result.entry.bootstrapHead.poolTxid,
        lpGenesisTxid: result.entry.bootstrapHead.lpGenesisTxid,
        lpGenesisVout: result.entry.bootstrapHead.lpGenesisVout,
      },
    })
  }
}

async function stepE2E(live: boolean) {
  const state = loadState()
  const entry = state.poolEntry
  if (!entry || !state.currentHead) {
    throw new Error('Pool not deployed yet; run --step deploy-pool --live first')
  }
  const wif = loadKeyWif()
  const chain = new SwapEngine.MetaletChainClient({ net: 'livenet' })
  const tsContracts = loadTokenSwapContractsFast()
  const signer = new SwapEngine.LocalMetaletSigner(wif, 'livenet')
  const userAddress = await signer.getAddress()

  const poolConfig = {
    swapContractID: entry.contract.swapContractID,
    swapSensibleID: entry.contract.swapSensibleID,
    token1ID: entry.contract.token1ID || entry.token1.tokenID,
    token1ScriptCodeHash: entry.contract.token1ScriptCodeHash || entry.token1.codeHash,
    token2ID: entry.contract.token2ID || entry.token2.tokenID,
    token2ScriptCodeHash: entry.contract.token2ScriptCodeHash || entry.token2.codeHash,
    lpTokenID: entry.contract.lpTokenID,
    fetchTokenHash: entry.contract.fetchTokenHash,
    swapCodeHash: entry.contract.swapCodeHash,
    contractHashRoot: entry.contract.contractHashRoot,
    contractHashArrayHex: entry.contract.contractHashArrayHex,
  }

  let head = { ...state.currentHead }
  const history: any[] = state.e2eHistory || []

  const waitChain = async (ms = 3000) => {
    if (live) await new Promise((r) => setTimeout(r, ms))
  }

  const pickTokenUtxo = async (codeHash: string, tokenID: string, minAmt: bigint, label: string) => {
    const list = await chain.resolveUserTokenUtxos(userAddress, codeHash, tokenID)
    const hit = [...list]
      .sort((a: any, b: any) => (b.amount > a.amount ? 1 : b.amount < a.amount ? -1 : 0))
      .find((u: any) => u.amount >= minAmt)
    if (!hit) throw new Error(`No ${label} UTXO >= ${minAmt} for ${userAddress}`)
    return hit
  }

  // 1. Swap TMVA -> TMVB (0.1 TMVA = 10,000,000)
  {
    const poolState = await TokenSwapEngine.loadTokenSwapPoolState(chain, head.poolTxid, head.lpGenesisTxid, head.lpGenesisVout)
    const userTok1 = await pickTokenUtxo(poolConfig.token1ScriptCodeHash, poolConfig.token1ID, BigInt(10_000_000), 'TMVA')
    console.log(`[e2e 1/4] Pool before swap T1->T2: r1=${poolState.r1} r2=${poolState.r2} lp=${poolState.lp}`)
    const res = await TokenSwapEngine.executeTokenToTokenSwap({
      contracts: tsContracts,
      chain,
      signer,
      poolConfig,
      poolState,
      direction: 'token1ToToken2',
      userTokenUtxo: userTok1,
      tokenAmountIn: BigInt(10_000_000),
      minAmountOut: BigInt(1),
      broadcast: live,
    })
    console.log(`[e2e 1/4] Swap TMVA->TMVB PASS! execTxid=${res.execTxid} details=${JSON.stringify(res.details)} reservesAfter=${res.reservesAfter.r1}/${res.reservesAfter.r2}`)
    if (live) {
      head = { ...res.newPoolHead }
      history.push({ op: 'swap-t1-t2', execTxid: res.execTxid, prepTxid: res.prepTxid, depositTxid: res.tokenDepositTxid, details: res.details })
      saveState({ currentHead: head, e2eHistory: history })
      await waitChain()
    }
  }

  // 2. Swap TMVB -> TMVA (0.5 TMVB = 50,000,000)
  {
    const poolState = await TokenSwapEngine.loadTokenSwapPoolState(chain, head.poolTxid, head.lpGenesisTxid, head.lpGenesisVout)
    const userTok2 = await pickTokenUtxo(poolConfig.token2ScriptCodeHash, poolConfig.token2ID, BigInt(50_000_000), 'TMVB')
    console.log(`[e2e 2/4] Pool before swap T2->T1: r1=${poolState.r1} r2=${poolState.r2} lp=${poolState.lp}`)
    const res = await TokenSwapEngine.executeTokenToTokenSwap({
      contracts: tsContracts,
      chain,
      signer,
      poolConfig,
      poolState,
      direction: 'token2ToToken1',
      userTokenUtxo: userTok2,
      tokenAmountIn: BigInt(50_000_000),
      minAmountOut: BigInt(1),
      broadcast: live,
    })
    console.log(`[e2e 2/4] Swap TMVB->TMVA PASS! execTxid=${res.execTxid} details=${JSON.stringify(res.details)} reservesAfter=${res.reservesAfter.r1}/${res.reservesAfter.r2}`)
    if (live) {
      head = { ...res.newPoolHead }
      history.push({ op: 'swap-t2-t1', execTxid: res.execTxid, prepTxid: res.prepTxid, depositTxid: res.tokenDepositTxid, details: res.details })
      saveState({ currentHead: head, e2eHistory: history })
      await waitChain()
    }
  }

  // 3. Subsequent Dual-FT AddLiquidity (0.2 TMVA = 20,000,000, matched TMVB computed dynamically)
  {
    const poolState = await TokenSwapEngine.loadTokenSwapPoolState(chain, head.poolTxid, head.lpGenesisTxid, head.lpGenesisVout)
    const addT1 = BigInt(20_000_000)
    const addT2 = (addT1 * poolState.r2 + poolState.r1 - BigInt(1)) / poolState.r1
    const userTok1 = await pickTokenUtxo(poolConfig.token1ScriptCodeHash, poolConfig.token1ID, addT1, 'TMVA')
    const userTok2 = await pickTokenUtxo(poolConfig.token2ScriptCodeHash, poolConfig.token2ID, addT2, 'TMVB')
    console.log(`[e2e 3/4] Pool before AddLiq: r1=${poolState.r1} r2=${poolState.r2} lp=${poolState.lp}, adding t1=${addT1} t2=${addT2}`)
    const res = await TokenSwapEngine.executeTokenSwapAddLiquidity({
      contracts: tsContracts,
      chain,
      signer,
      poolConfig,
      poolState,
      userToken1Utxo: userTok1,
      userToken2Utxo: userTok2,
      token1AddAmount: addT1,
      token2AddAmount: addT2,
      broadcast: live,
    })
    console.log(`[e2e 3/4] Dual-FT AddLiquidity PASS! execTxid=${res.execTxid} details=${JSON.stringify(res.details)} reservesAfter=${res.reservesAfter.r1}/${res.reservesAfter.r2}/${res.reservesAfter.lp}`)
    if (live) {
      head = { ...res.newPoolHead }
      history.push({ op: 'add-liquidity', execTxid: res.execTxid, prepTxid: res.prepTxid, dep1: res.tokenDepositTxid, dep2: res.tokenDeposit2Txid, details: res.details })
      saveState({ currentHead: head, e2eHistory: history })
      await waitChain()
    }
  }

  // 4. Partial RemoveLiquidity (remove 15,000,000 LP, keeping pool well-funded)
  {
    const poolState = await TokenSwapEngine.loadTokenSwapPoolState(chain, head.poolTxid, head.lpGenesisTxid, head.lpGenesisVout)
    const remLp = BigInt(15_000_000)
    const userLp = await pickTokenUtxo(poolConfig.token1ScriptCodeHash, poolConfig.lpTokenID, remLp, 'LP')
    console.log(`[e2e 4/4] Pool before RemoveLiq: r1=${poolState.r1} r2=${poolState.r2} lp=${poolState.lp}, removing lp=${remLp}`)
    const res = await TokenSwapEngine.executeTokenSwapRemoveLiquidity({
      contracts: tsContracts,
      chain,
      signer,
      poolConfig,
      poolState,
      userLpUtxo: userLp,
      lpBurnAmount: remLp,
      broadcast: live,
    })
    console.log(`[e2e 4/4] RemoveLiquidity PASS! execTxid=${res.execTxid} details=${JSON.stringify(res.details)} reservesAfter=${res.reservesAfter.r1}/${res.reservesAfter.r2}/${res.reservesAfter.lp}`)
    if (live) {
      head = { ...res.newPoolHead }
      history.push({ op: 'remove-liquidity', execTxid: res.execTxid, prepTxid: res.prepTxid, depositTxid: res.tokenDepositTxid, details: res.details })
      saveState({ currentHead: head, e2eHistory: history })
    }
  }

  console.log('[e2e] ALL STEPS PASSED! Final head:', JSON.stringify(head))
}

async function main() {
  const args = process.argv.slice(2)
  const live = args.includes('--live')
  const stepIdx = args.indexOf('--step')
  const step = stepIdx >= 0 ? args[stepIdx + 1] : ''

  if (step === 'mint-musd') {
    await stepMintMusd(live)
  } else if (step === 'mint-prod-pair') {
    await stepMintProdPair(live)
  } else if (step === 'deploy-pool') {
    await stepDeployPool(live)
  } else if (step === 'e2e') {
    await stepE2E(live)
  } else {
    console.error('Usage: validate-tokenswap-mainnet.ts --step <mint-musd|deploy-pool|e2e> [--live]')
    process.exit(1)
  }
}

main().catch((err) => {
  console.error('FAILED:', err?.stack || err)
  process.exit(1)
})
