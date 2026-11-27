import Database from 'better-sqlite3'

export interface CoinRow {
  tickId: string
  tick: string
  mode: 'legacy' | 'new' | 'unknown'
  payTo: string
  lpVault: string
  liquidityPerMint: number
  amtPerMint: string
  decimals: string
  updatedAt: number
}

export class KeeperDb {
  db: Database.Database

  constructor(path: string) {
    this.db = new Database(path)
    this.db.pragma('journal_mode = WAL')
    this.migrate()
  }

  private migrate() {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS coins (
        tick_id TEXT PRIMARY KEY,
        tick TEXT NOT NULL,
        mode TEXT NOT NULL,
        pay_to TEXT NOT NULL,
        lp_vault TEXT NOT NULL,
        liquidity_per_mint INTEGER NOT NULL,
        amt_per_mint TEXT NOT NULL,
        decimals TEXT NOT NULL,
        updated_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS legacy_seen_utxos (
        outpoint TEXT PRIMARY KEY,       -- txid:vout
        tick_id TEXT NOT NULL,
        address TEXT NOT NULL,
        sats INTEGER NOT NULL,
        first_seen INTEGER NOT NULL,
        last_seen INTEGER NOT NULL,
        present INTEGER NOT NULL DEFAULT 1
      );
      CREATE TABLE IF NOT EXISTS sweeps (
        outpoint TEXT PRIMARY KEY,       -- input txid:vout, swept at most once (live)
        tick_id TEXT NOT NULL,
        sweep_txid TEXT NOT NULL,
        sats INTEGER NOT NULL,
        dry_run INTEGER NOT NULL DEFAULT 0,
        created_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS kv (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL
      );
      CREATE TABLE IF NOT EXISTS sweep_txs (
        txid TEXT PRIMARY KEY,
        tick_id TEXT NOT NULL,
        hex TEXT NOT NULL,
        total_in INTEGER NOT NULL,
        fee INTEGER NOT NULL,
        output_sats INTEGER NOT NULL,
        status TEXT NOT NULL,            -- built | broadcast | confirmed | dropped
        dry_run INTEGER NOT NULL,
        created_at INTEGER NOT NULL,
        broadcast_at INTEGER
      );
      CREATE TABLE IF NOT EXISTS alerts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        kind TEXT NOT NULL,
        tick_id TEXT,
        message TEXT NOT NULL,
        created_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS vault_utxos (
        outpoint TEXT PRIMARY KEY,
        tick_id TEXT NOT NULL,
        address TEXT NOT NULL,
        sats INTEGER NOT NULL,
        confirmed INTEGER NOT NULL,
        provenance TEXT NOT NULL,        -- sweep:<txid> | direct | unknown
        first_seen INTEGER NOT NULL,
        last_seen INTEGER NOT NULL,
        present INTEGER NOT NULL DEFAULT 1
      );
      CREATE TABLE IF NOT EXISTS batches (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        tick_id TEXT NOT NULL,
        state TEXT NOT NULL,             -- planned|order_created|deposit_signed|deposit_submitted|minted|bid_placed|failed|held
        input_outpoints TEXT NOT NULL,   -- json array of txid:vout
        total_in INTEGER NOT NULL,
        est_tx_fee INTEGER NOT NULL DEFAULT 0,
        btc_fee_sats INTEGER NOT NULL DEFAULT 0,
        amount INTEGER NOT NULL,         -- principal to bridge vault
        order_id TEXT,
        bridge_address TEXT,
        fee_address TEXT,
        deposit_txid TEXT,
        deposit_hex TEXT,
        mint_txid TEXT,
        minted_wbtc INTEGER,
        price_num TEXT,
        price_den TEXT,
        bid_outpoint TEXT,
        bid_txids TEXT,
        fail_reason TEXT,
        hold_reason TEXT,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS batch_inputs (
        outpoint TEXT PRIMARY KEY,       -- a vault utxo is in at most one batch ever
        batch_id INTEGER NOT NULL,
        sats INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS discrepancies (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        address TEXT NOT NULL,
        detail TEXT NOT NULL,
        created_at INTEGER NOT NULL
      );
    `)
  }

  upsertCoin(c: CoinRow) {
    this.db.prepare(
      `INSERT INTO coins (tick_id, tick, mode, pay_to, lp_vault, liquidity_per_mint, amt_per_mint, decimals, updated_at)
       VALUES (@tickId, @tick, @mode, @payTo, @lpVault, @liquidityPerMint, @amtPerMint, @decimals, @updatedAt)
       ON CONFLICT(tick_id) DO UPDATE SET
         tick=excluded.tick, mode=excluded.mode, pay_to=excluded.pay_to, lp_vault=excluded.lp_vault,
         liquidity_per_mint=excluded.liquidity_per_mint, amt_per_mint=excluded.amt_per_mint,
         decimals=excluded.decimals, updated_at=excluded.updated_at`
    ).run(c)
  }

  listCoins(): CoinRow[] {
    return this.db.prepare('SELECT tick_id AS tickId, tick, mode, pay_to AS payTo, lp_vault AS lpVault, liquidity_per_mint AS liquidityPerMint, amt_per_mint AS amtPerMint, decimals, updated_at AS updatedAt FROM coins').all() as CoinRow[]
  }

  /** true only when a LIVE (broadcast) sweep consumed this outpoint — dry-run
   *  records never block a real sweep. */
  isSwept(outpoint: string): boolean {
    return !!this.db.prepare('SELECT 1 FROM sweeps WHERE outpoint = ? AND dry_run = 0').get(outpoint)
  }

  /** true when any sweep row (dry-run or live) exists — used to keep dry-run
   *  from rebuilding and re-logging the identical tx every poll tick. */
  isSweepRecorded(outpoint: string): boolean {
    return !!this.db.prepare('SELECT 1 FROM sweeps WHERE outpoint = ?').get(outpoint)
  }

  getKv(key: string): string | null {
    const row = this.db.prepare('SELECT value FROM kv WHERE key = ?').get(key) as any
    return row ? row.value : null
  }

  setKv(key: string, value: string) {
    this.db.prepare('INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value').run(key, value)
  }

  recordSweepTx(t: { txid: string; tickId: string; hex: string; totalIn: number; fee: number; outputSats: number; status: string; dryRun: boolean; createdAt: number }) {
    this.db.prepare(
      `INSERT OR IGNORE INTO sweep_txs (txid, tick_id, hex, total_in, fee, output_sats, status, dry_run, created_at)
       VALUES (@txid, @tickId, @hex, @totalIn, @fee, @outputSats, 'built', @dryRun, @createdAt)`
    ).run({ ...t, dryRun: t.dryRun ? 1 : 0 })
  }

  setSweepTxStatus(txid: string, status: string, broadcastAt?: number) {
    this.db.prepare('UPDATE sweep_txs SET status = ?, broadcast_at = COALESCE(?, broadcast_at) WHERE txid = ?').run(status, broadcastAt ?? null, txid)
  }

  getSweepTx(txid: string): any {
    return this.db.prepare('SELECT * FROM sweep_txs WHERE txid = ?').get(txid)
  }

  recordSweepInputs(txid: string, tickId: string, inputs: Array<{ outpoint: string; sats: number }>, createdAt: number, dryRun: boolean) {
    // live sweeps REPLACE dry-run rows for the same outpoints (promoting them
    // to swept-once); dry-run rows never overwrite anything.
    const verb = dryRun ? 'INSERT OR IGNORE' : 'INSERT OR REPLACE'
    const ins = this.db.prepare(`${verb} INTO sweeps (outpoint, tick_id, sweep_txid, sats, dry_run, created_at) VALUES (?, ?, ?, ?, ?, ?)`)
    const tx = this.db.transaction(() => {
      for (const i of inputs) ins.run(i.outpoint, tickId, txid, i.sats, dryRun ? 1 : 0, createdAt)
    })
    tx()
  }

  sweepTxForOutpoint(outpoint: string): string | null {
    const row = this.db.prepare('SELECT sweep_txid AS t FROM sweeps WHERE outpoint = ?').get(outpoint) as any
    return row ? row.t : null
  }

  markSeenUtxo(outpoint: string, tickId: string, address: string, sats: number, now: number) {
    this.db.prepare(
      `INSERT INTO legacy_seen_utxos (outpoint, tick_id, address, sats, first_seen, last_seen, present)
       VALUES (?, ?, ?, ?, ?, ?, 1)
       ON CONFLICT(outpoint) DO UPDATE SET last_seen = excluded.last_seen, present = 1, sats = excluded.sats`
    ).run(outpoint, tickId, address, sats, now, now)
  }

  markSeenUtxosAbsent(tickId: string, presentOutpoints: string[], now: number) {
    const rows = this.db.prepare('SELECT outpoint FROM legacy_seen_utxos WHERE tick_id = ? AND present = 1').all(tickId) as any[]
    const upd = this.db.prepare('UPDATE legacy_seen_utxos SET present = 0, last_seen = ? WHERE outpoint = ?')
    const present = new Set(presentOutpoints)
    for (const r of rows) {
      if (!present.has(r.outpoint)) upd.run(now, r.outpoint)
    }
  }

  disappearedUnswept(tickId: string): Array<{ outpoint: string; sats: number }> {
    return this.db.prepare(
      `SELECT l.outpoint, l.sats FROM legacy_seen_utxos l
       LEFT JOIN sweeps s ON s.outpoint = l.outpoint AND s.dry_run = 0
       WHERE l.tick_id = ? AND l.present = 0 AND s.outpoint IS NULL`
    ).all(tickId) as any[]
  }

  recordAlert(kind: string, tickId: string | null, message: string, now: number) {
    this.db.prepare('INSERT INTO alerts (kind, tick_id, message, created_at) VALUES (?, ?, ?, ?)').run(kind, tickId, message, now)
  }

  upsertVaultUtxo(v: { outpoint: string; tickId: string; address: string; sats: number; confirmed: boolean; provenance: string; now: number }) {
    this.db.prepare(
      `INSERT INTO vault_utxos (outpoint, tick_id, address, sats, confirmed, provenance, first_seen, last_seen, present)
       VALUES (@outpoint, @tickId, @address, @sats, @confirmed, @provenance, @now, @now, 1)
       ON CONFLICT(outpoint) DO UPDATE SET last_seen = excluded.last_seen, present = 1,
         confirmed = excluded.confirmed, sats = excluded.sats`
    ).run({ ...v, confirmed: v.confirmed ? 1 : 0 })
  }

  markVaultUtxosAbsent(tickId: string, presentOutpoints: string[], now: number) {
    const rows = this.db.prepare('SELECT outpoint FROM vault_utxos WHERE tick_id = ? AND present = 1').all(tickId) as any[]
    const upd = this.db.prepare('UPDATE vault_utxos SET present = 0, last_seen = ? WHERE outpoint = ?')
    const present = new Set(presentOutpoints)
    for (const r of rows) {
      if (!present.has(r.outpoint)) upd.run(now, r.outpoint)
    }
  }

  vaultUtxos(tickId?: string): any[] {
    if (tickId) return this.db.prepare('SELECT * FROM vault_utxos WHERE tick_id = ? AND present = 1').all(tickId)
    return this.db.prepare('SELECT * FROM vault_utxos WHERE present = 1').all()
  }

  recordDiscrepancy(address: string, detail: string, now: number) {
    this.db.prepare('INSERT INTO discrepancies (address, detail, created_at) VALUES (?, ?, ?)').run(address, detail, now)
  }

  /** Atomically creates the batch row AND reserves its inputs — a vault utxo
   *  can never be reserved by two batches (batch_inputs.outpoint is PK). */
  createBatch(b: {
    tickId: string; inputs: Array<{ outpoint: string; sats: number }>; totalIn: number; estTxFee: number;
    btcFeeSats: number; amount: number; now: number
  }): number {
    const tx = this.db.transaction(() => {
      const res = this.db.prepare(
        `INSERT INTO batches (tick_id, state, input_outpoints, total_in, est_tx_fee, btc_fee_sats, amount, created_at, updated_at)
         VALUES (?, 'planned', ?, ?, ?, ?, ?, ?, ?)`
      ).run(b.tickId, JSON.stringify(b.inputs.map((i) => i.outpoint)), b.totalIn, b.estTxFee, b.btcFeeSats, b.amount, b.now, b.now)
      const id = Number(res.lastInsertRowid)
      const ins = this.db.prepare('INSERT INTO batch_inputs (outpoint, batch_id, sats) VALUES (?, ?, ?)')
      for (const i of b.inputs) ins.run(i.outpoint, id, i.sats)
      return id
    })
    return tx()
  }

  isBatched(outpoint: string): boolean {
    return !!this.db.prepare('SELECT 1 FROM batch_inputs WHERE outpoint = ?').get(outpoint)
  }

  updateBatch(id: number, fields: Record<string, any>, now: number) {
    const keys = Object.keys(fields)
    if (keys.length === 0) return
    const set = keys.map((k) => `${k} = ?`).join(', ')
    this.db.prepare(`UPDATE batches SET ${set}, updated_at = ? WHERE id = ?`).run(...keys.map((k) => fields[k]), now, id)
  }

  getBatch(id: number): any {
    return this.db.prepare('SELECT * FROM batches WHERE id = ?').get(id)
  }

  batchesByState(states?: string[]): any[] {
    if (states && states.length) {
      const ph = states.map(() => '?').join(',')
      return this.db.prepare(`SELECT * FROM batches WHERE state IN (${ph}) ORDER BY id`).all(...states)
    }
    return this.db.prepare('SELECT * FROM batches ORDER BY id').all()
  }

  counts(): Record<string, number> {
    const c = (q: string) => (this.db.prepare(q).get() as any).n
    return {
      coins: c('SELECT COUNT(*) AS n FROM coins'),
      sweeps: c('SELECT COUNT(*) AS n FROM sweeps'),
      sweepTxs: c('SELECT COUNT(*) AS n FROM sweep_txs'),
      vaultUtxos: c('SELECT COUNT(*) AS n FROM vault_utxos WHERE present = 1'),
      alerts: c('SELECT COUNT(*) AS n FROM alerts'),
      discrepancies: c('SELECT COUNT(*) AS n FROM discrepancies'),
    }
  }

  close() { this.db.close() }
}
