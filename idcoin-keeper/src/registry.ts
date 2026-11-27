import axios, { AxiosInstance } from 'axios'
import { BIP32Interface } from 'bip32'
import { deriveLpVault } from './keys'
import { CoinRow, KeeperDb } from './db'
import { LegacyCoinKey } from './keys'
import { Logger } from './log'

export interface CoinListItem {
  tickId: string
  tick: string
  decimals: string
  amtPerMint: string
  liquidityPerMint: number | string
  payCheck?: { payTo?: string; payAmount?: string } | null
}

export interface RegistryResult {
  total: number
  legacy: number
  fresh: number
  unknown: number
  coins: CoinRow[]
}

/**
 * Fetch the ID-Coins list and classify every coin:
 *  - 'legacy'  payTo equals the legacy-file address for that tickId
 *  - 'new'     payTo == LP vault(tick) derived from the LP mnemonic
 *  - 'unknown' anything else (skipped with a warning)
 */
export async function refreshCoinRegistry(opts: {
  coinsUrl: string
  accountNode: BIP32Interface
  legacyKeys: Map<string, LegacyCoinKey>
  db: KeeperDb
  log: Logger
  http?: AxiosInstance
  /** test hook: pre-fetched list items instead of HTTP */
  items?: CoinListItem[]
}): Promise<RegistryResult> {
  const http = opts.http || axios.create({ timeout: 15_000 })
  let items: CoinListItem[]
  if (opts.items) {
    items = opts.items
  } else {
    const resp = await http.get(opts.coinsUrl)
    const body = resp.data
    items = body?.data?.list || body?.list || []
  }

  const now = Date.now()
  let legacy = 0
  let fresh = 0
  let unknown = 0
  const rows: CoinRow[] = []

  for (const item of items) {
    const tick = String(item.tick || '')
    const tickId = String(item.tickId || '')
    if (!tick || !tickId) continue
    // payCheck is an object in the golden sample; some API versions return it
    // as a JSON-encoded string — accept both.
    let payCheck = item.payCheck as any
    if (typeof payCheck === 'string') {
      try { payCheck = JSON.parse(payCheck) } catch { payCheck = null }
    }
    const payTo = payCheck?.payTo || ''
    const lpVault = deriveLpVault(opts.accountNode, tick).address
    let mode: CoinRow['mode'] = 'unknown'
    const legacyEntry = opts.legacyKeys.get(tickId)
    if (legacyEntry && payTo === legacyEntry.address) {
      mode = 'legacy'
    } else if (payTo && payTo === lpVault) {
      mode = 'new'
    }
    if (mode === 'legacy') legacy++
    else if (mode === 'new') fresh++
    else {
      unknown++
      opts.log.warn('coin classification unknown — skipped', { tickId, tick })
    }
    const row: CoinRow = {
      tickId,
      tick,
      mode,
      payTo,
      lpVault,
      liquidityPerMint: Number(item.liquidityPerMint) || 0,
      amtPerMint: String(item.amtPerMint ?? ''),
      decimals: String(item.decimals ?? '8'),
      updatedAt: now,
    }
    rows.push(row)
    opts.db.upsertCoin(row)
  }

  opts.log.info('coin registry refreshed', { total: rows.length, legacy, fresh: fresh, unknown })
  return { total: rows.length, legacy, fresh, unknown, coins: rows }
}
