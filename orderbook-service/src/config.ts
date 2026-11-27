import * as path from 'path'

export interface OrderbookSidecarConfig {
  host: string
  port: number
  net: 'livenet' | 'testnet'
  higunUrl: string
  metaletApiBase: string
  feeRateSatPerByte: number
  draftTtlMs: number
  artifactsDir: string
}

export function loadConfig(): OrderbookSidecarConfig {
  const netRaw = (process.env.ORDERBOOK_NET || process.env.SWAP_NET || 'livenet').toLowerCase()
  return {
    host: process.env.ORDERBOOK_SERVICE_HOST || '127.0.0.1',
    port: Number(process.env.ORDERBOOK_SERVICE_PORT || 18081),
    net: netRaw === 'testnet' ? 'testnet' : 'livenet',
    higunUrl: (process.env.HIGUN_URL || process.env.ORDERBOOK_HIGUN_URL || '').replace(/\/+$/, ''),
    metaletApiBase: (process.env.METALET_API_BASE || 'https://www.metalet.space/wallet-api').replace(/\/+$/, ''),
    feeRateSatPerByte: Number(process.env.ORDERBOOK_FEE_RATE || 1.25),
    draftTtlMs: Number(process.env.ORDERBOOK_DRAFT_TTL_MS || 120000),
    artifactsDir:
      process.env.ORDERBOOK_ARTIFACTS_DIR ||
      path.resolve(__dirname, '..', 'data', 'compiled-orderbook'),
  }
}
