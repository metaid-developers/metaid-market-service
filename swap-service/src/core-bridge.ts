/**
 * Resolves mvcswap-core (tslib + compiled contracts + verified swap-engine)
 * so swap-service runs directly on top of the exact mainnet-verified contract engine.
 */
import * as path from 'path'
import * as fs from 'fs'

function resolveCoreDir(): string {
  const candidates = [
    process.env.MVCSWAP_CORE_DIR,
    path.resolve(__dirname, '..', '..', '..', 'mvcswap-core-swap'),
    path.resolve(__dirname, '..', '..', '..', '..', 'mvcswap-core-mainnet'),
  ].filter(Boolean) as string[]

  for (const dir of candidates) {
    if (fs.existsSync(path.join(dir, 'scripts', 'swap-engine.ts')) && fs.existsSync(path.join(dir, 'tslib', 'tokenSwapProto.ts'))) {
      return dir
    }
  }
  throw new Error(`Unable to locate mvcswap-core with scripts/swap-engine.ts. Checked: ${candidates.join(', ')}`)
}

export const MVCSWAP_CORE_DIR = resolveCoreDir()

// Allow node to resolve mvc-scrypt & dependencies from mvcswap-core/node_modules seamlessly
const coreNodeModules = path.join(MVCSWAP_CORE_DIR, 'node_modules')
if (!module.paths.includes(coreNodeModules)) {
  module.paths.push(coreNodeModules)
}

export const scrypt = require('mvc-scrypt')
export const Common = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'common'))
export const TokenProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenProto'))
export const TokenSwapProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'tokenSwapProto'))
export const UniqueProto = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'uniqueProto'))
export const ProtoHeader = require(path.join(MVCSWAP_CORE_DIR, 'tslib', 'protoheader'))
export const SwapEngine = require(path.join(MVCSWAP_CORE_DIR, 'scripts', 'swap-engine'))
