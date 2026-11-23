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
  // Prefer a directory that has swap-engine.ts + out/ + node_modules/mvc-scrypt
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
      fs.existsSync(path.join(dir, 'tslib', 'tokenSwapProto.ts')) &&
      fs.existsSync(path.join(dir, 'out'))
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

// Allow node to resolve mvc-scrypt & dependencies from mvcswap-core/node_modules seamlessly
for (const dir of [MVCSWAP_CORE_DIR, ...candidateDirs()]) {
  const nm = path.join(dir, 'node_modules')
  if (fs.existsSync(nm) && !module.paths.includes(nm)) {
    module.paths.push(nm)
  }
}

export const scrypt = require('mvc-scrypt')
export const axios = require('axios')
export const Common = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'common'))
export const TokenProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenProto'))
export const TokenSwapProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenSwapProto'))
export const UniqueProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'uniqueProto'))
export const ProtoHeader = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'protoheader'))
export const SwapEngine = require(path.join(MVCSWAP_CORE_DIR, 'scripts', 'swap-engine'))

export const SWAP_ARTIFACTS_DIR =
  process.env.SWAP_ARTIFACTS_DIR || path.resolve(__dirname, '..', 'data', 'compiled-swap')

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
 * `/contracts/<relPath>.scrypt` so `contracts/swap/*` is NEVER shadowed by `contracts/swapRange/*`.
 * Caches compiled JSON descriptors into SWAP_ARTIFACTS_DIR so subsequent startups take <200ms.
 */
export function loadVerifiedContractDesc(relPath: string): any {
  const baseName = path.basename(relPath)
  const fileName = `${baseName}_debug_desc.json`
  if (!fs.existsSync(SWAP_ARTIFACTS_DIR)) {
    fs.mkdirSync(SWAP_ARTIFACTS_DIR, { recursive: true })
  }
  const cachedPath = path.join(SWAP_ARTIFACTS_DIR, fileName)
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
  const compiled = scrypt.compileContract(srcPath, { out: SWAP_ARTIFACTS_DIR })
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
    TxOutputProof,
    TxInputProof,
    transferCheckCodeHash,
    unlockCheckCodeHash,
    swapMainCodeHash,
    transferCheckCodeHashArray,
    unlockCheckCodeHashArray,
  }
  return cachedFastContracts
}
