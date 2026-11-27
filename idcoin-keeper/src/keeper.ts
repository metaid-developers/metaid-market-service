import { KeeperConfig } from './config'
import { KeeperDb, CoinRow } from './db'
import { ChainClient } from './chain'
import { LpKeys, LegacyCoinKey, loadLpKeys, loadLegacyKeys, deriveLpVault } from './keys'
import { Logger } from './log'
import { buildSweepTx, eligibleUtxos } from './sweep'

export interface KeeperDeps {
  config: KeeperConfig
  db: KeeperDb
  chain: ChainClient
  log: Logger
  lpKeys?: LpKeys
  legacyKeys?: Map<string, LegacyCoinKey>
}

export interface TickSweepReport {
  tickId: string
  tick: string
  action: 'swept' | 'skipped' | 'disabled' | 'not-legacy' | 'none'
  detail?: string
  txid?: string
  broadcast?: boolean
}

/**
 * Core keeper: owns keys, the DB, and one iteration of the legacy-sweep and
 * vault-ledger pipelines. Bridge batching + floor-bid placement (P2) will plug
 * in behind `vaultLedgerTick()` outputs.
 */
export class Keeper {
  config: KeeperConfig
  db: KeeperDb
  chain: ChainClient
  log: Logger
  lpKeys: LpKeys
  legacyKeys: Map<string, LegacyCoinKey>
  lastPollAt = 0
  lastError = ''

  constructor(deps: KeeperDeps) {
    this.config = deps.config
    this.db = deps.db
    this.chain = deps.chain
    this.log = deps.log
    this.lpKeys = deps.lpKeys || loadLpKeys(deps.config.mnemonicFile)
    this.legacyKeys = deps.legacyKeys || loadLegacyKeys(deps.config.legacyKeysFile)
  }

  /** One sweep pass over all enabled legacy coins. Safe to call repeatedly —
   *  swept outpoints are persisted, so a UTXO is never swept twice. */
  async sweepTick(now = Date.now()): Promise<TickSweepReport[]> {
    this.lastPollAt = now
    if (this.config.killSwitch) {
      this.log.warn('kill switch engaged — sweep pass skipped')
      return []
    }
    const coins = this.db.listCoins()
    const reports: TickSweepReport[] = []
    for (const coin of coins) {
      const rep = await this.sweepCoin(coin, now)
      reports.push(rep)
    }
    return reports
  }

  async sweepCoin(coin: CoinRow, now = Date.now()): Promise<TickSweepReport> {
    const base = { tickId: coin.tickId, tick: coin.tick }
    if (coin.mode !== 'legacy') return { ...base, action: 'not-legacy' }
    if (!this.config.enableTicks.has(coin.tick.toUpperCase())) {
      return { ...base, action: 'disabled' }
    }
    const legacyKey = this.legacyKeys.get(coin.tickId)
    if (!legacyKey) {
      this.log.warn('legacy-mode coin missing legacy key entry', { tickId: coin.tickId, tick: coin.tick })
      return { ...base, action: 'skipped', detail: 'missing legacy key' }
    }

    let utxos
    try {
      utxos = await this.chain.getUtxos(coin.payTo)
    } catch (e: any) {
      this.lastError = `higun utxo fetch failed for ${coin.tick}: ${e.message}`
      this.log.error('utxo fetch failed', { tickId: coin.tickId, tick: coin.tick, error: e.message })
      return { ...base, action: 'skipped', detail: 'fetch failed' }
    }

    // track every seen outpoint for compromise detection
    const presentOutpoints: string[] = []
    for (const u of utxos) {
      const op = `${u.txid}:${u.vout}`
      presentOutpoints.push(op)
      this.db.markSeenUtxo(op, coin.tickId, coin.payTo, u.sats, now)
    }
    this.db.markSeenUtxosAbsent(coin.tickId, presentOutpoints, now)

    // compromise detection: a previously-seen, never-swept UTXO disappeared,
    // or a swept input was spent by a different tx
    await this.detectCompromise(coin)

    // dry-run: skip outpoints with ANY sweep record (avoids rebuilding/re-logging
    // the identical tx every tick) — but those records never block a live sweep.
    // live: skip only outpoints consumed by a previous LIVE sweep.
    const candidates = eligibleUtxos(utxos, this.config.minUtxoSats).filter((u) => {
      const op = `${u.txid}:${u.vout}`
      return this.config.dryRun ? !this.db.isSweepRecorded(op) : !this.db.isSwept(op)
    })
    if (candidates.length === 0) return { ...base, action: 'none' }

    // fee oracle: mempool.space fastestFee when available (persisted as the
    // last-known value); on failure fall back to max(floor, last-known, 10)
    // and raise an alert rather than skipping the sweep.
    let feeRate: number
    try {
      const fastest = await this.chain.getFastestFee()
      this.db.setKv('lastFastestFee', String(fastest))
      feeRate = Math.max(fastest, this.config.feeFloorSatPerVb)
    } catch (e: any) {
      const lastKnown = Number(this.db.getKv('lastFastestFee')) || 0
      feeRate = Math.max(this.config.feeFloorSatPerVb, lastKnown, 10)
      const msg = `fee oracle unavailable (${e.message}); falling back to ${feeRate} sat/vB (floor ${this.config.feeFloorSatPerVb}, last-known ${lastKnown})`
      this.log.alert(msg, { tickId: coin.tickId, tick: coin.tick })
      this.db.recordAlert('fee-oracle-fallback', coin.tickId, msg, now)
    }

    const outcome = buildSweepTx({
      tickId: coin.tickId,
      tick: coin.tick,
      legacyKey,
      accountNode: this.lpKeys.accountNode,
      utxos: candidates,
      feeRateSatPerVb: feeRate,
      maxFeeRatio: this.config.maxFeeRatio,
    })

    if ('skipped' in outcome && outcome.skipped) {
      if (outcome.reason.includes('fee ratio')) {
        const msg = `sweep fee cap hit: ${outcome.reason}`
        this.log.alert(msg, { tickId: coin.tickId, tick: coin.tick })
        this.db.recordAlert('fee-cap', coin.tickId, msg, now)
      } else {
        this.log.info('sweep skipped', { tickId: coin.tickId, tick: coin.tick, reason: outcome.reason })
      }
      return { ...base, action: 'skipped', detail: outcome.reason }
    }
    if (!('built' in outcome)) return { ...base, action: 'skipped', detail: 'unexpected' }

    // persist atomically BEFORE broadcast: inputs are swept-once even in dry-run
    this.db.recordSweepTx({
      txid: outcome.txid,
      tickId: coin.tickId,
      hex: outcome.hex,
      totalIn: outcome.totalIn,
      fee: outcome.fee,
      outputSats: outcome.outputSats,
      status: 'built',
      dryRun: this.config.dryRun,
      createdAt: now,
    })
    this.db.recordSweepInputs(
      outcome.txid,
      coin.tickId,
      candidates.map((u) => ({ outpoint: `${u.txid}:${u.vout}`, sats: u.sats })),
      now,
      this.config.dryRun
    )

    if (this.config.dryRun) {
      this.log.info('DRY-RUN sweep built (not broadcast)', {
        tickId: coin.tickId,
        tick: coin.tick,
        txid: outcome.txid,
        inputs: candidates.length,
        totalIn: outcome.totalIn,
        fee: outcome.fee,
        outputSats: outcome.outputSats,
      })
      return { ...base, action: 'swept', txid: outcome.txid, broadcast: false }
    }

    try {
      const btxid = await this.chain.broadcastTx(outcome.hex)
      this.db.setSweepTxStatus(outcome.txid, 'broadcast', now)
      this.log.info('sweep broadcast', { tickId: coin.tickId, tick: coin.tick, txid: outcome.txid, broadcastTxid: btxid })
      return { ...base, action: 'swept', txid: outcome.txid, broadcast: true }
    } catch (e: any) {
      this.db.setSweepTxStatus(outcome.txid, 'built')
      this.log.error('sweep broadcast failed', { tickId: coin.tickId, tick: coin.tick, txid: outcome.txid, error: e.message })
      return { ...base, action: 'skipped', detail: 'broadcast failed', txid: outcome.txid }
    }
  }

  /**
   * Compromise detection:
   *  (a) a seen UTXO vanished without one of our sweeps covering it
   *  (b) an outpoint our sweep claims was spent by a DIFFERENT txid
   */
  async detectCompromise(coin: CoinRow) {
    const now = Date.now()
    for (const row of this.db.disappearedUnswept(coin.tickId)) {
      const msg = `utxo ${row.outpoint} (${row.sats} sats) disappeared from ${coin.payTo} without a keeper sweep — possible key compromise`
      this.log.alert(msg, { tickId: coin.tickId, tick: coin.tick })
      this.db.recordAlert('compromise-vanished', coin.tickId, msg, now)
    }
    // verify outspends of swept inputs still match our sweep txids
    const sweptRows = this.db.db
      .prepare('SELECT outpoint, sweep_txid FROM sweeps WHERE tick_id = ? AND dry_run = 0')
      .all(coin.tickId) as any[]
    for (const row of sweptRows) {
      const [txid, vout] = row.outpoint.split(':')
      try {
        const spender = await this.chain.getOutspend(txid, Number(vout))
        if (spender && spender !== row.sweep_txid) {
          const msg = `swept input ${row.outpoint} was spent by foreign tx ${spender} (our sweep ${row.sweep_txid}) — possible key compromise`
          this.log.alert(msg, { tickId: coin.tickId, tick: coin.tick })
          this.db.recordAlert('compromise-double-spend', coin.tickId, msg, now)
        }
      } catch {
        // best-effort; mempool.space may not know old txs
      }
    }
  }

  /** Vault ledger pass: track UTXOs at every LP vault with provenance. */
  async vaultLedgerTick(now = Date.now()): Promise<void> {
    if (this.config.killSwitch) return
    for (const coin of this.db.listCoins()) {
      if (coin.mode === 'unknown') continue
      try {
        const utxos = await this.chain.getUtxos(coin.lpVault)
        const present: string[] = []
        for (const u of utxos) {
          const op = `${u.txid}:${u.vout}`
          present.push(op)
          const sweepTx = this.db.getSweepTx(u.txid)
          const provenance = sweepTx ? `sweep:${u.txid}` : 'direct'
          this.db.upsertVaultUtxo({
            outpoint: op,
            tickId: coin.tickId,
            address: coin.lpVault,
            sats: u.sats,
            confirmed: u.confirmed,
            provenance,
            now,
          })
        }
        this.db.markVaultUtxosAbsent(coin.tickId, present, now)
      } catch (e: any) {
        this.log.error('vault ledger fetch failed', { tickId: coin.tickId, error: e.message })
      }
    }
  }

  status(): Record<string, any> {
    return {
      dryRun: this.config.dryRun,
      killSwitch: this.config.killSwitch,
      enableTicks: [...this.config.enableTicks],
      lastPollAt: this.lastPollAt,
      lastError: this.lastError,
      mvcFundingAddress: this.lpKeys.mvcFundingAddress,
      counts: this.db.counts(),
      pipeline: (this as any).pipelineStatus ? (this as any).pipelineStatus() : undefined,
      coins: this.db.listCoins().map((c) => ({
        tickId: c.tickId,
        tick: c.tick,
        mode: c.mode,
        payTo: c.payTo,
        lpVault: c.lpVault,
      })),
    }
  }
}
