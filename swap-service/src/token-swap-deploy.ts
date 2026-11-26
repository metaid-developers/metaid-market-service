import { PoolRegistryEntry } from './config'
import {
  Common,
  ProtoHeader,
  SwapEngine,
  TokenProto,
  TokenSwapProto,
  UniqueProto,
  loadTokenSwapContractsFast,
  scrypt,
} from './core-bridge'
import { deriveTokenSwapContractHashArray } from './pool-admin'

const { mvc, getPreimage, toHex, SigHashPreimage, Bytes, Ripemd160, PubKey, Sig } = scrypt

const CONTRACT_DUST = 100100
const DUST = 1000

function normalizeHex(input: string, byteLen: number, fieldName: string): string {
  const clean = (input || '').trim().toLowerCase()
  const re = new RegExp(`^[0-9a-f]{${byteLen * 2}}$`)
  if (!re.test(clean)) {
    throw new Error(`Invalid ${fieldName}: expected ${byteLen * 2} hex characters`)
  }
  return clean
}

function decodeCString(buf: Buffer): string {
  const zeroIdx = buf.indexOf(0)
  const slice = zeroIdx >= 0 ? buf.slice(0, zeroIdx) : buf
  return slice.toString('utf8').trim()
}

function encodeFixedString(str: string, len: number): Buffer {
  const out = Buffer.alloc(len, 0)
  Buffer.from(str, 'utf8').copy(out, 0, 0, len)
  return out
}

export interface DeployTokenSwapPoolInput {
  poolId?: string
  pairName?: string
  token1ID: string
  token1CodeHash: string
  token1Symbol?: string
  token1Name?: string
  token1Decimals?: number
  token2ID: string
  token2CodeHash: string
  token2Symbol?: string
  token2Name?: string
  token2Decimals?: number
  lpSymbol?: string
  initToken1Amount: bigint
  initToken2Amount: bigint
  feeRate?: number
  projFeeRate?: number
  minProjFee?: bigint
  projFeeAddress?: string
  feeRateSatPerByte?: number
  dryRun?: boolean
}

export interface DeployTokenSwapPoolResult {
  dryRun: boolean
  adminAddress: string
  entry: PoolRegistryEntry
  txids: {
    splitTxid: string
    lpGenesisTxid: string
    swapGenesisTxid: string
    prepChecksTxid: string
    depositToken1Txid: string
    depositToken2Txid: string
    prepAddLiqTxid: string
    initAddLiqTxid: string
  }
  fees: {
    totalEstimatedSats: number
    initAddLiqBytes: number
  }
  rawTxs: Array<{ label: string; txid: string; rawHex: string }>
}

/**
 * End-to-end admin deployment of a native MVCSwap Token1 / Token2 pool (`contracts/tokenSwap/*`) on MVC:
 *   1. T4a:  Split funding into F5 (SwapMain anchor) + T7 funding + operational change
 *   2. T7:   Deploy tokenSwapLpGenesis (bound to swapContractID / swapCodeHash / fetchTokenHash)
 *   3. T5:   Deploy tokenSwapMain genesis (reserves = 0, bound to F5 + lpTokenID + contractHashRoot)
 *   4. T6b:  Fund TC1 + TC2 + UC1 + UC2 check contracts
 *   5. T6c1: Transfer initial Token1 amount from admin wallet -> AddLiquidity opHash
 *   6. T6c2: Transfer initial Token2 amount from admin wallet -> AddLiquidity opHash
 *   7. T6a:  Fund tokenSwapAddLiquidity op UTXO
 *   8. T6d:  First Dual-FT AddLiquidity (8-input genesis ceremony, every contract input verified offline via `.verify()`)
 *
 * Defaults to dryRun=true (builds & verifies all 8 txs offline against live wallet UTXOs without broadcasting).
 */
export async function deployTokenSwapPoolOnChain(params: {
  chain: any
  tokenSwapContracts?: any
  whitelistSwapCodeHashes?: string[]
  adminWif: string
  input: DeployTokenSwapPoolInput
  onProgress?: (msg: string) => void
}): Promise<DeployTokenSwapPoolResult> {
  const { chain, adminWif, input } = params
  const contracts = params.tokenSwapContracts || loadTokenSwapContractsFast()
  const log = params.onProgress || (() => {})
  const dryRun = input.dryRun !== false

  const token1ID = normalizeHex(input.token1ID, 20, 'token1ID')
  const token1CodeHash = normalizeHex(input.token1CodeHash, 20, 'token1CodeHash')
  const token2ID = normalizeHex(input.token2ID, 20, 'token2ID')
  const token2CodeHash = normalizeHex(input.token2CodeHash, 20, 'token2CodeHash')
  if (token1ID === token2ID) {
    throw new Error('token1ID and token2ID must be distinct tokens')
  }

  const initToken1Amount = BigInt(input.initToken1Amount)
  const initToken2Amount = BigInt(input.initToken2Amount)
  if (initToken1Amount <= BigInt(0) || initToken2Amount <= BigInt(0)) {
    throw new Error('initToken1Amount and initToken2Amount must both be > 0')
  }

  const feeRate = input.feeRate !== undefined ? Number(input.feeRate) : 25
  const projFeeRate = input.projFeeRate !== undefined ? Number(input.projFeeRate) : 5
  const minProjFee = input.minProjFee !== undefined ? BigInt(input.minProjFee) : BigInt(0)

  const privKey = mvc.PrivateKey.fromWIF(adminWif)
  const signer = new SwapEngine.LocalMetaletSigner(adminWif)
  const adminAddrObj = privKey.toAddress(mvc.Networks.livenet)
  const adminAddress: string = adminAddrObj.toString()
  const ownerHashBuf: Buffer = adminAddrObj.hashBuffer
  const projAddrObj = input.projFeeAddress
    ? mvc.Address.fromString(input.projFeeAddress)
    : adminAddrObj
  const p2pkhScript = mvc.Script.buildPublicKeyHashOut(adminAddrObj)

  const feeRateSatPerByte = input.feeRateSatPerByte || (await chain.getFeeRate())
  log(`Admin address (token-token): ${adminAddress} (feeRate=${feeRateSatPerByte} sat/B, mode=${dryRun ? 'DRY-RUN' : 'LIVE'})`)

  // 1. Resolve admin's live Token1 & Token2 UTXOs
  const resolveOne = async (label: string, codeHash: string, tokID: string, need: bigint) => {
    const utxos: any[] = await chain.resolveUserTokenUtxos(adminAddress, codeHash, tokID)
    const picked = utxos
      .filter((u) => u.amount >= need)
      .sort((a, b) => (b.amount > a.amount ? 1 : -1))[0]
    if (!picked) {
      const maxHave = utxos.reduce((m, u) => (u.amount > m ? u.amount : m), BigInt(0))
      throw new Error(`Admin address ${adminAddress} has no single ${label} UTXO >= ${need} (largest=${maxHave})`)
    }
    const realCodeHash = TokenProto.getScriptCodeHash(picked.scriptBuf).toString('hex')
    if (realCodeHash !== codeHash) {
      throw new Error(`${label} on-chain codeHash mismatch: expected ${codeHash}, actual ${realCodeHash}`)
    }
    return {
      utxo: picked,
      realCodeHash,
      symbol: decodeCString(TokenProto.getTokenSymbol(picked.scriptBuf)) || label,
      name: decodeCString(TokenProto.getTokenName(picked.scriptBuf)) || label,
      decimals: TokenProto.getTokenDecimal(picked.scriptBuf),
    }
  }

  const tok1 = await resolveOne('Token1', token1CodeHash, token1ID, initToken1Amount)
  const tok2 = await resolveOne('Token2', token2CodeHash, token2ID, initToken2Amount)

  if (tok1.utxo.scriptBuf.length !== tok2.utxo.scriptBuf.length) {
    throw new Error(
      `Token1 script length (${tok1.utxo.scriptBuf.length}) !== Token2 script length (${tok2.utxo.scriptBuf.length})`
    )
  }

  const token1Symbol = (input.token1Symbol || tok1.symbol).trim()
  const token1Name = (input.token1Name || tok1.name).trim()
  const token1Decimals = input.token1Decimals !== undefined ? Number(input.token1Decimals) : tok1.decimals

  const token2Symbol = (input.token2Symbol || tok2.symbol).trim()
  const token2Name = (input.token2Name || tok2.name).trim()
  const token2Decimals = input.token2Decimals !== undefined ? Number(input.token2Decimals) : tok2.decimals

  const slug1 = token1Symbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || token1ID.slice(0, 6)
  const slug2 = token2Symbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || token2ID.slice(0, 6)
  const poolId = (input.poolId || `${slug1}-${slug2}`).trim()
  if (!/^[a-zA-Z0-9._-]{2,64}$/.test(poolId)) {
    throw new Error(`Invalid poolId "${poolId}"`)
  }
  const pairName = (input.pairName || `${token1Symbol} / ${token2Symbol}`).trim()
  const lpSymbol = (input.lpSymbol || `${token1Symbol}-${token2Symbol} LP`).trim()
  const lpName = `${token1Symbol}-${token2Symbol} LP`

  // 2. Fetch admin SPACE UTXOs
  const spaceUtxos: any[] = await chain.getSpaceUtxos(adminAddress)
  const totalSpace = spaceUtxos.reduce((s, u) => s + BigInt(u.satoshis), BigInt(0))
  const requiredMinSpace = BigInt(1_600_000)
  if (totalSpace < requiredMinSpace) {
    throw new Error(
      `Insufficient SPACE on admin address ${adminAddress}: have ${totalSpace} sats, need at least ${requiredMinSpace} sats`
    )
  }

  // 3. T4a: Split funding into out0=F5 (300,000), out1=T7 (300,000), out2=operational change
  const t4aPrep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    spaceUtxos,
    [
      { script: p2pkhScript, satoshis: 300_000 },
      { script: p2pkhScript, satoshis: 300_000 },
    ],
    feeRateSatPerByte
  )
  chain.seedRawTx(t4aPrep.tx.id, t4aPrep.rawHex)

  const F5 = Common.getOutpoint(t4aPrep.tx.id, 0)
  const swapSensibleID = Common.genGenesisTxid(t4aPrep.tx.id, 0)
  const swapContractID = Common.hash160(F5).toString('hex')
  const swapCodeHash = contracts.swapMainCodeHash.toString('hex')
  if (params.whitelistSwapCodeHashes && !params.whitelistSwapCodeHashes.includes(swapCodeHash)) {
    throw new Error(
      `Compiled tokenSwapMain codeHash ${swapCodeHash} is not in whitelistSwapCodeHashes`
    )
  }

  const fetchTokInst = new contracts.SwapFetchToken(new Bytes(swapContractID))
  fetchTokInst.setDataPart(Common.buildScriptData(Buffer.from(swapCodeHash, 'hex')).toString('hex'))
  const fetchTokenHash = Common.hash160(fetchTokInst.lockingScript.toBuffer()).toString('hex')

  // 4. T7: Deploy tokenSwapLpGenesis
  const lpGenesisInst = new contracts.SwapLpGenesis(
    new Bytes(swapContractID),
    new Bytes(swapCodeHash),
    new Bytes(fetchTokenHash)
  )
  const lpData = Common.buildScriptData(
    Buffer.concat([
      encodeFixedString(lpName, TokenProto.TOKEN_NAME_LEN),
      encodeFixedString(lpSymbol, TokenProto.TOKEN_SYMBOL_LEN),
      Buffer.from([token1Decimals & 0xff]),
      Buffer.alloc(20, 0),
      Buffer.alloc(8, 0),
      Buffer.alloc(20, 0),
      Buffer.alloc(36, 0),
      Common.getUInt32Buf(TokenProto.PROTO_VERSION),
      Common.getUInt32Buf(TokenProto.PROTO_TYPE),
      ProtoHeader.PROTO_FLAG,
    ])
  )
  lpGenesisInst.setDataPart(lpData.toString('hex'))

  const t7Prep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [{ txid: t4aPrep.tx.id, vout: 1, satoshis: 300_000, height: 0 }],
    [{ script: lpGenesisInst.lockingScript, satoshis: CONTRACT_DUST }],
    feeRateSatPerByte
  )
  chain.seedRawTx(t7Prep.tx.id, t7Prep.rawHex)

  const lpTokenSensibleID = Common.genGenesisTxid(t7Prep.tx.id, 0)
  const t7LpGenesisBuf: Buffer = t7Prep.tx.outputs[0].script.toBuffer()
  const newLpGenScriptBuf: Buffer = TokenProto.getNewGenesisScript(
    t7LpGenesisBuf,
    Buffer.from(lpTokenSensibleID, 'hex')
  )
  const lpGenesisHashBuf = Common.hash160(newLpGenScriptBuf)
  const lpTokenID = Common.hash160(
    Buffer.concat([lpGenesisHashBuf, Buffer.from(lpTokenSensibleID, 'hex')])
  ).toString('hex')

  // 5. Derive 6-op contractHashArray + contractHashRoot
  const { contractHashArrayHex, contractHashRootHex } = deriveTokenSwapContractHashArray({
    tokenSwapContracts: contracts,
    swapContractID,
    token1ID,
    token1ScriptCodeHash: tok1.realCodeHash,
    token2ID,
    token2ScriptCodeHash: tok2.realCodeHash,
    ownerAddressHashHex: ownerHashBuf.toString('hex'),
  })

  // 6. T5: Deploy tokenSwapMain genesis (r1=0, r2=0, lp=0)
  const swapMainInst = new contracts.SwapMain()
  const poolData = Common.buildScriptData(
    Buffer.concat([
      Common.getUInt32Buf(feeRate),
      Common.getUInt32Buf(projFeeRate),
      Common.getUInt64Buf(minProjFee),
      projAddrObj.hashBuffer,
      ownerHashBuf,
      Buffer.from(contractHashRootHex, 'hex'),
      Buffer.from(lpTokenID, 'hex'),
      Buffer.from(fetchTokenHash, 'hex'),
      Common.getUInt64Buf(0),
      Common.getUInt64Buf(0),
      Common.getUInt64Buf(0),
      Common.getUInt32Buf(TokenSwapProto.CUSTOM_DATA_LEN),
      F5,
      Common.getUInt32Buf(UniqueProto.PROTO_VERSION),
      Common.getUInt32Buf(UniqueProto.PROTO_TYPE),
      ProtoHeader.PROTO_FLAG,
    ])
  )
  swapMainInst.setDataPart(poolData.toString('hex'))

  const t5Prep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [{ txid: t4aPrep.tx.id, vout: 0, satoshis: 300_000, height: 0 }],
    [{ script: swapMainInst.lockingScript, satoshis: CONTRACT_DUST }],
    feeRateSatPerByte
  )
  chain.seedRawTx(t5Prep.tx.id, t5Prep.rawHex)
  const t5PoolScriptBuf: Buffer = t5Prep.tx.outputs[0].script.toBuffer()

  log(`Prepared genesis txs: T4a=${t4aPrep.tx.id.slice(0, 12)} T7=${t7Prep.tx.id.slice(0, 12)} T5=${t5Prep.tx.id.slice(0, 12)}`)

  // 7. Build AddLiquidity op + 4 check contracts (TC1, TC2, UC1, UC2)
  const addLiqInst = new contracts.SwapAddLiquidity(
    new Bytes(swapContractID),
    [new Bytes(token1ID), new Bytes(token2ID)],
    [new Bytes(tok1.realCodeHash), new Bytes(tok2.realCodeHash)]
  )
  addLiqInst.setDataPart(
    Common.buildScriptData(
      Buffer.concat([
        ownerHashBuf,
        Buffer.from(lpTokenID, 'hex'),
        Buffer.from(fetchTokenHash, 'hex'),
        Buffer.from(swapCodeHash, 'hex'),
      ])
    ).toString('hex')
  )
  const addLiqHash = Common.hash160(addLiqInst.lockingScript.toBuffer())

  const tok1ChgAmt = tok1.utxo.amount - initToken1Amount
  const tok2ChgAmt = tok2.utxo.amount - initToken2Amount

  const tc1 = SwapEngine.buildTokenTransferCheck(
    contracts,
    Buffer.from(token1ID, 'hex'),
    Buffer.from(tok1.realCodeHash, 'hex'),
    1,
    tok1ChgAmt > BigInt(0) ? [initToken1Amount, tok1ChgAmt] : [initToken1Amount],
    tok1ChgAmt > BigInt(0) ? [addLiqHash, ownerHashBuf] : [addLiqHash]
  )
  const tc2 = SwapEngine.buildTokenTransferCheck(
    contracts,
    Buffer.from(token2ID, 'hex'),
    Buffer.from(tok2.realCodeHash, 'hex'),
    1,
    tok2ChgAmt > BigInt(0) ? [initToken2Amount, tok2ChgAmt] : [initToken2Amount],
    tok2ChgAmt > BigInt(0) ? [addLiqHash, ownerHashBuf] : [addLiqHash]
  )
  // In T6d (first addLiq): input 3 = depositToken1 -> output 3 = poolToken1; input 4 = depositToken2 -> output 4 = poolToken2
  const t1Use4to8 = SwapEngine.tokenSupports4to8UnlockCheck(tok1.utxo.scriptBuf)
  const t2Use4to8 = SwapEngine.tokenSupports4to8UnlockCheck(tok2.utxo.scriptBuf)
  const t1HashIdx = t1Use4to8 ? 1 : 0
  const t2HashIdx = t2Use4to8 ? 1 : 0
  const uc1 = SwapEngine.buildTokenUnlockContractCheck(
    contracts,
    Buffer.from(token1ID, 'hex'),
    Buffer.from(tok1.realCodeHash, 'hex'),
    [3],
    [initToken1Amount],
    [Buffer.from(fetchTokenHash, 'hex')],
    t1Use4to8
  )
  const uc2 = SwapEngine.buildTokenUnlockContractCheck(
    contracts,
    Buffer.from(token2ID, 'hex'),
    Buffer.from(tok2.realCodeHash, 'hex'),
    [4],
    [initToken2Amount],
    [Buffer.from(fetchTokenHash, 'hex')],
    t2Use4to8
  )

  // 8. T6b: Fund TC1 (out 0), TC2 (out 1), UC1 (out 2), UC2 (out 3)
  const t6bPrep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [{ txid: t4aPrep.tx.id, vout: t4aPrep.changeVout, satoshis: t4aPrep.changeSatoshis, height: 0 }],
    [
      { script: tc1.lockingScript, satoshis: CONTRACT_DUST },
      { script: tc2.lockingScript, satoshis: CONTRACT_DUST },
      { script: uc1.lockingScript, satoshis: CONTRACT_DUST },
      { script: uc2.lockingScript, satoshis: CONTRACT_DUST },
    ],
    feeRateSatPerByte
  )
  chain.seedRawTx(t6bPrep.tx.id, t6bPrep.rawHex)

  // 9. T6c1: Transfer initial Token1 to addLiqHash
  const t6c1Dep = await SwapEngine.buildSignedTokenTransferTx(
    contracts,
    chain,
    signer,
    adminAddress,
    tok1.utxo,
    tc1,
    t6bPrep.tx,
    0,
    CONTRACT_DUST,
    t6bPrep.tx.id,
    t6bPrep.changeVout,
    t6bPrep.changeSatoshis,
    addLiqHash,
    initToken1Amount,
    feeRateSatPerByte
  )
  chain.seedRawTx(t6c1Dep.tx.id, t6c1Dep.rawHex)

  // 10. T6c2: Transfer initial Token2 to addLiqHash
  const t6c2Dep = await SwapEngine.buildSignedTokenTransferTx(
    contracts,
    chain,
    signer,
    adminAddress,
    tok2.utxo,
    tc2,
    t6bPrep.tx,
    1,
    CONTRACT_DUST,
    t6c1Dep.tx.id,
    t6c1Dep.spaceChangeVout,
    t6c1Dep.spaceChangeSatoshis,
    addLiqHash,
    initToken2Amount,
    feeRateSatPerByte
  )
  chain.seedRawTx(t6c2Dep.tx.id, t6c2Dep.rawHex)

  // 11. T6a: Fund AddLiquidity op UTXO
  const t6aPrep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [{ txid: t6c2Dep.tx.id, vout: t6c2Dep.spaceChangeVout, satoshis: t6c2Dep.spaceChangeSatoshis, height: 0 }],
    [{ script: addLiqInst.lockingScript, satoshis: CONTRACT_DUST }],
    feeRateSatPerByte
  )
  chain.seedRawTx(t6aPrep.tx.id, t6aPrep.rawHex)

  log(
    `Prepared funding txs: T6b=${t6bPrep.tx.id.slice(0, 12)} T6c1=${t6c1Dep.tx.id.slice(0, 12)} T6c2=${t6c2Dep.tx.id.slice(0, 12)} T6a=${t6aPrep.tx.id.slice(0, 12)}`
  )

  // 12. T6d: First Dual-FT AddLiquidity (8 inputs, 6 outputs)
  const depTok1Buf: Buffer = t6c1Dep.tx.outputs[0].script.toBuffer()
  const depTok2Buf: Buffer = t6c2Dep.tx.outputs[0].script.toBuffer()
  const depTok1Backtrace = await SwapEngine.resolveTokenBacktrace(chain, t6c1Dep.tx, 0)
  const depTok2Backtrace = await SwapEngine.resolveTokenBacktrace(chain, t6c2Dep.tx, 0)

  const swapVerifyInst = new contracts.SwapMain()
  swapVerifyInst.setDataPart(poolData.toString('hex'))
  if (!swapVerifyInst.lockingScript.toBuffer().equals(t5PoolScriptBuf)) {
    throw new Error('tokenSwapMain instance lockingScript mismatch')
  }
  const lpGenVerifyInst = new contracts.SwapLpGenesis(
    new Bytes(swapContractID),
    new Bytes(swapCodeHash),
    new Bytes(fetchTokenHash)
  )
  lpGenVerifyInst.setDataPart(lpData.toString('hex'))
  if (!lpGenVerifyInst.lockingScript.toBuffer().equals(t7LpGenesisBuf)) {
    throw new Error('tokenSwapLpGenesis instance lockingScript mismatch')
  }
  const depTok1Inst = SwapEngine.instantiateTokenFromChain(contracts, depTok1Buf)
  const depTok2Inst = SwapEngine.instantiateTokenFromChain(contracts, depTok2Buf)

  // Expected T6d outputs (matching tokenSwapAddLiquidity.scrypt line 172 exactly):
  // [0: swapOutput, 1: lpTokenOutput, 2: swapLpGenesisOutput, 3: poolToken1Output, 4: poolToken2Output, 5: changeOutput]
  const swapScriptV2Buf: Buffer = TokenSwapProto.getNewSwapScript(
    t5PoolScriptBuf,
    initToken1Amount,
    initToken2Amount,
    initToken1Amount
  )
  const poolToken1Buf: Buffer = TokenProto.getNewTokenScript(
    depTok1Buf,
    Buffer.from(fetchTokenHash, 'hex'),
    initToken1Amount
  )
  const poolToken2Buf: Buffer = TokenProto.getNewTokenScript(
    depTok2Buf,
    Buffer.from(fetchTokenHash, 'hex'),
    initToken2Amount
  )
  const lpTokenBaseScript = Buffer.concat([
    TokenProto.getScriptCode(depTok1Buf),
    TokenProto.getScriptData(newLpGenScriptBuf),
  ])
  const newLpTokenScriptBuf = Buffer.concat([
    lpTokenBaseScript.subarray(0, lpTokenBaseScript.length - TokenProto.TOKEN_ADDRESS_OFFSET),
    ownerHashBuf,
    Common.getUInt64Buf(initToken1Amount),
    lpGenesisHashBuf,
    lpTokenBaseScript.subarray(lpTokenBaseScript.length - TokenProto.GENESIS_HASH_OFFSET + ProtoHeader.HASH_LEN),
  ])
  if (TokenProto.getTokenID(newLpTokenScriptBuf).toString('hex') !== lpTokenID) {
    throw new Error('Computed first dual-FT LP tokenID mismatch')
  }

  let t6dFee = Math.ceil(65_000 * feeRateSatPerByte)
  let t6dTx: any = null
  let t6dRawHex = ''

  for (let pass = 0; pass < 2; pass++) {
    const p2pkhInSat = t6aPrep.changeSatoshis
    const inTotal =
      CONTRACT_DUST + // 0: addLiq
      t5Prep.tx.outputs[0].satoshis + // 1: swapMain genesis
      t7Prep.tx.outputs[0].satoshis + // 2: lpGenesis
      t6c1Dep.tx.outputs[0].satoshis + // 3: deposited Token1
      t6c2Dep.tx.outputs[0].satoshis + // 4: deposited Token2
      p2pkhInSat + // 5: admin P2PKH
      CONTRACT_DUST + // 6: uc1
      CONTRACT_DUST // 7: uc2
    const outContractTotal = CONTRACT_DUST * 5
    const dChange = inTotal - outContractTotal - t6dFee
    if (dChange < DUST) {
      throw new Error(`Insufficient SPACE for T6d first dual-FT addLiquidity: change=${dChange}, fee=${t6dFee}`)
    }

    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    const prevouts: Buffer[] = []
    Common.addInput(tx, t6aPrep.tx.id, 0, addLiqInst.lockingScript, CONTRACT_DUST, prevouts) // 0: AddLiq op
    Common.addInput(tx, t5Prep.tx.id, 0, t5Prep.tx.outputs[0].script, t5Prep.tx.outputs[0].satoshis, prevouts) // 1: SwapMain
    Common.addInput(tx, t7Prep.tx.id, 0, t7Prep.tx.outputs[0].script, t7Prep.tx.outputs[0].satoshis, prevouts) // 2: LpGenesis
    Common.addInput(tx, t6c1Dep.tx.id, 0, mvc.Script.fromBuffer(depTok1Buf), t6c1Dep.tx.outputs[0].satoshis, prevouts) // 3: depTok1
    Common.addInput(tx, t6c2Dep.tx.id, 0, mvc.Script.fromBuffer(depTok2Buf), t6c2Dep.tx.outputs[0].satoshis, prevouts) // 4: depTok2
    Common.addInput(tx, t6aPrep.tx.id, t6aPrep.changeVout, p2pkhScript, p2pkhInSat, prevouts, true) // 5: admin P2PKH
    Common.addInput(tx, t6bPrep.tx.id, 2, uc1.lockingScript, CONTRACT_DUST, prevouts) // 6: uc1
    Common.addInput(tx, t6bPrep.tx.id, 3, uc2.lockingScript, CONTRACT_DUST, prevouts) // 7: uc2

    Common.addOutput(tx, mvc.Script.fromBuffer(swapScriptV2Buf), CONTRACT_DUST) // 0: swap
    Common.addOutput(tx, mvc.Script.fromBuffer(newLpTokenScriptBuf), CONTRACT_DUST) // 1: lpToken
    Common.addOutput(tx, mvc.Script.fromBuffer(newLpGenScriptBuf), CONTRACT_DUST) // 2: lpGenesis
    Common.addOutput(tx, mvc.Script.fromBuffer(poolToken1Buf), CONTRACT_DUST) // 3: poolToken1
    Common.addOutput(tx, mvc.Script.fromBuffer(poolToken2Buf), CONTRACT_DUST) // 4: poolToken2
    Common.addOutput(tx, p2pkhScript, dChange) // 5: change

    const prevoutsBuf = Buffer.concat(prevouts)
    const { TxInputProof, TxOutputProof } = contracts

    // Input 0: tokenSwapAddLiquidity (first dual-FT addLiq)
    {
      const preimage = getPreimage(tx, addLiqInst.lockingScript.subScript(0), CONTRACT_DUST, 0, Common.SIG_HASH_ALL)
      const swapProof = new TxOutputProof(Common.getTxOutputProof(t5Prep.tx, 0))
      const lpProof = new TxOutputProof(Common.getTxOutputProof(t7Prep.tx, 0))
      const tok1Proof = new TxOutputProof(Common.getTxOutputProof(t6c1Dep.tx, 0))
      const tok2Proof = new TxOutputProof(Common.getTxOutputProof(t6c2Dep.tx, 0))
      const empty = new TxOutputProof(Common.getEmptyTxOutputProof())
      const res = addLiqInst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        new Bytes(t5PoolScriptBuf.toString('hex')),
        swapProof,
        new Bytes(t7LpGenesisBuf.toString('hex')),
        lpProof,
        [new Bytes(depTok1Buf.toString('hex')), new Bytes(depTok2Buf.toString('hex'))],
        [tok1Proof, tok2Proof],
        [empty, empty],
        CONTRACT_DUST, // swapOutputSatoshis
        CONTRACT_DUST, // tokenOutputSatoshis
        CONTRACT_DUST, // lpGenesisOutputSatoshis
        dChange,
        new Ripemd160(ownerHashBuf.toString('hex')),
        true
      )
      const v = res.verify({ tx, inputIndex: 0, inputSatoshis: CONTRACT_DUST })
      if (!v.success) throw new Error(`T6d input 0 (tokenSwapAddLiquidity) verify failed: ${v.error}`)
      tx.inputs[0].setScript(res.unlockingScript)
    }

    // Input 1: tokenSwapMain genesis unlock
    {
      const inSat = t5Prep.tx.outputs[0].satoshis
      const preimage = getPreimage(tx, t5Prep.tx.outputs[0].script.subScript(0), inSat, 1, Common.SIG_HASH_ALL)
      const contractTxProof = new TxOutputProof(Common.getTxOutputProof(t6aPrep.tx, 0))
      const contractTxScript = t6aPrep.tx.outputs[0].script.toBuffer()
      const swapTxOutputProof = new TxOutputProof(Common.getTxOutputProof(t5Prep.tx, 0))
      const swapTxInputProof = new TxInputProof(Common.getTxInputProof(t5Prep.tx, 0)[0])
      const prevSwapTxProof = new TxOutputProof(Common.getTxOutputProof(t4aPrep.tx, 0))
      const res = swapVerifyInst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        contractTxProof,
        new Bytes(contractTxScript.toString('hex')),
        new Bytes(contractHashArrayHex),
        TokenSwapProto.OP_ADD_LIQUIDITY,
        0,
        swapTxOutputProof.txHeader,
        swapTxInputProof,
        prevSwapTxProof,
        new Bytes('')
      )
      const v = res.verify({ tx, inputIndex: 1, inputSatoshis: inSat })
      if (!v.success) throw new Error(`T6d input 1 (tokenSwapMain) verify failed: ${v.error}`)
      tx.inputs[1].setScript(res.unlockingScript)
    }

    // Input 2: tokenSwapLpGenesis first mint unlock (SIG_HASH_SINGLE)
    {
      const inSat = t7Prep.tx.outputs[0].satoshis
      const preimage = getPreimage(tx, t7Prep.tx.outputs[0].script, inSat, 2, Common.SIG_HASH_SINGLE)
      const [gInProof, gHeader] = Common.getTxInputProof(t7Prep.tx, 0)
      const prevGenesisProof = Common.getTxOutputProof(t4aPrep.tx, 1)
      const res = lpGenVerifyInst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        gHeader,
        0,
        new TxInputProof(gInProof),
        prevGenesisProof.txHeader,
        prevGenesisProof.hashProof,
        prevGenesisProof.satoshiBytes,
        new TxOutputProof(Common.getTxOutputProof(t6aPrep.tx, 0)),
        new Bytes(t6aPrep.tx.outputs[0].script.toBuffer().toString('hex')),
        new TxOutputProof(Common.getTxOutputProof(t5Prep.tx, 0)),
        new Bytes(t5PoolScriptBuf.toString('hex')),
        new Bytes(contractHashArrayHex),
        CONTRACT_DUST
      )
      const v = res.verify({ tx, inputIndex: 2, inputSatoshis: inSat })
      if (!v.success) throw new Error(`T6d input 2 (tokenSwapLpGenesis) verify failed: ${v.error}`)
      tx.inputs[2].setScript(res.unlockingScript)
    }

    // Input 3: Token1 op=2 unlock-from-contract (checked by uc1 at input 6)
    {
      const inSat = t6c1Dep.tx.outputs[0].satoshis
      const preimage = getPreimage(tx, mvc.Script.fromBuffer(depTok1Buf), inSat, 3, Common.SIG_HASH_ALL)
      const [depInProof, depTxHeader] = Common.getTxInputProof(t6c1Dep.tx, depTok1Backtrace.prevTokenInputIndex)
      const prevTokOutProof = Common.getTxOutputProof(depTok1Backtrace.prevTokTx, depTok1Backtrace.prevTokVout)
      const res = depTok1Inst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        0,
        t1HashIdx,
        6,
        new TxOutputProof(Common.getTxOutputProof(t6bPrep.tx, 2)),
        new Bytes(uc1.lockingScript.toBuffer().toString('hex')),
        depTok1Backtrace.prevTokenInputIndex,
        new Bytes(depTok1Backtrace.prevTokenAddress.toString('hex')),
        depTok1Backtrace.prevTokenAmount,
        depTxHeader,
        new TxInputProof(depInProof),
        new TxOutputProof(prevTokOutProof),
        new PubKey(Buffer.alloc(33, 0).toString('hex')),
        new Sig(Buffer.alloc(72, 0).toString('hex')),
        0,
        new TxOutputProof(Common.getTxOutputProof(t6aPrep.tx, 0)),
        TokenProto.OP_UNLOCK_FROM_CONTRACT
      )
      const v = res.verify({ tx, inputIndex: 3, inputSatoshis: inSat })
      if (!v.success) throw new Error(`T6d input 3 (Token1 op=2) verify failed: ${v.error}`)
      tx.inputs[3].setScript(res.unlockingScript)
    }

    // Input 4: Token2 op=2 unlock-from-contract (checked by uc2 at input 7)
    {
      const inSat = t6c2Dep.tx.outputs[0].satoshis
      const preimage = getPreimage(tx, mvc.Script.fromBuffer(depTok2Buf), inSat, 4, Common.SIG_HASH_ALL)
      const [depInProof, depTxHeader] = Common.getTxInputProof(t6c2Dep.tx, depTok2Backtrace.prevTokenInputIndex)
      const prevTokOutProof = Common.getTxOutputProof(depTok2Backtrace.prevTokTx, depTok2Backtrace.prevTokVout)
      const res = depTok2Inst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        0,
        t2HashIdx,
        7,
        new TxOutputProof(Common.getTxOutputProof(t6bPrep.tx, 3)),
        new Bytes(uc2.lockingScript.toBuffer().toString('hex')),
        depTok2Backtrace.prevTokenInputIndex,
        new Bytes(depTok2Backtrace.prevTokenAddress.toString('hex')),
        depTok2Backtrace.prevTokenAmount,
        depTxHeader,
        new TxInputProof(depInProof),
        new TxOutputProof(prevTokOutProof),
        new PubKey(Buffer.alloc(33, 0).toString('hex')),
        new Sig(Buffer.alloc(72, 0).toString('hex')),
        0,
        new TxOutputProof(Common.getTxOutputProof(t6aPrep.tx, 0)),
        TokenProto.OP_UNLOCK_FROM_CONTRACT
      )
      const v = res.verify({ tx, inputIndex: 4, inputSatoshis: inSat })
      if (!v.success) throw new Error(`T6d input 4 (Token2 op=2) verify failed: ${v.error}`)
      tx.inputs[4].setScript(res.unlockingScript)
    }

    // Input 6: uc1 (Token1 input 3 -> poolToken1 output 3)
    {
      const preimage = getPreimage(tx, uc1.lockingScript.subScript(0), CONTRACT_DUST, 6, Common.SIG_HASH_ALL)
      const proof = Common.getTxOutputProof(t6c1Dep.tx, 0)
      const headerBuf = Buffer.from(proof.txHeader.serialize(), 'hex')
      const hashProofBuf = Buffer.from(proof.hashProof.serialize(), 'hex')
      const satBuf = Buffer.from(proof.satoshiBytes.serialize(), 'hex')
      let otherOutputArray = Buffer.alloc(0)
      for (const idx of [0, 1, 2, 4, 5]) {
        const o = tx.outputs[idx]
        const ob = Common.buildOutput(o.script.toBuffer(), o.satoshis)
        otherOutputArray = Buffer.concat([otherOutputArray, Common.getUInt32Buf(ob.length), ob])
      }
      const res = uc1.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        new Bytes(depTok1Buf.toString('hex')),
        new Bytes(headerBuf.toString('hex')),
        new Bytes(Common.getUInt32Buf(hashProofBuf.length).toString('hex') + hashProofBuf.toString('hex')),
        new Bytes(satBuf.toString('hex')),
        new Bytes(addLiqHash.toString('hex')),
        new Bytes(Common.getUInt64Buf(initToken1Amount).toString('hex')),
        tx.outputs.length,
        new Bytes(Common.getUInt32Buf(3).toString('hex')),
        CONTRACT_DUST,
        new Bytes(otherOutputArray.toString('hex'))
      )
      const v = res.verify({ tx, inputIndex: 6, inputSatoshis: CONTRACT_DUST })
      if (!v.success) throw new Error(`T6d input 6 (uc1 UnlockContractCheck) verify failed: ${v.error}`)
      tx.inputs[6].setScript(res.unlockingScript)
    }

    // Input 7: uc2 (Token2 input 4 -> poolToken2 output 4)
    {
      const preimage = getPreimage(tx, uc2.lockingScript.subScript(0), CONTRACT_DUST, 7, Common.SIG_HASH_ALL)
      const proof = Common.getTxOutputProof(t6c2Dep.tx, 0)
      const headerBuf = Buffer.from(proof.txHeader.serialize(), 'hex')
      const hashProofBuf = Buffer.from(proof.hashProof.serialize(), 'hex')
      const satBuf = Buffer.from(proof.satoshiBytes.serialize(), 'hex')
      let otherOutputArray = Buffer.alloc(0)
      for (const idx of [0, 1, 2, 3, 5]) {
        const o = tx.outputs[idx]
        const ob = Common.buildOutput(o.script.toBuffer(), o.satoshis)
        otherOutputArray = Buffer.concat([otherOutputArray, Common.getUInt32Buf(ob.length), ob])
      }
      const res = uc2.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        new Bytes(depTok2Buf.toString('hex')),
        new Bytes(headerBuf.toString('hex')),
        new Bytes(Common.getUInt32Buf(hashProofBuf.length).toString('hex') + hashProofBuf.toString('hex')),
        new Bytes(satBuf.toString('hex')),
        new Bytes(addLiqHash.toString('hex')),
        new Bytes(Common.getUInt64Buf(initToken2Amount).toString('hex')),
        tx.outputs.length,
        new Bytes(Common.getUInt32Buf(4).toString('hex')),
        CONTRACT_DUST,
        new Bytes(otherOutputArray.toString('hex'))
      )
      const v = res.verify({ tx, inputIndex: 7, inputSatoshis: CONTRACT_DUST })
      if (!v.success) throw new Error(`T6d input 7 (uc2 UnlockContractCheck) verify failed: ${v.error}`)
      tx.inputs[7].setScript(res.unlockingScript)
    }

    // Input 5: Admin P2PKH signature
    Common.signP2PKH(tx, privKey, 5)

    const rawHex = tx.uncheckedSerialize()
    const actualBytes = rawHex.length / 2
    const minNeededFee = Math.ceil(actualBytes * feeRateSatPerByte) + 300
    if (t6dFee < minNeededFee && pass === 0) {
      t6dFee = minNeededFee + 1500
      continue
    }
    if (t6dFee < actualBytes) {
      throw new Error(`T6d fee ${t6dFee} < tx bytes ${actualBytes}`)
    }
    t6dTx = tx
    t6dRawHex = rawHex
    break
  }

  log(`T6d first dual-FT AddLiquidity verified offline! txid=${t6dTx.id} bytes=${t6dRawHex.length / 2} fee=${t6dFee}`)

  const rawTxs: Array<{ label: string; txid: string; rawHex: string }> = [
    { label: 'T4a_split', txid: t4aPrep.tx.id, rawHex: t4aPrep.rawHex },
    { label: 'T7_lpGenesis', txid: t7Prep.tx.id, rawHex: t7Prep.rawHex },
    { label: 'T5_swapGenesis', txid: t5Prep.tx.id, rawHex: t5Prep.rawHex },
    { label: 'T6b_prepChecks', txid: t6bPrep.tx.id, rawHex: t6bPrep.rawHex },
    { label: 'T6c1_depositToken1', txid: t6c1Dep.tx.id, rawHex: t6c1Dep.rawHex },
    { label: 'T6c2_depositToken2', txid: t6c2Dep.tx.id, rawHex: t6c2Dep.rawHex },
    { label: 'T6a_prepAddLiq', txid: t6aPrep.tx.id, rawHex: t6aPrep.rawHex },
    { label: 'T6d_initAddLiq', txid: t6dTx.id, rawHex: t6dRawHex },
  ]

  if (!dryRun) {
    for (const step of rawTxs) {
      log(`Broadcasting ${step.label} (${step.txid})...`)
      const bTxid = await chain.broadcast(step.rawHex)
      if (bTxid !== step.txid) {
        log(`Broadcast returned ${bTxid} (expected ${step.txid})`)
      }
      await new Promise((r) => setTimeout(r, 1200))
    }
  }

  const entry: PoolRegistryEntry = {
    poolId,
    pairName,
    poolType: 'token-token',
    verified: true,
    token1: {
      symbol: token1Symbol,
      name: token1Name,
      decimals: token1Decimals,
      tokenID: token1ID,
      codeHash: tok1.realCodeHash,
    },
    token2: {
      symbol: token2Symbol,
      name: token2Name,
      decimals: token2Decimals,
      tokenID: token2ID,
      codeHash: tok2.realCodeHash,
    },
    lpToken: {
      symbol: lpSymbol,
      decimals: token1Decimals,
      tokenID: lpTokenID,
      codeHash: tok1.realCodeHash,
    },
    contract: {
      swapContractID,
      swapSensibleID,
      tokenID: token2ID,
      tokenScriptCodeHash: tok2.realCodeHash,
      token1ID,
      token1ScriptCodeHash: tok1.realCodeHash,
      token2ID,
      token2ScriptCodeHash: tok2.realCodeHash,
      lpTokenID,
      fetchTokenHash,
      swapCodeHash,
      contractHashRoot: contractHashRootHex,
      contractHashArrayHex,
    },
    bootstrapHead: {
      poolTxid: t6dTx.id,
      lpGenesisTxid: t6dTx.id,
      lpGenesisVout: 2,
    },
  }

  const totalEstimatedSats =
    t4aPrep.fee +
    t7Prep.fee +
    t5Prep.fee +
    t6bPrep.fee +
    t6c1Dep.fee +
    t6c2Dep.fee +
    t6aPrep.fee +
    t6dFee

  return {
    dryRun,
    adminAddress,
    entry,
    txids: {
      splitTxid: t4aPrep.tx.id,
      lpGenesisTxid: t7Prep.tx.id,
      swapGenesisTxid: t5Prep.tx.id,
      prepChecksTxid: t6bPrep.tx.id,
      depositToken1Txid: t6c1Dep.tx.id,
      depositToken2Txid: t6c2Dep.tx.id,
      prepAddLiqTxid: t6aPrep.tx.id,
      initAddLiqTxid: t6dTx.id,
    },
    fees: {
      totalEstimatedSats,
      initAddLiqBytes: t6dRawHex.length / 2,
    },
    rawTxs,
  }
}
