import { PoolRegistryEntry } from './config'
import {
  Common,
  ProtoHeader,
  SwapEngine,
  TokenProto,
  TokenSwapEngine,
  TokenSwapProto,
  UniqueProto,
  loadTokenSwapContractsFast,
  loadVerifiedContractDesc,
  scrypt,
} from './core-bridge'

const { mvc, Bytes, Ripemd160, PubKey, Sig, SigHashPreimage, getPreimage, toHex, buildContractClass } = scrypt
const OP_RETURN = Buffer.from('6a', 'hex')
const CONTRACT_DUST = 100100
const DUST = 1000

let cachedAuxContracts: { SwapUpdateContract: any; SwapAdmin: any } | null = null

export function loadAdminAuxContracts() {
  if (cachedAuxContracts) return cachedAuxContracts
  cachedAuxContracts = {
    SwapUpdateContract: buildContractClass(loadVerifiedContractDesc('swap/swapUpdateContract')),
    SwapAdmin: buildContractClass(loadVerifiedContractDesc('swap/swapAdmin')),
  }
  return cachedAuxContracts
}

function getCodeHash(scriptBuf: Buffer): Buffer {
  return mvc.crypto.Hash.sha256ripemd160(Buffer.concat([scriptBuf, OP_RETURN]))
}

function decodeCString(buf: Buffer): string {
  const zeroIdx = buf.indexOf(0)
  const slice = zeroIdx >= 0 ? buf.subarray(0, zeroIdx) : buf
  return slice.toString('utf8').trim()
}

function encodeFixedString(str: string, len: number): Buffer {
  const buf = Buffer.alloc(len, 0)
  buf.write(str, 0, len, 'utf8')
  return buf
}

export function normalizeHex(val: string, expectedBytes: number, fieldName: string): string {
  const cleaned = String(val || '').trim().toLowerCase()
  const re = new RegExp(`^[0-9a-f]{${expectedBytes * 2}}$`)
  if (!re.test(cleaned)) {
    throw new Error(`${fieldName} must be a ${expectedBytes * 2}-char lowercase hex string`)
  }
  return cleaned
}

export function deriveContractHashArray(params: {
  contracts: any
  swapContractID: string
  tokenID: string
  tokenScriptCodeHash: string
  ownerAddressHashHex: string
}): {
  contractHashArrayHex: string
  contractHashRootHex: string
  codeHashes: Record<string, string>
} {
  const { contracts, swapContractID, tokenID, tokenScriptCodeHash, ownerAddressHashHex } = params
  const { SwapUpdateContract, SwapAdmin } = loadAdminAuxContracts()

  const updateContract = new SwapUpdateContract(new Bytes(swapContractID))
  const updateContractCodeHash = getCodeHash(updateContract.lockingScript.toBuffer()).toString('hex')

  const addLiquidity = new contracts.SwapAddLiquidity(
    new Bytes(swapContractID),
    new Bytes(tokenID),
    new Bytes(tokenScriptCodeHash)
  )
  const addLiquidityCodeHash = getCodeHash(addLiquidity.lockingScript.toBuffer()).toString('hex')

  const removeLiquidity = new contracts.SwapRemoveLiquidity(
    new Bytes(swapContractID),
    new Bytes(tokenID),
    new Bytes(tokenScriptCodeHash)
  )
  const removeLiquidityCodeHash = getCodeHash(removeLiquidity.lockingScript.toBuffer()).toString('hex')

  const mvcToToken = new contracts.SwapMvcToToken(
    new Bytes(swapContractID),
    new Bytes(tokenID),
    new Bytes(tokenScriptCodeHash)
  )
  const mvcToTokenCodeHash = getCodeHash(mvcToToken.lockingScript.toBuffer()).toString('hex')

  const tokenToMvc = new contracts.SwapTokenToMvc(
    new Bytes(swapContractID),
    new Bytes(tokenID),
    new Bytes(tokenScriptCodeHash)
  )
  const tokenToMvcCodeHash = getCodeHash(tokenToMvc.lockingScript.toBuffer()).toString('hex')

  const swapAdmin = new SwapAdmin(new Bytes(swapContractID), new Ripemd160(ownerAddressHashHex))
  const adminCodeHash = getCodeHash(swapAdmin.lockingScript.toBuffer()).toString('hex')

  const contractHashArray = Buffer.concat([
    Buffer.from(updateContractCodeHash, 'hex'),
    Buffer.from(addLiquidityCodeHash, 'hex'),
    Buffer.from(removeLiquidityCodeHash, 'hex'),
    Buffer.from(mvcToTokenCodeHash, 'hex'),
    Buffer.from(tokenToMvcCodeHash, 'hex'),
    Buffer.from(adminCodeHash, 'hex'),
  ])
  const contractHashRoot = Common.hash160(contractHashArray)

  return {
    contractHashArrayHex: contractHashArray.toString('hex'),
    contractHashRootHex: contractHashRoot.toString('hex'),
    codeHashes: {
      updateContractCodeHash,
      addLiquidityCodeHash,
      removeLiquidityCodeHash,
      mvcToTokenCodeHash,
      tokenToMvcCodeHash,
      adminCodeHash,
    },
  }
}

export function deriveTokenSwapContractHashArray(params: {
  tokenSwapContracts?: any
  swapContractID: string
  token1ID: string
  token1ScriptCodeHash: string
  token2ID: string
  token2ScriptCodeHash: string
  ownerAddressHashHex: string
}): {
  contractHashArrayHex: string
  contractHashRootHex: string
  codeHashes: Record<string, string>
} {
  const c = params.tokenSwapContracts || loadTokenSwapContractsFast()
  const { swapContractID, token1ID, token1ScriptCodeHash, token2ID, token2ScriptCodeHash, ownerAddressHashHex } = params
  const tokenIDs = [new Bytes(token1ID), new Bytes(token2ID)]
  const tokenCodeHashes = [new Bytes(token1ScriptCodeHash), new Bytes(token2ScriptCodeHash)]

  const updateContract = new c.SwapUpdateContract(new Bytes(swapContractID))
  const updateContractCodeHash = getCodeHash(updateContract.lockingScript.toBuffer()).toString('hex')

  const addLiquidity = new c.SwapAddLiquidity(new Bytes(swapContractID), tokenIDs, tokenCodeHashes)
  const addLiquidityCodeHash = getCodeHash(addLiquidity.lockingScript.toBuffer()).toString('hex')

  const removeLiquidity = new c.SwapRemoveLiquidity(new Bytes(swapContractID), tokenIDs, tokenCodeHashes)
  const removeLiquidityCodeHash = getCodeHash(removeLiquidity.lockingScript.toBuffer()).toString('hex')

  const token1ToToken2 = new c.SwapTokenToToken(new Bytes(swapContractID), tokenIDs, tokenCodeHashes, 0, 1)
  const token1ToToken2CodeHash = getCodeHash(token1ToToken2.lockingScript.toBuffer()).toString('hex')

  const token2ToToken1 = new c.SwapTokenToToken(new Bytes(swapContractID), tokenIDs, tokenCodeHashes, 1, 0)
  const token2ToToken1CodeHash = getCodeHash(token2ToToken1.lockingScript.toBuffer()).toString('hex')

  const swapAdmin = new c.SwapAdmin(new Bytes(swapContractID), new Ripemd160(ownerAddressHashHex))
  const adminCodeHash = getCodeHash(swapAdmin.lockingScript.toBuffer()).toString('hex')

  const contractHashArray = Buffer.concat([
    Buffer.from(updateContractCodeHash, 'hex'),
    Buffer.from(addLiquidityCodeHash, 'hex'),
    Buffer.from(removeLiquidityCodeHash, 'hex'),
    Buffer.from(token1ToToken2CodeHash, 'hex'),
    Buffer.from(token2ToToken1CodeHash, 'hex'),
    Buffer.from(adminCodeHash, 'hex'),
  ])
  const contractHashRoot = Common.hash160(contractHashArray)

  return {
    contractHashArrayHex: contractHashArray.toString('hex'),
    contractHashRootHex: contractHashRoot.toString('hex'),
    codeHashes: {
      updateContractCodeHash,
      addLiquidityCodeHash,
      removeLiquidityCodeHash,
      token1ToToken2CodeHash,
      token2ToToken1CodeHash,
      adminCodeHash,
    },
  }
}

export interface ImportPoolInput {
  poolId?: string
  pairName?: string
  poolTxid: string
  lpGenesisTxid: string
  lpGenesisVout?: number
  token1Symbol?: string
  token1Name?: string
  token1Decimals?: number
  tokenSymbol?: string
  tokenName?: string
  tokenDecimals?: number
  lpSymbol?: string
  contractHashArrayHex?: string
}

export interface InspectPoolResult {
  entry: PoolRegistryEntry
  onChain: {
    poolType: 'space-token' | 'token-token'
    r1: string
    r2: string
    lp: string
    feeRate: number
    projFeeRate: number
    minProjFee: string
    ownerAddressHash: string
    poolTokenVout: number
    poolToken2Vout?: number
  }
}

/**
 * Fetches an existing on-chain MVCSwap pool head (auto-detecting `space-token` vs `token-token`),
 * verifies it strictly against whitelistSwapCodeHashes + cryptographic contractHashRoot +
 * FetchToken + LpGenesis bindings, and derives a complete PoolRegistryEntry automatically.
 */
export async function inspectOnChainPoolAndBuildEntry(params: {
  chain: any
  contracts: any
  whitelistSwapCodeHashes: string[]
  input: ImportPoolInput
}): Promise<InspectPoolResult> {
  const { chain, contracts, whitelistSwapCodeHashes, input } = params
  const poolTxid = normalizeHex(input.poolTxid, 32, 'poolTxid')
  const lpGenesisTxid = normalizeHex(input.lpGenesisTxid, 32, 'lpGenesisTxid')
  const lpGenesisVout = input.lpGenesisVout !== undefined ? Number(input.lpGenesisVout) : 2
  if (!Number.isInteger(lpGenesisVout) || lpGenesisVout < 0) {
    throw new Error('lpGenesisVout must be a non-negative integer')
  }

  // Probe poolTx outputs to auto-detect whether this pool holds 1 FT (space-token) or 2 FTs (token-token)
  const poolTx = await chain.getTx(poolTxid)
  const poolScriptBuf: Buffer = poolTx.outputs[0].script.toBuffer()
  const parsed = SwapEngine.parseSwapMainScript(poolScriptBuf)
  const fetchHashHex = parsed.fetchTokenHash.toString('hex')
  const heldTokenVouts: number[] = []
  for (let i = 1; i < poolTx.outputs.length; i++) {
    const b: Buffer = poolTx.outputs[i].script.toBuffer()
    if (b.length > TokenProto.DATA_LEN) {
      try {
        if (TokenProto.getTokenAddress(b).toString('hex') === fetchHashHex) {
          heldTokenVouts.push(i)
        }
      } catch (_) {}
    }
  }

  if (heldTokenVouts.length === 2) {
    return inspectTokenSwapPoolAndBuildEntry({
      chain,
      whitelistSwapCodeHashes,
      input,
      poolTxid,
      lpGenesisTxid,
      lpGenesisVout,
    })
  }
  if (heldTokenVouts.length !== 1) {
    throw new Error(
      `Expected 1 (space-token) or 2 (token-token) pool token outputs locked to ${fetchHashHex} in ${poolTxid}, found ${heldTokenVouts.length}`
    )
  }

  const state = await SwapEngine.loadPoolState(chain, poolTxid, lpGenesisTxid, lpGenesisVout)
  const swapCodeHash = state.swapCodeHash.toString('hex')
  if (!whitelistSwapCodeHashes.includes(swapCodeHash)) {
    throw new Error(
      `Pool swapCodeHash ${swapCodeHash} is NOT in whitelistSwapCodeHashes (${whitelistSwapCodeHashes.join(', ')})`
    )
  }

  if (state.r1 <= BigInt(0) || state.r2 <= BigInt(0) || state.lp <= BigInt(0)) {
    throw new Error(
      `Pool has zero reserves (r1=${state.r1}, r2=${state.r2}, lp=${state.lp}); initialize liquidity before importing`
    )
  }

  const swapContractID = state.swapContractID.toString('hex')
  const swapSensibleID = state.swapSensibleID.toString('hex')
  const fetchTokenHash = state.fetchTokenHash.toString('hex')
  const contractHashRoot = state.contractHashRoot.toString('hex')
  const lpTokenID = state.lpTokenID.toString('hex')
  const ownerAddressHash = state.ownerAddress.toString('hex')

  // Verify FetchToken script hash binding
  const fetchTokInst = new contracts.SwapFetchToken(new Bytes(swapContractID))
  fetchTokInst.setDataPart(Common.buildScriptData(Buffer.from(swapCodeHash, 'hex')).toString('hex'))
  const derivedFetchHash = Common.hash160(fetchTokInst.lockingScript.toBuffer()).toString('hex')
  if (derivedFetchHash !== fetchTokenHash) {
    throw new Error(`FetchToken hash mismatch: expected ${derivedFetchHash}, on-chain ${fetchTokenHash}`)
  }

  // Extract token2 metadata directly from on-chain poolToken output
  const poolTokBuf: Buffer = state.poolTokenScriptBuf
  const tokenID = TokenProto.getTokenID(poolTokBuf).toString('hex')
  const tokenScriptCodeHash = TokenProto.getScriptCodeHash(poolTokBuf).toString('hex')
  const parsedSymbol = decodeCString(TokenProto.getTokenSymbol(poolTokBuf)) || 'TOKEN'
  const parsedName = decodeCString(TokenProto.getTokenName(poolTokBuf)) || parsedSymbol
  const parsedDecimals = TokenProto.getTokenDecimal(poolTokBuf)

  const tokenSymbol = (input.tokenSymbol || parsedSymbol).trim()
  const tokenName = (input.tokenName || parsedName).trim()
  const tokenDecimals = input.tokenDecimals !== undefined ? Number(input.tokenDecimals) : parsedDecimals

  // Verify LP Genesis output matches pool's lpTokenID
  const lpGenBuf: Buffer = state.lpGenesisScriptBuf
  const lpGenTxidBuf: Buffer = TokenProto.getGenesisTxid(lpGenBuf)
  const resolvedLpSensibleBuf = lpGenTxidBuf.equals(Buffer.alloc(36, 0))
    ? Buffer.from(Common.genGenesisTxid(lpGenesisTxid, lpGenesisVout), 'hex')
    : lpGenTxidBuf
  const canonicalLpGenBuf = TokenProto.getNewGenesisScript(lpGenBuf, resolvedLpSensibleBuf)
  const lpGenHashBuf = Common.hash160(canonicalLpGenBuf)
  const derivedLpTokenID = Common.hash160(Buffer.concat([lpGenHashBuf, resolvedLpSensibleBuf])).toString('hex')
  if (derivedLpTokenID !== lpTokenID) {
    throw new Error(
      `LP Genesis (${lpGenesisTxid}:${lpGenesisVout}) yields lpTokenID=${derivedLpTokenID}, expected pool lpTokenID=${lpTokenID}`
    )
  }
  const lpDecimals = TokenProto.getTokenDecimal(lpGenBuf)

  // Verify contractHashArray -> contractHashRoot cryptographic commitment
  let contractHashArrayHex: string
  if (input.contractHashArrayHex) {
    contractHashArrayHex = normalizeHex(input.contractHashArrayHex, 120, 'contractHashArrayHex')
    const checkRoot = Common.hash160(Buffer.from(contractHashArrayHex, 'hex')).toString('hex')
    if (checkRoot !== contractHashRoot) {
      throw new Error(
        `Provided contractHashArrayHex hashes to ${checkRoot}, which does not match on-chain contractHashRoot ${contractHashRoot}`
      )
    }
  } else {
    const derived = deriveContractHashArray({
      contracts,
      swapContractID,
      tokenID,
      tokenScriptCodeHash,
      ownerAddressHashHex: ownerAddressHash,
    })
    if (derived.contractHashRootHex !== contractHashRoot) {
      throw new Error(
        `Derived standard contractHashRoot (${derived.contractHashRootHex}) does not match on-chain contractHashRoot (${contractHashRoot}). Provide --contract-hash-array if this pool uses a custom admin/op contract.`
      )
    }
    contractHashArrayHex = derived.contractHashArrayHex
  }

  const slugSymbol = tokenSymbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || tokenID.slice(0, 8)
  const poolId = (input.poolId || `space-${slugSymbol}`).trim()
  if (!/^[a-zA-Z0-9._-]{2,64}$/.test(poolId)) {
    throw new Error(`Invalid poolId "${poolId}": use 2-64 alphanumeric, dot, underscore, or dash characters`)
  }
  const pairName = (input.pairName || `SPACE / ${tokenSymbol}`).trim()
  const lpSymbol = (input.lpSymbol || `SPACE-${tokenSymbol} LP`).trim()

  const entry: PoolRegistryEntry = {
    poolId,
    pairName,
    poolType: 'space-token',
    verified: true,
    token1: {
      symbol: 'SPACE',
      name: 'MicroVisionChain',
      decimals: 8,
    },
    token2: {
      symbol: tokenSymbol,
      name: tokenName,
      decimals: tokenDecimals,
      tokenID,
      codeHash: tokenScriptCodeHash,
    },
    lpToken: {
      symbol: lpSymbol,
      decimals: lpDecimals,
      tokenID: lpTokenID,
      codeHash: tokenScriptCodeHash,
    },
    contract: {
      swapContractID,
      swapSensibleID,
      tokenID,
      tokenScriptCodeHash,
      lpTokenID,
      fetchTokenHash,
      swapCodeHash,
      contractHashRoot,
      contractHashArrayHex,
    },
    bootstrapHead: {
      poolTxid,
      lpGenesisTxid,
      lpGenesisVout,
    },
  }

  return {
    entry,
    onChain: {
      poolType: 'space-token',
      r1: state.r1.toString(),
      r2: state.r2.toString(),
      lp: state.lp.toString(),
      feeRate: state.feeRate,
      projFeeRate: state.projFeeRate,
      minProjFee: state.minProjFee.toString(),
      ownerAddressHash,
      poolTokenVout: state.poolTokenVout,
    },
  }
}

async function inspectTokenSwapPoolAndBuildEntry(params: {
  chain: any
  whitelistSwapCodeHashes: string[]
  input: ImportPoolInput
  poolTxid: string
  lpGenesisTxid: string
  lpGenesisVout: number
}): Promise<InspectPoolResult> {
  const { chain, whitelistSwapCodeHashes, input, poolTxid, lpGenesisTxid, lpGenesisVout } = params
  const tsContracts = loadTokenSwapContractsFast()
  const state = await TokenSwapEngine.loadTokenSwapPoolState(chain, poolTxid, lpGenesisTxid, lpGenesisVout)

  const swapCodeHash = state.swapCodeHash.toString('hex')
  if (!whitelistSwapCodeHashes.includes(swapCodeHash)) {
    throw new Error(
      `Pool swapCodeHash ${swapCodeHash} is NOT in whitelistSwapCodeHashes (${whitelistSwapCodeHashes.join(', ')})`
    )
  }
  if (state.r1 <= BigInt(0) || state.r2 <= BigInt(0) || state.lp <= BigInt(0)) {
    throw new Error(
      `Token/Token pool has zero reserves (r1=${state.r1}, r2=${state.r2}, lp=${state.lp}); initialize liquidity before importing`
    )
  }

  const swapContractID = state.swapContractID.toString('hex')
  const swapSensibleID = state.swapSensibleID.toString('hex')
  const fetchTokenHash = state.fetchTokenHash.toString('hex')
  const contractHashRoot = state.contractHashRoot.toString('hex')
  const lpTokenID = state.lpTokenID.toString('hex')
  const ownerAddressHash = state.ownerAddress.toString('hex')

  // Verify tokenSwapFetchToken binding
  const fetchTokInst = new tsContracts.SwapFetchToken(new Bytes(swapContractID))
  fetchTokInst.setDataPart(Common.buildScriptData(Buffer.from(swapCodeHash, 'hex')).toString('hex'))
  const derivedFetchHash = Common.hash160(fetchTokInst.lockingScript.toBuffer()).toString('hex')
  if (derivedFetchHash !== fetchTokenHash) {
    throw new Error(`tokenSwap FetchToken hash mismatch: expected ${derivedFetchHash}, on-chain ${fetchTokenHash}`)
  }

  const tok1Buf: Buffer = state.poolToken1ScriptBuf
  const tok2Buf: Buffer = state.poolToken2ScriptBuf
  const token1ID = TokenProto.getTokenID(tok1Buf).toString('hex')
  const token1CodeHash = TokenProto.getScriptCodeHash(tok1Buf).toString('hex')
  const token2ID = TokenProto.getTokenID(tok2Buf).toString('hex')
  const token2CodeHash = TokenProto.getScriptCodeHash(tok2Buf).toString('hex')

  const parsedSym1 = decodeCString(TokenProto.getTokenSymbol(tok1Buf)) || 'TOK1'
  const parsedName1 = decodeCString(TokenProto.getTokenName(tok1Buf)) || parsedSym1
  const parsedDec1 = TokenProto.getTokenDecimal(tok1Buf)

  const parsedSym2 = decodeCString(TokenProto.getTokenSymbol(tok2Buf)) || 'TOK2'
  const parsedName2 = decodeCString(TokenProto.getTokenName(tok2Buf)) || parsedSym2
  const parsedDec2 = TokenProto.getTokenDecimal(tok2Buf)

  const token1Symbol = (input.token1Symbol || parsedSym1).trim()
  const token1Name = (input.token1Name || parsedName1).trim()
  const token1Decimals = input.token1Decimals !== undefined ? Number(input.token1Decimals) : parsedDec1

  const token2Symbol = (input.tokenSymbol || parsedSym2).trim()
  const token2Name = (input.tokenName || parsedName2).trim()
  const token2Decimals = input.tokenDecimals !== undefined ? Number(input.tokenDecimals) : parsedDec2

  // Verify LP Genesis output matches pool's lpTokenID
  const lpGenBuf: Buffer = state.lpGenesisScriptBuf
  const lpGenTxidBuf: Buffer = TokenProto.getGenesisTxid(lpGenBuf)
  const resolvedLpSensibleBuf = lpGenTxidBuf.equals(Buffer.alloc(36, 0))
    ? Buffer.from(Common.genGenesisTxid(lpGenesisTxid, lpGenesisVout), 'hex')
    : lpGenTxidBuf
  const canonicalLpGenBuf = TokenProto.getNewGenesisScript(lpGenBuf, resolvedLpSensibleBuf)
  const lpGenHashBuf = Common.hash160(canonicalLpGenBuf)
  const derivedLpTokenID = Common.hash160(Buffer.concat([lpGenHashBuf, resolvedLpSensibleBuf])).toString('hex')
  if (derivedLpTokenID !== lpTokenID) {
    throw new Error(
      `LP Genesis (${lpGenesisTxid}:${lpGenesisVout}) yields lpTokenID=${derivedLpTokenID}, expected pool lpTokenID=${lpTokenID}`
    )
  }
  const lpDecimals = TokenProto.getTokenDecimal(lpGenBuf)

  let contractHashArrayHex: string
  if (input.contractHashArrayHex) {
    contractHashArrayHex = normalizeHex(input.contractHashArrayHex, 120, 'contractHashArrayHex')
    const checkRoot = Common.hash160(Buffer.from(contractHashArrayHex, 'hex')).toString('hex')
    if (checkRoot !== contractHashRoot) {
      throw new Error(
        `Provided contractHashArrayHex hashes to ${checkRoot}, which does not match on-chain contractHashRoot ${contractHashRoot}`
      )
    }
  } else {
    const derived = deriveTokenSwapContractHashArray({
      tokenSwapContracts: tsContracts,
      swapContractID,
      token1ID,
      token1ScriptCodeHash: token1CodeHash,
      token2ID,
      token2ScriptCodeHash: token2CodeHash,
      ownerAddressHashHex: ownerAddressHash,
    })
    if (derived.contractHashRootHex !== contractHashRoot) {
      throw new Error(
        `Derived standard tokenSwap contractHashRoot (${derived.contractHashRootHex}) does not match on-chain contractHashRoot (${contractHashRoot}).`
      )
    }
    contractHashArrayHex = derived.contractHashArrayHex
  }

  const slug1 = token1Symbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || token1ID.slice(0, 6)
  const slug2 = token2Symbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || token2ID.slice(0, 6)
  const poolId = (input.poolId || `${slug1}-${slug2}`).trim()
  if (!/^[a-zA-Z0-9._-]{2,64}$/.test(poolId)) {
    throw new Error(`Invalid poolId "${poolId}": use 2-64 alphanumeric, dot, underscore, or dash characters`)
  }
  const pairName = (input.pairName || `${token1Symbol} / ${token2Symbol}`).trim()
  const lpSymbol = (input.lpSymbol || `${token1Symbol}-${token2Symbol} LP`).trim()

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
      codeHash: token1CodeHash,
    },
    token2: {
      symbol: token2Symbol,
      name: token2Name,
      decimals: token2Decimals,
      tokenID: token2ID,
      codeHash: token2CodeHash,
    },
    lpToken: {
      symbol: lpSymbol,
      decimals: lpDecimals,
      tokenID: lpTokenID,
      codeHash: token1CodeHash,
    },
    contract: {
      swapContractID,
      swapSensibleID,
      tokenID: token2ID,
      tokenScriptCodeHash: token2CodeHash,
      token1ID,
      token1ScriptCodeHash: token1CodeHash,
      token2ID,
      token2ScriptCodeHash: token2CodeHash,
      lpTokenID,
      fetchTokenHash,
      swapCodeHash,
      contractHashRoot,
      contractHashArrayHex,
    },
    bootstrapHead: {
      poolTxid,
      lpGenesisTxid,
      lpGenesisVout,
    },
  }

  return {
    entry,
    onChain: {
      poolType: 'token-token',
      r1: state.r1.toString(),
      r2: state.r2.toString(),
      lp: state.lp.toString(),
      feeRate: state.feeRate,
      projFeeRate: state.projFeeRate,
      minProjFee: state.minProjFee.toString(),
      ownerAddressHash,
      poolTokenVout: state.poolToken1Vout,
      poolToken2Vout: state.poolToken2Vout,
    },
  }
}

export interface CreatePoolOnChainParams {
  chain: any
  contracts: any
  whitelistSwapCodeHashes: string[]
  adminWif: string
  tokenID: string
  tokenCodeHash: string
  initSpaceSats: bigint
  initTokenAmount: bigint
  poolId?: string
  pairName?: string
  tokenSymbol?: string
  tokenName?: string
  lpSymbol?: string
  feeRate?: number
  projFeeRate?: number
  minProjFee?: bigint
  projFeeAddress?: string
  feeRateSatPerByte?: number
  dryRun?: boolean
  onProgress?: (msg: string) => void
}

export interface CreatePoolOnChainResult {
  dryRun: boolean
  adminAddress: string
  entry: PoolRegistryEntry
  txids: {
    splitTxid: string
    lpGenesisTxid: string
    swapGenesisTxid: string
    prepChecksTxid: string
    depositTokenTxid: string
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
 * End-to-end admin pool deployment on MVC:
 *   1. T4a: Split funding into F5 (SwapMain anchor) + T7 funding + operational change
 *   2. T7:  Deploy SwapLpGenesis (bound to swapContractID / swapCodeHash / fetchTokenHash)
 *   3. T5:  Deploy SwapMain genesis (reserves = 0, bound to F5 + lpTokenID + contractHashRoot)
 *   4. T6b: Fund TokenTransferCheck + TokenUnlockContractCheck
 *   5. T6c: Transfer initial FT amount from admin wallet -> AddLiquidity opHash (with FT change if any)
 *   6. T6a: Fund AddLiquidity op UTXO
 *   7. T6d: First AddLiquidity (6-input genesis ceremony, verifies every contract input offline via .verify())
 *
 * Defaults to dryRun=true (builds & verifies all 7 txs offline against real wallet UTXOs without broadcasting).
 */
export async function deployNewPoolOnChain(params: CreatePoolOnChainParams): Promise<CreatePoolOnChainResult> {
  const { chain, contracts, whitelistSwapCodeHashes } = params
  const log = params.onProgress || (() => {})
  const dryRun = params.dryRun !== false

  const tokenID = normalizeHex(params.tokenID, 20, 'tokenID')
  const tokenCodeHash = normalizeHex(params.tokenCodeHash, 20, 'tokenCodeHash')
  const initSpaceSats = BigInt(params.initSpaceSats)
  const initTokenAmount = BigInt(params.initTokenAmount)
  if (initSpaceSats < BigInt(10_000)) {
    throw new Error('initSpaceSats must be >= 10,000 sats')
  }
  if (initTokenAmount <= BigInt(0)) {
    throw new Error('initTokenAmount must be > 0')
  }

  const feeRate = params.feeRate !== undefined ? Number(params.feeRate) : 25
  const projFeeRate = params.projFeeRate !== undefined ? Number(params.projFeeRate) : 5
  const minProjFee = params.minProjFee !== undefined ? BigInt(params.minProjFee) : BigInt(1000)

  const privKey = mvc.PrivateKey.fromWIF(params.adminWif)
  const signer = new SwapEngine.LocalMetaletSigner(params.adminWif)
  const adminAddrObj = privKey.toAddress(mvc.Networks.livenet)
  const adminAddress: string = adminAddrObj.toString()
  const ownerHashBuf: Buffer = adminAddrObj.hashBuffer
  const projAddrObj = params.projFeeAddress
    ? mvc.Address.fromString(params.projFeeAddress)
    : adminAddrObj
  const p2pkhScript = mvc.Script.buildPublicKeyHashOut(adminAddrObj)

  const feeRateSatPerByte = params.feeRateSatPerByte || (await chain.getFeeRate())
  log(`Admin address: ${adminAddress} (feeRate=${feeRateSatPerByte} sat/B, mode=${dryRun ? 'DRY-RUN' : 'LIVE'})`)

  // 1. Resolve admin's live FT UTXO & verify real on-chain tokenScriptCodeHash
  const tokenUtxos: any[] = await chain.resolveUserTokenUtxos(adminAddress, tokenCodeHash, tokenID)
  const userTokenUtxo = tokenUtxos
    .filter((u) => u.amount >= initTokenAmount)
    .sort((a, b) => (b.amount > a.amount ? 1 : -1))[0]
  if (!userTokenUtxo) {
    const maxHave = tokenUtxos.reduce((m, u) => (u.amount > m ? u.amount : m), BigInt(0))
    throw new Error(
      `Admin address ${adminAddress} has no single FT UTXO >= ${initTokenAmount} for tokenID=${tokenID} (largest=${maxHave}). Merge FT UTXOs first if needed.`
    )
  }

  const realTokenScriptCodeHash = TokenProto.getScriptCodeHash(userTokenUtxo.scriptBuf).toString('hex')
  if (realTokenScriptCodeHash !== tokenCodeHash) {
    throw new Error(
      `On-chain tokenScriptCodeHash mismatch: expected ${tokenCodeHash}, actual ${realTokenScriptCodeHash}`
    )
  }

  const parsedSymbol = decodeCString(TokenProto.getTokenSymbol(userTokenUtxo.scriptBuf)) || 'TOKEN'
  const parsedName = decodeCString(TokenProto.getTokenName(userTokenUtxo.scriptBuf)) || parsedSymbol
  const tokenDecimals = TokenProto.getTokenDecimal(userTokenUtxo.scriptBuf)
  const tokenSymbol = (params.tokenSymbol || parsedSymbol).trim()
  const tokenName = (params.tokenName || parsedName).trim()
  const lpSymbol = (params.lpSymbol || `SPACE-${tokenSymbol} LP`).trim()
  const lpName = `${tokenName} LP`

  // 2. Fetch admin's SPACE UTXOs
  const spaceUtxos: any[] = await chain.getSpaceUtxos(adminAddress)
  const totalSpace = spaceUtxos.reduce((s, u) => s + BigInt(u.satoshis), BigInt(0))
  const requiredMinSpace = initSpaceSats + BigInt(1_100_000)
  if (totalSpace < requiredMinSpace) {
    throw new Error(
      `Insufficient SPACE on admin address ${adminAddress}: have ${totalSpace} sats, need at least ${requiredMinSpace} sats (initSpace + contract dust + fees)`
    )
  }

  // 3. T4a: Split funding into out0=F5 (300,000 sats), out1=T7 (300,000 sats), out2=change
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
  const swapCodeHash = getCodeHash(new contracts.SwapMain().lockingScript.toBuffer()).toString('hex')
  if (!whitelistSwapCodeHashes.includes(swapCodeHash)) {
    throw new Error(
      `Compiled SwapMain codeHash ${swapCodeHash} is not in whitelistSwapCodeHashes (${whitelistSwapCodeHashes.join(', ')})`
    )
  }

  const fetchTokInst = new contracts.SwapFetchToken(new Bytes(swapContractID))
  fetchTokInst.setDataPart(Common.buildScriptData(Buffer.from(swapCodeHash, 'hex')).toString('hex'))
  const fetchTokenHash = Common.hash160(fetchTokInst.lockingScript.toBuffer()).toString('hex')

  // 4. T7: Deploy SwapLpGenesis
  const lpGenesisInst = new contracts.SwapLpGenesis(
    new Bytes(swapContractID),
    new Bytes(swapCodeHash),
    new Bytes(fetchTokenHash)
  )
  const lpData = Common.buildScriptData(
    Buffer.concat([
      encodeFixedString(lpName, TokenProto.TOKEN_NAME_LEN),
      encodeFixedString(lpSymbol, TokenProto.TOKEN_SYMBOL_LEN),
      Buffer.from([tokenDecimals & 0xff]),
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

  // 5. Derive contractHashArray + contractHashRoot
  const { contractHashArrayHex, contractHashRootHex } = deriveContractHashArray({
    contracts,
    swapContractID,
    tokenID,
    tokenScriptCodeHash: realTokenScriptCodeHash,
    ownerAddressHashHex: ownerHashBuf.toString('hex'),
  })

  // 6. T5: Deploy SwapMain genesis (r1=0, r2=0, lp=0)
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

  // 7. Build AddLiquidity op + TransferCheck + UnlockContractCheck for first addLiquidity
  const addLiqInst = new contracts.SwapAddLiquidity(
    new Bytes(swapContractID),
    new Bytes(tokenID),
    new Bytes(realTokenScriptCodeHash)
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

  const tokChangeAmt = userTokenUtxo.amount - initTokenAmount
  const transferCheck = SwapEngine.buildTokenTransferCheck(
    contracts,
    Buffer.from(tokenID, 'hex'),
    Buffer.from(realTokenScriptCodeHash, 'hex'),
    1,
    tokChangeAmt > BigInt(0) ? [initTokenAmount, tokChangeAmt] : [initTokenAmount],
    tokChangeAmt > BigInt(0) ? [addLiqHash, ownerHashBuf] : [addLiqHash]
  )
  const unlockCheck = SwapEngine.buildTokenUnlockContractCheck(
    contracts,
    Buffer.from(tokenID, 'hex'),
    Buffer.from(realTokenScriptCodeHash, 'hex'),
    [3],
    [initTokenAmount],
    [Buffer.from(fetchTokenHash, 'hex')]
  )

  // T6b: Fund transferCheck (vout 0), unlockCheck (vout 1), and keep P2PKH change (vout 2)
  const t6bPrep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [
      { txid: t4aPrep.tx.id, vout: t4aPrep.changeVout, satoshis: t4aPrep.changeSatoshis, height: 0 },
      { txid: t7Prep.tx.id, vout: t7Prep.changeVout, satoshis: t7Prep.changeSatoshis, height: 0 },
      { txid: t5Prep.tx.id, vout: t5Prep.changeVout, satoshis: t5Prep.changeSatoshis, height: 0 },
    ],
    [
      { script: transferCheck.lockingScript, satoshis: CONTRACT_DUST },
      { script: unlockCheck.lockingScript, satoshis: CONTRACT_DUST },
    ],
    feeRateSatPerByte
  )
  chain.seedRawTx(t6bPrep.tx.id, t6bPrep.rawHex)

  // T6c: Transfer initial FT -> addLiqHash (with optional token change back to admin)
  const t6cDep = await SwapEngine.buildSignedTokenTransferTx(
    contracts,
    chain,
    signer,
    adminAddress,
    userTokenUtxo,
    transferCheck,
    t6bPrep.tx,
    0,
    CONTRACT_DUST,
    t6bPrep.tx.id,
    t6bPrep.changeVout,
    t6bPrep.changeSatoshis,
    addLiqHash,
    initTokenAmount,
    feeRateSatPerByte
  )
  chain.seedRawTx(t6cDep.tx.id, t6cDep.rawHex)
  const depTokScriptBuf: Buffer = t6cDep.tx.outputs[0].script.toBuffer()

  // T6a: Fund AddLiquidity op UTXO (vout 0) and keep P2PKH change (vout 1) for T6d
  const t6aPrep = await SwapEngine.buildSignedP2PKHPrepTx(
    signer,
    adminAddress,
    [{ txid: t6cDep.tx.id, vout: t6cDep.spaceChangeVout, satoshis: t6cDep.spaceChangeSatoshis, height: 0 }],
    [{ script: addLiqInst.lockingScript, satoshis: CONTRACT_DUST }],
    feeRateSatPerByte
  )
  chain.seedRawTx(t6aPrep.tx.id, t6aPrep.rawHex)

  // 8. T6d: First AddLiquidity (6 inputs, verified offline with two-pass fee calibration)
  const lpMinted = initSpaceSats
  const swapScriptV2Buf: Buffer = TokenSwapProto.getNewSwapScript(
    t5PoolScriptBuf,
    initSpaceSats,
    initTokenAmount,
    lpMinted
  )
  // Construct LP token output script using exact same token code prefix as userTokenUtxo
  const lpTokenBaseScript = Buffer.concat([
    TokenProto.getScriptCode(depTokScriptBuf),
    TokenProto.getScriptData(newLpGenScriptBuf),
  ])
  const newLpTokenScriptBuf = Buffer.concat([
    lpTokenBaseScript.subarray(0, lpTokenBaseScript.length - TokenProto.TOKEN_ADDRESS_OFFSET),
    ownerHashBuf,
    Common.getUInt64Buf(lpMinted),
    lpGenesisHashBuf,
    lpTokenBaseScript.subarray(lpTokenBaseScript.length - TokenProto.GENESIS_HASH_OFFSET + ProtoHeader.HASH_LEN),
  ])
  if (TokenProto.getTokenID(newLpTokenScriptBuf).toString('hex') !== lpTokenID) {
    throw new Error('Computed first LP tokenID mismatch')
  }
  const poolTokenScriptBuf: Buffer = TokenProto.getNewTokenScript(
    depTokScriptBuf,
    Buffer.from(fetchTokenHash, 'hex'),
    initTokenAmount
  )

  const depTokInst = SwapEngine.instantiateTokenFromChain(contracts, depTokScriptBuf)
  const depTokBacktrace = await SwapEngine.resolveTokenBacktrace(chain, t6cDep.tx, 0)
  const lpGenVerifyInst = new contracts.SwapLpGenesis(
    new Bytes(swapContractID),
    new Bytes(swapCodeHash),
    new Bytes(fetchTokenHash)
  )
  lpGenVerifyInst.setDataPart(lpData.toString('hex'))
  if (!lpGenVerifyInst.lockingScript.toBuffer().equals(t7LpGenesisBuf)) {
    throw new Error('LpGenesis instance lockingScript mismatch')
  }

  const swapVerifyInst = new contracts.SwapMain()
  swapVerifyInst.setDataPart(poolData.toString('hex'))
  if (!swapVerifyInst.lockingScript.toBuffer().equals(t5PoolScriptBuf)) {
    throw new Error('SwapMain instance lockingScript mismatch')
  }

  let t6dFee = Math.max(52_000, Math.ceil(46_000 * feeRateSatPerByte))
  let t6dTx: any = null
  let t6dRawHex = ''

  for (let pass = 0; pass < 2; pass++) {
    const p2pkhInSat = t6aPrep.changeSatoshis
    const inTotal =
      CONTRACT_DUST + // 0: addLiq
      t5Prep.tx.outputs[0].satoshis + // 1: swapMain genesis
      t7Prep.tx.outputs[0].satoshis + // 2: lpGenesis
      t6cDep.tx.outputs[0].satoshis + // 3: deposited FT
      p2pkhInSat + // 4: admin P2PKH
      CONTRACT_DUST // 5: unlockCheck
    const outContractTotal =
      t5Prep.tx.outputs[0].satoshis + Number(initSpaceSats) + // 0: updated swapMain
      CONTRACT_DUST + // 1: minted LP token
      CONTRACT_DUST + // 2: updated LP genesis
      CONTRACT_DUST // 3: poolToken
    const dChange = inTotal - outContractTotal - t6dFee
    if (dChange < DUST) {
      throw new Error(`Insufficient SPACE for T6d first addLiquidity: change=${dChange}, fee=${t6dFee}`)
    }

    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    const prevouts: Buffer[] = []
    Common.addInput(tx, t6aPrep.tx.id, 0, addLiqInst.lockingScript, CONTRACT_DUST, prevouts) // 0: AddLiquidity op
    Common.addInput(tx, t5Prep.tx.id, 0, t5Prep.tx.outputs[0].script, t5Prep.tx.outputs[0].satoshis, prevouts) // 1: SwapMain genesis
    Common.addInput(tx, t7Prep.tx.id, 0, t7Prep.tx.outputs[0].script, t7Prep.tx.outputs[0].satoshis, prevouts) // 2: LpGenesis
    Common.addInput(tx, t6cDep.tx.id, 0, mvc.Script.fromBuffer(depTokScriptBuf), t6cDep.tx.outputs[0].satoshis, prevouts) // 3: deposited FT
    Common.addInput(tx, t6aPrep.tx.id, t6aPrep.changeVout, p2pkhScript, p2pkhInSat, prevouts, true) // 4: admin P2PKH
    Common.addInput(tx, t6bPrep.tx.id, 1, unlockCheck.lockingScript, CONTRACT_DUST, prevouts) // 5: UnlockContractCheck

    Common.addOutput(tx, mvc.Script.fromBuffer(swapScriptV2Buf), t5Prep.tx.outputs[0].satoshis + Number(initSpaceSats)) // 0
    Common.addOutput(tx, mvc.Script.fromBuffer(newLpTokenScriptBuf), CONTRACT_DUST) // 1
    Common.addOutput(tx, mvc.Script.fromBuffer(newLpGenScriptBuf), CONTRACT_DUST) // 2
    Common.addOutput(tx, mvc.Script.fromBuffer(poolTokenScriptBuf), CONTRACT_DUST) // 3
    Common.addOutput(tx, p2pkhScript, dChange) // 4

    const prevoutsBuf = Buffer.concat(prevouts)
    const { TxInputProof, TxOutputProof } = contracts

    // Input 0: AddLiquidity op (isFirst = true)
    {
      const preimage = getPreimage(tx, addLiqInst.lockingScript.subScript(0), CONTRACT_DUST, 0, Common.SIG_HASH_ALL)
      const swapProof = new TxOutputProof(Common.getTxOutputProof(t5Prep.tx, 0))
      const lpProof = new TxOutputProof(Common.getTxOutputProof(t7Prep.tx, 0))
      const tokenProof = new TxOutputProof(Common.getTxOutputProof(t6cDep.tx, 0))
      const empty = new TxOutputProof(Common.getEmptyTxOutputProof())
      const res = addLiqInst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        new Bytes(t5PoolScriptBuf.toString('hex')),
        swapProof.txHeader,
        swapProof.hashProof,
        swapProof.satoshiBytes,
        new Bytes(t7LpGenesisBuf.toString('hex')),
        lpProof.txHeader,
        lpProof.hashProof,
        lpProof.satoshiBytes,
        new Bytes(depTokScriptBuf.toString('hex')),
        tokenProof.txHeader,
        tokenProof.hashProof,
        tokenProof.satoshiBytes,
        empty.txHeader,
        empty.hashProof,
        empty.satoshiBytes,
        Number(initSpaceSats),
        CONTRACT_DUST,
        CONTRACT_DUST,
        dChange,
        new Ripemd160(ownerHashBuf.toString('hex')),
        true
      )
      const v = res.verify({ tx, inputIndex: 0, inputSatoshis: CONTRACT_DUST })
      if (!v.success) throw new Error(`T6d input 0 (AddLiquidity) verify failed: ${v.error}`)
      tx.inputs[0].setScript(res.unlockingScript)
    }

    // Input 1: SwapMain genesis unlock
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
      if (!v.success) throw new Error(`T6d input 1 (SwapMain) verify failed: ${v.error}`)
      tx.inputs[1].setScript(res.unlockingScript)
    }

    // Input 2: LpGenesis first mint unlock (SIG_HASH_SINGLE)
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
      if (!v.success) throw new Error(`T6d input 2 (LpGenesis) verify failed: ${v.error}`)
      tx.inputs[2].setScript(res.unlockingScript)
    }

    // Input 3: Token op=2 unlock-from-contract
    {
      const inSat = t6cDep.tx.outputs[0].satoshis
      const preimage = getPreimage(tx, mvc.Script.fromBuffer(depTokScriptBuf), inSat, 3, Common.SIG_HASH_ALL)
      const [depInProof, depTxHeader] = Common.getTxInputProof(t6cDep.tx, depTokBacktrace.prevTokenInputIndex)
      const prevTokOutProof = Common.getTxOutputProof(depTokBacktrace.prevTokTx, depTokBacktrace.prevTokVout)
      const res = depTokInst.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        0, // tokenInputIndex inside UnlockContractCheck
        0, // amountCheckHashIndex
        5, // amountCheckInputIndex
        new TxOutputProof(Common.getTxOutputProof(t6bPrep.tx, 1)),
        new Bytes(unlockCheck.lockingScript.toBuffer().toString('hex')),
        depTokBacktrace.prevTokenInputIndex,
        new Bytes(depTokBacktrace.prevTokenAddress.toString('hex')),
        depTokBacktrace.prevTokenAmount,
        depTxHeader,
        new TxInputProof(depInProof),
        new TxOutputProof(prevTokOutProof),
        new PubKey(Buffer.alloc(33, 0).toString('hex')),
        new Sig(Buffer.alloc(72, 0).toString('hex')),
        0, // contractInputIndex (AddLiquidity op)
        new TxOutputProof(Common.getTxOutputProof(t6aPrep.tx, 0)),
        TokenProto.OP_UNLOCK_FROM_CONTRACT
      )
      const v = res.verify({ tx, inputIndex: 3, inputSatoshis: inSat })
      if (!v.success) throw new Error(`T6d input 3 (Token op=2) verify failed: ${v.error}`)
      tx.inputs[3].setScript(res.unlockingScript)
    }

    // Input 5: TokenUnlockContractCheck (single token input -> poolToken output at index 3)
    {
      const preimage = getPreimage(tx, unlockCheck.lockingScript.subScript(0), CONTRACT_DUST, 5, Common.SIG_HASH_ALL)
      const proof = Common.getTxOutputProof(t6cDep.tx, 0)
      const headerBuf = Buffer.from(proof.txHeader.serialize(), 'hex')
      const hashProofBuf = Buffer.from(proof.hashProof.serialize(), 'hex')
      const satBuf = Buffer.from(proof.satoshiBytes.serialize(), 'hex')
      let otherOutputArray = Buffer.alloc(0)
      for (const idx of [0, 1, 2, 4]) {
        const o = tx.outputs[idx]
        const ob = Common.buildOutput(o.script.toBuffer(), o.satoshis)
        otherOutputArray = Buffer.concat([otherOutputArray, Common.getUInt32Buf(ob.length), ob])
      }
      const res = unlockCheck.unlock(
        new SigHashPreimage(toHex(preimage)),
        new Bytes(prevoutsBuf.toString('hex')),
        new Bytes(depTokScriptBuf.toString('hex')),
        new Bytes(headerBuf.toString('hex')),
        new Bytes(Common.getUInt32Buf(hashProofBuf.length).toString('hex') + hashProofBuf.toString('hex')),
        new Bytes(satBuf.toString('hex')),
        new Bytes(addLiqHash.toString('hex')),
        new Bytes(Common.getUInt64Buf(initTokenAmount).toString('hex')),
        tx.outputs.length,
        new Bytes(Common.getUInt32Buf(3).toString('hex')),
        CONTRACT_DUST,
        new Bytes(otherOutputArray.toString('hex'))
      )
      const v = res.verify({ tx, inputIndex: 5, inputSatoshis: CONTRACT_DUST })
      if (!v.success) throw new Error(`T6d input 5 (UnlockContractCheck) verify failed: ${v.error}`)
      tx.inputs[5].setScript(res.unlockingScript)
    }

    // Input 4: Admin P2PKH signature
    Common.signP2PKH(tx, privKey, 4)

    const rawHex = tx.uncheckedSerialize()
    const actualBytes = rawHex.length / 2
    const minNeededFee = Math.ceil(actualBytes * feeRateSatPerByte) + 300
    if (t6dFee < minNeededFee && pass === 0) {
      t6dFee = minNeededFee + 1000
      continue
    }
    if (t6dFee < actualBytes) {
      throw new Error(`T6d fee ${t6dFee} < byte size ${actualBytes}`)
    }
    t6dTx = tx
    t6dRawHex = rawHex
    break
  }

  chain.seedRawTx(t6dTx.id, t6dRawHex)
  log(`All 6 contract inputs of T6d (${t6dTx.id.slice(0, 12)}) verified OFFLINE! size=${t6dRawHex.length / 2}B fee=${t6dFee}`)

  const rawTxs = [
    { label: 'T4a (split funding)', txid: t4aPrep.tx.id, rawHex: t4aPrep.rawHex },
    { label: 'T7 (LP genesis)', txid: t7Prep.tx.id, rawHex: t7Prep.rawHex },
    { label: 'T5 (SwapMain genesis)', txid: t5Prep.tx.id, rawHex: t5Prep.rawHex },
    { label: 'T6b (fund checks)', txid: t6bPrep.tx.id, rawHex: t6bPrep.rawHex },
    { label: 'T6c (deposit FT)', txid: t6cDep.tx.id, rawHex: t6cDep.rawHex },
    { label: 'T6a (fund AddLiq op)', txid: t6aPrep.tx.id, rawHex: t6aPrep.rawHex },
    { label: 'T6d (first AddLiquidity)', txid: t6dTx.id, rawHex: t6dRawHex },
  ]

  if (!dryRun) {
    for (const item of rawTxs) {
      log(`Broadcasting ${item.label}: ${item.txid} ...`)
      await chain.broadcast(item.rawHex)
      log(`Broadcasted ${item.label}: ${item.txid}`)
    }
  } else {
    log('DRY-RUN complete: all 7 transactions built & cryptographically verified offline; nothing broadcasted.')
  }

  const slugSymbol =
    tokenSymbol.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || tokenID.slice(0, 8)
  const poolId = (params.poolId || `space-${slugSymbol}`).trim()
  const pairName = (params.pairName || `SPACE / ${tokenSymbol}`).trim()

  const entry: PoolRegistryEntry = {
    poolId,
    pairName,
    verified: true,
    token1: {
      symbol: 'SPACE',
      name: 'MicroVisionChain',
      decimals: 8,
    },
    token2: {
      symbol: tokenSymbol,
      name: tokenName,
      decimals: tokenDecimals,
      tokenID,
      codeHash: realTokenScriptCodeHash,
    },
    lpToken: {
      symbol: lpSymbol,
      decimals: tokenDecimals,
      tokenID: lpTokenID,
      codeHash: realTokenScriptCodeHash,
    },
    contract: {
      swapContractID,
      swapSensibleID,
      tokenID,
      tokenScriptCodeHash: realTokenScriptCodeHash,
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
    t4aPrep.fee + t7Prep.fee + t5Prep.fee + t6bPrep.fee + t6cDep.fee + t6aPrep.fee + t6dFee

  return {
    dryRun,
    adminAddress,
    entry,
    txids: {
      splitTxid: t4aPrep.tx.id,
      lpGenesisTxid: t7Prep.tx.id,
      swapGenesisTxid: t5Prep.tx.id,
      prepChecksTxid: t6bPrep.tx.id,
      depositTokenTxid: t6cDep.tx.id,
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

export {
  deployTokenSwapPoolOnChain,
  DeployTokenSwapPoolInput,
  DeployTokenSwapPoolResult,
} from './token-swap-deploy'
