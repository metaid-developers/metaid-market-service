/**
 * Resolves sibling `mvcswap-core` so orderbook-service shares the exact compiled
 * OrderAskV2 / OrderBidV2 / OrderBidFiller / Token contracts and tslib helpers.
 */
import * as fs from 'fs'
import * as path from 'path'

function candidateDirs(): string[] {
  return [
    process.env.MVCSWAP_CORE_DIR,
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core'),
    path.resolve(__dirname, '..', '..', '..', '..', 'mvcswap-core'),
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core-mainnet'),
    path.resolve(__dirname, '..', '..', '..', '..', 'mvcswap-core-mainnet'),
  ].filter(Boolean) as string[]
}

function resolveCoreDir(): string {
  const candidates = candidateDirs()
  for (const dir of candidates) {
    if (
      fs.existsSync(path.join(dir, 'tslib', 'orderbookProto.ts')) &&
      fs.existsSync(path.join(dir, 'contracts', 'orderbook', 'orderAskV2.scrypt')) &&
      fs.existsSync(path.join(dir, 'node_modules', 'mvc-scrypt'))
    ) {
      return dir
    }
  }
  for (const dir of candidates) {
    if (
      fs.existsSync(path.join(dir, 'tslib', 'orderbookProto.ts')) &&
      fs.existsSync(path.join(dir, 'contracts', 'orderbook', 'orderAskV2.scrypt'))
    ) {
      return dir
    }
  }
  throw new Error(
    `Unable to locate mvcswap-core with contracts/orderbook/orderAskV2.scrypt. Checked: ${candidates.join(', ')}`
  )
}

export const MVCSWAP_CORE_DIR = resolveCoreDir()
export const MVCSWAP_OUT_DIR = path.join(MVCSWAP_CORE_DIR, 'out')

// Register mvcswap-core/node_modules so mvc-scrypt and deps resolve seamlessly.
const Module = require('module')
const extraNodeModules: string[] = []
for (const dir of [MVCSWAP_CORE_DIR, ...candidateDirs()]) {
  const nm = path.join(dir, 'node_modules')
  if (fs.existsSync(nm)) {
    if (!module.paths.includes(nm)) module.paths.push(nm)
    if (!Module.globalPaths.includes(nm)) Module.globalPaths.push(nm)
    extraNodeModules.push(nm)
  }
}
if (extraNodeModules.length > 0) {
  process.env.NODE_PATH = [process.env.NODE_PATH, ...extraNodeModules].filter(Boolean).join(path.delimiter)
  Module._initPaths()
}

export const scrypt = require('mvc-scrypt')
export const axios = require('axios')
export const Common = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'common'))
export const TokenProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenProto'))
export const ProtoHeader = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'protoheader'))
export const OrderProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'orderbookProto'))
export const SwapEngine = require(path.join(MVCSWAP_CORE_DIR, 'scripts', 'swap-engine'))

export const PROD_TRANSFER_CHECK_HASHES: string[] = SwapEngine.PROD_TRANSFER_CHECK_HASHES
export const PROD_UNLOCK_CHECK_HASHES: string[] = SwapEngine.PROD_UNLOCK_CHECK_HASHES

function descMatchesRelPath(desc: any, relPath: string): boolean {
  if (!desc || typeof desc.file !== 'string') return false
  return desc.file.replace(/\\/g, '/').endsWith(`/contracts/${relPath}.scrypt`)
}

function normalizeDesc(raw: any): any {
  if (raw && raw.version !== undefined) return raw
  if (raw && typeof raw.toDesc === 'function') return raw.toDesc()
  if (raw && raw.contract && raw.abi && raw.hex) {
    return new scrypt.CompileResult(raw).toDesc()
  }
  return raw
}

export function loadVerifiedContractDesc(relPath: string, cacheDir: string): any {
  const baseName = path.basename(relPath)
  const fileName = `${baseName}_debug_desc.json`
  if (!fs.existsSync(cacheDir)) {
    fs.mkdirSync(cacheDir, { recursive: true })
  }
  const cachedPath = path.join(cacheDir, fileName)
  if (fs.existsSync(cachedPath)) {
    try {
      const raw = JSON.parse(fs.readFileSync(cachedPath, 'utf8'))
      if (descMatchesRelPath(raw, relPath)) {
        return normalizeDesc(raw)
      }
    } catch (_) {}
  }

  const sharedPath = path.join(MVCSWAP_OUT_DIR, fileName)
  if (fs.existsSync(sharedPath)) {
    try {
      const raw = JSON.parse(fs.readFileSync(sharedPath, 'utf8'))
      if (descMatchesRelPath(raw, relPath)) {
        const desc = normalizeDesc(raw)
        fs.writeFileSync(cachedPath, JSON.stringify(desc, null, 2), 'utf8')
        return desc
      }
    } catch (_) {}
  }

  const srcPath = path.join(MVCSWAP_CORE_DIR, 'contracts', `${relPath}.scrypt`)
  const result = scrypt.compileContract(srcPath, { out: cacheDir, debug: false, hex: true, desc: true })
  if (result.errors && result.errors.length > 0) {
    throw new Error(`Compile ${relPath} failed: ${JSON.stringify(result.errors)}`)
  }
  const desc = normalizeDesc(result)
  fs.writeFileSync(cachedPath, JSON.stringify(desc, null, 2), 'utf8')
  return desc
}

export interface LoadedOrderbookContracts {
  OrderAskV2: any
  OrderBidV2: any
  OrderBidFiller: any
  Token: any
  TokenTransferCheck: any
  TokenUnlockContractCheck: any
  TokenUnlockContractCheck4to8: any
  TxInputProof: any
  TxOutputProof: any
  askUnlockCheckHashes: any[]
  askCodeHashHex: string
  bidCodeHashHex: string
}

let cachedOBContracts: LoadedOrderbookContracts | null = null

export function loadOrderbookContracts(cacheDir: string, unlockCheckHashesHex = PROD_UNLOCK_CHECK_HASHES.slice(0, 2)): LoadedOrderbookContracts {
  if (cachedOBContracts) return cachedOBContracts

  const OrderAskV2 = scrypt.buildContractClass(loadVerifiedContractDesc('orderbook/orderAskV2', cacheDir))
  const OrderBidV2 = scrypt.buildContractClass(loadVerifiedContractDesc('orderbook/orderBidV2', cacheDir))
  const OrderBidFiller = scrypt.buildContractClass(loadVerifiedContractDesc('orderbook/orderBidFiller', cacheDir))
  const Token = scrypt.buildContractClass(loadVerifiedContractDesc('token/token', cacheDir))
  const TokenTransferCheck = scrypt.buildContractClass(loadVerifiedContractDesc('token/tokenTransferCheck', cacheDir))
  const TokenUnlockContractCheck = scrypt.buildContractClass(loadVerifiedContractDesc('token/tokenUnlockContractCheck', cacheDir))
  const TokenUnlockContractCheck4to8 = scrypt.buildContractClass(loadVerifiedContractDesc('token/tokenUnlockContractCheck_4to8', cacheDir))

  const typeClasses = scrypt.buildTypeClasses(Token)
  const askUnlockCheckHashes = unlockCheckHashesHex.map((h) => new scrypt.Bytes(h))

  // Compute canonical contract-family codehashes (matching Higun hash160(script[:len-150]))
  const dummyData = OrderProto.buildOrderDataPart({
    salt: Buffer.alloc(8, 0),
    feeBps: 0,
    feeAddr: Buffer.alloc(20, 0),
    minFillAmount: BigInt(1),
    remainAmount: BigInt(1000),
    priceDen: BigInt(1),
    priceNum: BigInt(1),
    tokenCodeHash: Buffer.alloc(20, 0),
    tokenID: Buffer.alloc(20, 0),
    makerAddr: Buffer.alloc(20, 0),
    orderSide: OrderProto.SIDE_ASK,
  })
  const sampleAsk = new OrderAskV2(askUnlockCheckHashes)
  sampleAsk.setDataPart(dummyData.toString('hex'))
  const askCodeHashHex = OrderProto.getScriptCodeHash(sampleAsk.lockingScript.toBuffer()).toString('hex')

  const dummyBidData = OrderProto.buildOrderDataPart({
    salt: Buffer.alloc(8, 0),
    feeBps: 0,
    feeAddr: Buffer.alloc(20, 0),
    minFillAmount: BigInt(1),
    remainAmount: BigInt(1000),
    priceDen: BigInt(1),
    priceNum: BigInt(1),
    tokenCodeHash: Buffer.alloc(20, 0),
    tokenID: Buffer.alloc(20, 0),
    makerAddr: Buffer.alloc(20, 0),
    orderSide: OrderProto.SIDE_BID,
  })
  const sampleBid = new OrderBidV2()
  sampleBid.setDataPart(dummyBidData.toString('hex'))
  const bidCodeHashHex = OrderProto.getScriptCodeHash(sampleBid.lockingScript.toBuffer()).toString('hex')

  cachedOBContracts = {
    OrderAskV2,
    OrderBidV2,
    OrderBidFiller,
    Token,
    TokenTransferCheck,
    TokenUnlockContractCheck,
    TokenUnlockContractCheck4to8,
    TxInputProof: typeClasses.TxInputProof,
    TxOutputProof: typeClasses.TxOutputProof,
    askUnlockCheckHashes,
    askCodeHashHex,
    bidCodeHashHex,
  }
  return cachedOBContracts
}
