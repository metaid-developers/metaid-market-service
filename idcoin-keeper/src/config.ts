import * as fs from 'fs'
import * as yaml from 'js-yaml'

export interface KeeperConfig {
  mnemonicFile: string
  legacyKeysFile: string
  dbPath: string
  pollMs: number
  higunBtcUrl: string
  higunMvcUrl: string
  mempoolUrl: string
  btcRpcUrl: string
  btcRpcCredsFile: string
  coinsUrl: string
  dryRun: boolean
  killSwitch: boolean
  enableTicks: Set<string>   // UPPER tick; empty = nothing enabled
  feeFloorSatPerVb: number
  maxFeeRatio: number        // fee / totalInput cap for sweep txs
  minUtxoSats: number        // eligibility floor (1000)
  opsPort: number
  // ---- P2: batching / bridge / floor bids ----
  bridgeUrl: string
  bridgeVaultAddress: string       // pinned BTC bridge vault
  bridgeFeeAllowlist: Set<string>  // allowed btcFeeAddress values
  batchHaircutBps: number          // cost budget for batching (150 = 1.5%)
  floorHaircutBps: number          // floor-price haircut (150 -> x10000/9850)
  bridgeMinSats: number
  bridgeMaxSats: number
  fuelThresholdSats: number        // min SPACE balance at MVC LP funding for bidding
  depthUrl: string                 // orderbook depth endpoint base
}

function bool(v: any, dflt: boolean): boolean {
  if (v === undefined || v === null || v === '') return dflt
  const s = String(v).toLowerCase()
  return s === '1' || s === 'true' || s === 'yes'
}

function num(v: any, dflt: number): number {
  if (v === undefined || v === null || v === '') return dflt
  const n = Number(v)
  return Number.isFinite(n) ? n : dflt
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): KeeperConfig {
  let file: Record<string, any> = {}
  const cfgPath = env.IDCOIN_KEEPER_CONFIG
  if (cfgPath && fs.existsSync(cfgPath)) {
    file = (yaml.load(fs.readFileSync(cfgPath, 'utf8')) as Record<string, any>) || {}
  }
  const pick = (envKey: string, fileKey: string): any =>
    env[envKey] !== undefined && env[envKey] !== '' ? env[envKey] : file[fileKey]

  const ticksRaw = String(pick('IDCOIN_KEEPER_ENABLE_TICKS', 'enableTicks') || '')
  const enableTicks = new Set(
    ticksRaw.split(',').map((t) => t.trim().toUpperCase()).filter(Boolean)
  )

  return {
    mnemonicFile: String(pick('IDCOIN_LP_MNEMONIC_FILE', 'mnemonicFile') || ''),
    legacyKeysFile: String(pick('IDCOIN_LEGACY_KEYS_FILE', 'legacyKeysFile') || ''),
    dbPath: String(pick('IDCOIN_KEEPER_DB', 'dbPath') || './idcoin-keeper.db'),
    pollMs: num(pick('IDCOIN_KEEPER_POLL_MS', 'pollMs'), 10_000),
    higunBtcUrl: String(pick('IDCOIN_HIGUN_BTC_URL', 'higunBtcUrl') || 'http://172.31.165.190:8085'),
    higunMvcUrl: String(pick('IDCOIN_HIGUN_MVC_URL', 'higunMvcUrl') || 'http://47.239.239.128:8085'),
    mempoolUrl: String(pick('IDCOIN_MEMPOOL_URL', 'mempoolUrl') || 'https://mempool.space'),
    btcRpcUrl: String(pick('IDCOIN_BTC_RPC_URL', 'btcRpcUrl') || 'http://172.31.168.214:8332'),
    btcRpcCredsFile: String(pick('IDCOIN_BTC_RPC_CREDS_FILE', 'btcRpcCredsFile') || ''),
    coinsUrl: String(
      pick('IDCOIN_COINS_URL', 'coinsUrl') ||
        'https://api.metaid.market/api-market/api/v1/id-coins/coins-list?cursor=0&size=100'
    ),
    dryRun: bool(pick('IDCOIN_KEEPER_DRY_RUN', 'dryRun'), true),
    killSwitch: bool(pick('IDCOIN_KEEPER_KILL', 'killSwitch'), false),
    enableTicks,
    feeFloorSatPerVb: num(pick('IDCOIN_KEEPER_FEE_FLOOR', 'feeFloorSatPerVb'), 2),
    maxFeeRatio: num(pick('IDCOIN_KEEPER_MAX_FEE_RATIO', 'maxFeeRatio'), 0.3),
    minUtxoSats: num(pick('IDCOIN_KEEPER_MIN_UTXO_SATS', 'minUtxoSats'), 1000),
    opsPort: num(pick('IDCOIN_KEEPER_OPS_PORT', 'opsPort'), 18090),
    bridgeUrl: String(pick('IDCOIN_BRIDGE_URL', 'bridgeUrl') || 'https://api.octopus.space/api-bridge-v3'),
    bridgeVaultAddress: String(pick('IDCOIN_BRIDGE_VAULT', 'bridgeVaultAddress') || 'bc1qh90z428z4s0g9prcdf6rq00yg27uhxrtxlvvvv'),
    bridgeFeeAllowlist: new Set(
      String(pick('IDCOIN_BRIDGE_FEE_ALLOWLIST', 'bridgeFeeAllowlist') || 'bc1qyjg3wtuk7gyzd7tuug7sghhdj79whdf8wravre')
        .split(',').map((s) => s.trim()).filter(Boolean)
    ),
    batchHaircutBps: num(pick('IDCOIN_BATCH_HAIRCUT_BPS', 'batchHaircutBps'), 150),
    floorHaircutBps: num(pick('IDCOIN_FLOOR_HAIRCUT_BPS', 'floorHaircutBps'), 150),
    bridgeMinSats: num(pick('IDCOIN_BRIDGE_MIN_SATS', 'bridgeMinSats'), 10_000),
    bridgeMaxSats: num(pick('IDCOIN_BRIDGE_MAX_SATS', 'bridgeMaxSats'), 100_000_000),
    fuelThresholdSats: num(pick('IDCOIN_FUEL_THRESHOLD_SATS', 'fuelThresholdSats'), 20_000),
    depthUrl: String(pick('IDCOIN_DEPTH_URL', 'depthUrl') || 'https://api.metaid.market/api-market/api/v1/orderbook/depth'),
  }
}
