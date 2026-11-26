/**
 * Resolves mvcswap-core (tslib + compiled contracts + verified swap-engine)
 * so swap-service runs directly on top of the exact mainnet-verified contract engine.
 */
import * as path from 'path'
import * as fs from 'fs'

function candidateDirs(): string[] {
  return [
    process.env.MVCSWAP_CORE_DIR,
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core'),
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core-mainnet'),
    path.resolve(__dirname, '..', '..', '..', 'worktrees', 'mvcswap-core-swap'),
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core-swap'),
    path.resolve(__dirname, '..', '..', '..', '..', 'mvcswap-core'),
    path.resolve(__dirname, '..', '..', '..', '..', 'mvcswap-core-mainnet'),
  ].filter(Boolean) as string[]
}

function resolveCoreDir(): string {
  const candidates = candidateDirs()
  // Prefer a directory that has both swap-engine.ts and token-swap-engine.ts (latest main)
  for (const dir of candidates) {
    if (
      fs.existsSync(path.join(dir, 'scripts', 'swap-engine.ts')) &&
      fs.existsSync(path.join(dir, 'scripts', 'token-swap-engine.ts')) &&
      fs.existsSync(path.join(dir, 'tslib', 'tokenSwapProto.ts'))
    ) {
      return dir
    }
  }
  for (const dir of candidates) {
    if (
      fs.existsSync(path.join(dir, 'scripts', 'swap-engine.ts')) &&
      fs.existsSync(path.join(dir, 'tslib', 'tokenSwapProto.ts')) &&
      fs.existsSync(path.join(dir, 'out')) &&
      fs.existsSync(path.join(dir, 'node_modules', 'mvc-scrypt'))
    ) {
      return dir
    }
  }
  for (const dir of candidates) {
    if (
      fs.existsSync(path.join(dir, 'scripts', 'swap-engine.ts')) &&
      fs.existsSync(path.join(dir, 'tslib', 'tokenSwapProto.ts'))
    ) {
      return dir
    }
  }
  throw new Error(`Unable to locate mvcswap-core with scripts/swap-engine.ts. Checked: ${candidates.join(', ')}`)
}

export const MVCSWAP_CORE_DIR = resolveCoreDir()

export const MVCSWAP_OUT_DIR = (() => {
  const direct = path.join(MVCSWAP_CORE_DIR, 'out')
  if (fs.existsSync(direct)) return direct
  for (const dir of candidateDirs()) {
    const out = path.join(dir, 'out')
    if (fs.existsSync(path.join(out, 'swapMain_debug_desc.json'))) return out
  }
  return direct
})()

// Allow node (including cross-repo modules like mvcswap-core/tslib/*.ts) to resolve mvc-scrypt & deps seamlessly
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
export const TokenSwapProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenSwapProto'))
export const UniqueProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'uniqueProto'))
export const ProtoHeader = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'protoheader'))
export const SwapEngine = require(path.join(MVCSWAP_CORE_DIR, 'scripts', 'swap-engine'))
export const TokenSwapEngine = require(path.join(MVCSWAP_CORE_DIR, 'scripts', 'token-swap-engine'))

export const SWAP_ARTIFACTS_DIR =
  process.env.SWAP_ARTIFACTS_DIR || path.resolve(__dirname, '..', 'data', 'compiled-swap')
export const TOKEN_SWAP_ARTIFACTS_DIR =
  process.env.TOKEN_SWAP_ARTIFACTS_DIR || path.resolve(__dirname, '..', 'data', 'compiled-tokenswap')

function descMatchesRelPath(desc: any, relPath: string): boolean {
  if (!desc || typeof desc.file !== 'string') return false
  const normalizedFile = desc.file.replace(/\\/g, '/')
  return normalizedFile.endsWith(`/contracts/${relPath}.scrypt`)
}

function normalizeContractDesc(raw: any): any {
  if (raw && raw.version !== undefined) return raw
  if (raw && typeof raw.toDesc === 'function') return raw.toDesc()
  if (raw && raw.contract && raw.abi && raw.hex) {
    const cr = new scrypt.CompileResult(raw)
    return cr.toDesc()
  }
  return raw
}

/**
 * Loads a single sCrypt contract descriptor, strictly verifying `desc.file` ends with
 * `/contracts/<relPath>.scrypt` so `contracts/swap/*`, `contracts/tokenSwap/*`, and
 * `contracts/swapRange/*` NEVER shadow each other.
 * Caches compiled JSON descriptors so subsequent startups take <200ms.
 */
export function loadVerifiedContractDesc(relPath: string, cacheDir = SWAP_ARTIFACTS_DIR): any {
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
        const desc = normalizeContractDesc(raw)
        if (raw.version === undefined && desc && desc.version !== undefined) {
          fs.writeFileSync(cachedPath, JSON.stringify(desc, null, 2), 'utf8')
        }
        return desc
      }
    } catch (_) {}
  }

  const sharedPath = path.join(MVCSWAP_OUT_DIR, fileName)
  if (fs.existsSync(sharedPath)) {
    try {
      const desc = normalizeContractDesc(JSON.parse(fs.readFileSync(sharedPath, 'utf8')))
      if (descMatchesRelPath(desc, relPath)) {
        fs.writeFileSync(cachedPath, JSON.stringify(desc, null, 2), 'utf8')
        return desc
      }
    } catch (_) {}
  }

  const srcPath = path.join(MVCSWAP_CORE_DIR, 'contracts', `${relPath}.scrypt`)
  if (!fs.existsSync(srcPath)) {
    throw new Error(`Contract source not found: ${srcPath}`)
  }
  const compiled = scrypt.compileContract(srcPath, { out: cacheDir })
  if (compiled.errors && compiled.errors.length > 0) {
    throw new Error(`Failed to compile ${srcPath}: ${JSON.stringify(compiled.errors)}`)
  }
  const desc = normalizeContractDesc(compiled)
  fs.writeFileSync(cachedPath, JSON.stringify(desc, null, 2), 'utf8')
  return desc
}

let cachedFastContracts: any = null

/**
 * Loads all MVCSwap AMM + MetaContract FT contract classes from verified isolated cache
 * (<200ms once warmed; strictly isolated from contracts/swapRange/*).
 */
export function loadSwapContractsFast(): any {
  if (cachedFastContracts) return cachedFastContracts
  const loadClass = (relPath: string) => scrypt.buildContractClass(loadVerifiedContractDesc(relPath))

  const SwapMain = loadClass('swap/swapMain')
  const SwapFetchToken = loadClass('swap/swapFetchToken')
  const SwapLpGenesis = loadClass('swap/swapLpGenesis')
  const SwapMvcToToken = loadClass('swap/swapMvcToToken')
  const SwapTokenToMvc = loadClass('swap/swapTokenToMvc')
  const SwapAddLiquidity = loadClass('swap/swapAddLiquidity')
  const SwapRemoveLiquidity = loadClass('swap/swapRemoveLiquidity')
  const Token = loadClass('token/token')
  const TokenTransferCheck = loadClass('token/tokenTransferCheck')
  const TokenUnlockContractCheck = loadClass('token/tokenUnlockContractCheck')

  const { TxOutputProof, TxInputProof } = scrypt.buildTypeClasses(loadVerifiedContractDesc('token/token'))

  const tcBare = new TokenTransferCheck().lockingScript.toBuffer()
  const transferCheckCodeHash = Common.hash160(Buffer.concat([tcBare, Buffer.from([0x6a])]))
  const ucBare = new TokenUnlockContractCheck().lockingScript.toBuffer()
  const unlockCheckCodeHash = Common.hash160(Buffer.concat([ucBare, Buffer.from([0x6a])]))
  const smBare = new SwapMain().lockingScript.toBuffer()
  const swapMainCodeHash = Common.hash160(Buffer.concat([smBare, Buffer.from([0x6a])]))

  const transferCheckCodeHashArray = new Array(5).fill(new scrypt.Bytes(transferCheckCodeHash.toString('hex')))
  const unlockCheckCodeHashArray = new Array(5).fill(new scrypt.Bytes(unlockCheckCodeHash.toString('hex')))

  const TokenUnlockContractCheck4to8 = scrypt.buildContractClass(
    loadVerifiedContractDesc('token/tokenUnlockContractCheck_4to8', SWAP_ARTIFACTS_DIR)
  )
  const uc48Bare = new TokenUnlockContractCheck4to8().lockingScript.toBuffer()
  const unlockCheck4to8CodeHash = Common.hash160(Buffer.concat([uc48Bare, Buffer.from([0x6a])]))

  cachedFastContracts = {
    SwapMain,
    SwapFetchToken,
    SwapLpGenesis,
    SwapMvcToToken,
    SwapTokenToMvc,
    SwapAddLiquidity,
    SwapRemoveLiquidity,
    Token,
    TokenTransferCheck,
    TokenUnlockContractCheck,
    TokenUnlockContractCheck4to8,
    TxOutputProof,
    TxInputProof,
    transferCheckCodeHash,
    unlockCheckCodeHash,
    unlockCheck4to8CodeHash,
    swapMainCodeHash,
    transferCheckCodeHashArray,
    unlockCheckCodeHashArray,
  }
  return cachedFastContracts
}

let cachedFastTokenSwapContracts: any = null

/**
 * Loads all MVCSwap native Token/Token (`contracts/tokenSwap/*`) + MetaContract FT contract classes
 * from isolated TOKEN_SWAP_ARTIFACTS_DIR cache (<200ms once warmed).
 */
export function loadTokenSwapContractsFast(): any {
  if (cachedFastTokenSwapContracts) return cachedFastTokenSwapContracts
  const base = loadSwapContractsFast()
  const loadTokSwapClass = (relPath: string) =>
    scrypt.buildContractClass(loadVerifiedContractDesc(relPath, TOKEN_SWAP_ARTIFACTS_DIR))

  const SwapMain = loadTokSwapClass('tokenSwap/tokenSwapMain')
  const SwapFetchToken = loadTokSwapClass('tokenSwap/tokenSwapFetchToken')
  const SwapLpGenesis = loadTokSwapClass('tokenSwap/tokenSwapLpGenesis')
  const SwapTokenToToken = loadTokSwapClass('tokenSwap/tokenSwapTokenToToken')
  const SwapAddLiquidity = loadTokSwapClass('tokenSwap/tokenSwapAddLiquidity')
  const SwapRemoveLiquidity = loadTokSwapClass('tokenSwap/tokenSwapRemoveLiquidity')
  const SwapUpdateContract = loadTokSwapClass('tokenSwap/tokenSwapUpdateContract')
  const SwapAdmin = loadTokSwapClass('tokenSwap/tokenSwapAdmin')

  const smBare = new SwapMain().lockingScript.toBuffer()
  const swapMainCodeHash = Common.hash160(Buffer.concat([smBare, Buffer.from([0x6a])]))

  cachedFastTokenSwapContracts = {
    SwapMain,
    SwapFetchToken,
    SwapLpGenesis,
    SwapTokenToToken,
    SwapAddLiquidity,
    SwapRemoveLiquidity,
    SwapUpdateContract,
    SwapAdmin,
    Token: base.Token,
    TokenTransferCheck: base.TokenTransferCheck,
    TokenUnlockContractCheck: base.TokenUnlockContractCheck,
    TokenUnlockContractCheck4to8: base.TokenUnlockContractCheck4to8,
    TxOutputProof: base.TxOutputProof,
    TxInputProof: base.TxInputProof,
    transferCheckCodeHash: base.transferCheckCodeHash,
    unlockCheckCodeHash: base.unlockCheckCodeHash,
    unlockCheck4to8CodeHash: base.unlockCheck4to8CodeHash,
    swapMainCodeHash,
    transferCheckCodeHashArray: base.transferCheckCodeHashArray,
    unlockCheckCodeHashArray: base.unlockCheckCodeHashArray,
  }
  return cachedFastTokenSwapContracts
}
