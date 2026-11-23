import * as fs from 'fs'
import * as path from 'path'

export interface TokenMeta {
  symbol: string
  name: string
  decimals: number
  tokenID?: string
  codeHash?: string
}

export interface PoolRegistryEntry {
  poolId: string
  pairName: string
  verified: boolean
  token1: TokenMeta
  token2: TokenMeta & { tokenID: string; codeHash: string }
  lpToken: {
    symbol: string
    decimals: number
    tokenID: string
    codeHash: string
  }
  contract: {
    swapContractID: string
    swapSensibleID: string
    tokenID: string
    tokenScriptCodeHash: string
    lpTokenID: string
    fetchTokenHash: string
    swapCodeHash: string
    contractHashRoot: string
    contractHashArrayHex: string
  }
  bootstrapHead: {
    poolTxid: string
    lpGenesisTxid: string
    lpGenesisVout: number
  }
}

export interface ServiceConfig {
  host: string
  port: number
  metaletApiBase: string
  higunUrl?: string
  net: 'livenet' | 'testnet'
  poolsFile: string
  dataDir: string
  sessionTtlMs: number
  poolLockTimeoutMs: number
  whitelistSwapCodeHashes: string[]
  pools: PoolRegistryEntry[]
}

export function loadConfig(): ServiceConfig {
  const rootDir = path.resolve(__dirname, '..')
  const poolsFile = process.env.SWAP_POOLS_FILE || path.join(rootDir, 'pools.json')
  const dataDir = process.env.SWAP_DATA_DIR || path.join(rootDir, 'data')
  if (!fs.existsSync(dataDir)) {
    fs.mkdirSync(dataDir, { recursive: true })
  }
  const raw = JSON.parse(fs.readFileSync(poolsFile, 'utf8'))
  const whitelist: string[] = Array.isArray(raw.whitelistSwapCodeHashes) ? raw.whitelistSwapCodeHashes : []
  const pools: PoolRegistryEntry[] = (Array.isArray(raw.pools) ? raw.pools : []).filter((p: PoolRegistryEntry) => {
    return p.verified && whitelist.includes(p.contract.swapCodeHash)
  })

  return {
    host: process.env.SWAP_HOST || '127.0.0.1',
    port: Number(process.env.SWAP_PORT || '18080'),
    metaletApiBase: process.env.METALET_API_BASE || 'https://www.metalet.space/wallet-api/v4/mvc',
    higunUrl: process.env.HIGUN_URL || undefined,
    net: (process.env.MVC_NET as 'livenet' | 'testnet') || 'livenet',
    poolsFile,
    dataDir,
    sessionTtlMs: Number(process.env.SWAP_SESSION_TTL_MS || String(5 * 60 * 1000)),
    poolLockTimeoutMs: Number(process.env.SWAP_LOCK_TIMEOUT_MS || String(45 * 1000)),
    whitelistSwapCodeHashes: whitelist,
    pools,
  }
}
