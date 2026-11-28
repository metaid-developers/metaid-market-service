import axios, { AxiosInstance, AxiosResponse } from 'axios'
import * as bitcoin from 'bitcoinjs-lib'
import { BitcoinRpcClient, RpcError } from './btcrpc'

export interface ChainUtxo {
  txid: string
  vout: number
  sats: number
  confirmed: boolean
  inscriptionBearing: boolean
}

export type SweptInputState =
  | { state: 'unspent' }
  | { state: 'ours'; confirmations: number }
  | { state: 'foreign' }
  | { state: 'unknown' }

export interface ChainClient {
  getUtxos(address: string): Promise<ChainUtxo[]>
  getFastestFee(): Promise<number>
  broadcastTx(hex: string): Promise<string>
  getSweptInputState(txid: string, vout: number, sweepTxid: string): Promise<SweptInputState>
}

function asBool(v: any): boolean {
  return v === true || v === 'true' || v === 1
}

/** Higun BTC compat proxy (:8085) — primary. /address/btc-utxo?address=&unconfirmed=1
 *  Note: upstream already excludes dust <= 1000 sats. Inscription flags are
 *  passed through when the proxy exposes them (inscriptions / inscriptionIds /
 *  isInscriptionCarrier); absent fields are treated as unknown-not-bearing. */
export class HigunBtcClient {
  private http: AxiosInstance
  constructor(public baseUrl: string, http?: AxiosInstance) {
    this.http = http || axios.create({ timeout: 15_000 })
  }

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    const resp = await this.http.get(`${this.baseUrl}/address/btc-utxo`, {
      params: { address, unconfirmed: '1' },
    })
    const body = resp.data
    const items: any[] = Array.isArray(body) ? body : body?.result || body?.data || []
    const out: ChainUtxo[] = []
    for (const it of items) {
      const txid = String(it.txId ?? it.tx_id ?? it.txid ?? '')
      const vout = Number(it.vout ?? it.index ?? 0)
      const sats = Number(it.satoshi ?? it.value ?? it.amount ?? 0)
      if (!txid || !Number.isFinite(vout) || !Number.isFinite(sats)) continue
      const inscr = it.inscriptions
      const inscriptionBearing =
        asBool(it.isInscriptionCarrier) ||
        (Array.isArray(inscr) && inscr.length > 0) ||
        (inscr != null && typeof inscr === 'object' && !Array.isArray(inscr) && Object.keys(inscr).length > 0) ||
        (Array.isArray(it.inscriptionIds) && it.inscriptionIds.length > 0)
      out.push({
        txid,
        vout,
        sats,
        confirmed: it.confirmed === undefined ? !asBool(it.is_mempool) : asBool(it.confirmed),
        inscriptionBearing,
      })
    }
    return out
  }
}

export interface MempoolGuardOptions {
  failureThreshold?: number
  openDurationMs?: number
  intervalMs?: number
  maxWaitMs?: number
  timeoutMs?: number
  broadcastTimeoutMs?: number
  now?: () => number
  sleep?: (ms: number) => Promise<void>
}

interface MempoolGuardState {
  failures: number
  open: boolean
  openedAt: number
  probing: boolean
  nextSlot: number
}

function unavailable(detail: string): Error {
  return new Error(`mempool.space unavailable: ${detail}`)
}

/** mempool.space — guarded fallback for fee rates, broadcast and read-only checks. */
export class MempoolClient {
  private http: AxiosInstance
  private state: MempoolGuardState = { failures: 0, open: false, openedAt: 0, probing: false, nextSlot: 0 }
  private failureThreshold: number
  private openDurationMs: number
  private intervalMs: number
  private maxWaitMs: number
  private timeoutMs: number
  private broadcastTimeoutMs: number
  private now: () => number
  private sleep: (ms: number) => Promise<void>

  constructor(public baseUrl: string, http?: AxiosInstance, opts: MempoolGuardOptions = {}) {
    this.http = http || axios.create({ timeout: opts.timeoutMs ?? 5_000 })
    this.failureThreshold = opts.failureThreshold ?? 3
    this.openDurationMs = opts.openDurationMs ?? 5 * 60_000
    this.intervalMs = opts.intervalMs ?? 1_000
    this.maxWaitMs = opts.maxWaitMs ?? 1_000
    this.timeoutMs = opts.timeoutMs ?? 5_000
    this.broadcastTimeoutMs = opts.broadcastTimeoutMs ?? 15_000
    this.now = opts.now || Date.now
    this.sleep = opts.sleep || ((ms) => new Promise((resolve) => setTimeout(resolve, ms)))
  }

  private reserve(): { probe: boolean; wait: number } {
    const now = this.now()
    const st = this.state
    if (st.open && ((now - st.openedAt) < this.openDurationMs || st.probing)) {
      throw unavailable('circuit breaker open')
    }
    const slot = Math.max(now, st.nextSlot)
    const wait = slot - now
    if (wait > this.maxWaitMs) {
      throw unavailable(`rate limiter wait ${wait}ms exceeds ${this.maxWaitMs}ms`)
    }
    st.nextSlot = slot + this.intervalMs
    let probe = false
    if (st.open) {
      st.probing = true
      probe = true
    }
    return { probe, wait }
  }

  private recordSuccess(probe: boolean) {
    const st = this.state
    st.failures = 0
    if (st.open || probe) {
      st.open = false
      st.probing = false
      console.log('mempool.space circuit breaker closed')
    }
  }

  private recordFailure(probe: boolean, immediate: boolean, err: any) {
    const st = this.state
    if (probe) {
      st.open = true
      st.openedAt = this.now()
      st.probing = false
      console.log(`mempool.space circuit breaker opened: ${err?.message || err}`)
      return
    }
    st.failures++
    if (immediate) st.failures = this.failureThreshold
    if (st.failures >= this.failureThreshold && !st.open) {
      st.open = true
      st.openedAt = this.now()
      console.log(`mempool.space circuit breaker opened: ${err?.message || err}`)
    }
  }

  private async request(method: 'GET' | 'POST', path: string, data?: any, timeoutMs?: number): Promise<AxiosResponse> {
    const { probe, wait } = this.reserve()
    if (wait > 0) await this.sleep(wait)
    let resp: AxiosResponse | undefined
    let transportErr: any = null
    try {
      resp = await this.http.request({
        method,
        url: `${this.baseUrl}${path}`,
        data,
        headers: method === 'POST' ? { 'Content-Type': 'text/plain' } : undefined,
        timeout: timeoutMs ?? this.timeoutMs,
        validateStatus: () => true,
      })
    } catch (e: any) {
      transportErr = e
    }
    if (transportErr) {
      this.recordFailure(probe, false, transportErr)
      throw unavailable(transportErr.message || String(transportErr))
    }
    const status = Number(resp!.status)
    if (status === 429 || status >= 500) {
      this.recordFailure(probe, status === 429, new Error(`status ${status}`))
      throw unavailable(`status ${status}`)
    }
    this.recordSuccess(probe)
    return resp!
  }

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    const resp = await this.request('GET', `/api/address/${address}/utxo`)
    const items: any[] = Array.isArray(resp.data) ? resp.data : []
    return items.map((it) => ({
      txid: String(it.txid),
      vout: Number(it.vout),
      sats: Number(it.value),
      confirmed: asBool(it.status?.confirmed),
      inscriptionBearing: false, // mempool.space does not mark inscription carriers
    }))
  }

  async getFastestFee(): Promise<number> {
    const resp = await this.request('GET', '/api/v1/fees/recommended')
    const fee = Number(resp.data?.fastestFee)
    return Number.isFinite(fee) && fee > 0 ? fee : 1
  }

  async broadcastTx(hex: string): Promise<string> {
    const resp = await this.request('POST', '/api/tx', hex, this.broadcastTimeoutMs)
    if (resp.status < 200 || resp.status >= 300) {
      throw new Error(`mempool.space broadcast rejected with status ${resp.status}: ${String(resp.data).slice(0, 200)}`)
    }
    return String(resp.data)
  }

  async getOutspend(txid: string, vout: number): Promise<string> {
    const resp = await this.request('GET', `/api/tx/${txid}/outspends`)
    const arr: any[] = Array.isArray(resp.data) ? resp.data : []
    const rec = arr[vout]
    if (!rec || !rec.spent) return ''
    return String(rec.txid || '')
  }
}

/** Composite: Higun primary, Bitcoin Core verification/fallbacks, mempool.space guarded fallback. */
export class CompositeChainClient implements ChainClient {
  constructor(
    public higun: HigunBtcClient,
    public rpc: BitcoinRpcClient | null,
    public mempool: MempoolClient,
    private onDiscrepancy?: (address: string, detail: string) => void
  ) {}

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    const primary = await this.higun.getUtxos(address)
    if (!this.rpc) return primary
    const kept: ChainUtxo[] = []
    const dropped: string[] = []
    try {
      for (const utxo of primary) {
        const out = await this.rpc.getTxOut(utxo.txid, utxo.vout)
        if (out === null) dropped.push(`${utxo.txid}:${utxo.vout}`)
        else kept.push(utxo)
      }
    } catch {
      // Bitcoin Core verification is best-effort: on the first RPC/transport
      // failure keep the Higun answer unfiltered rather than dropping funds.
      return primary
    }
    if (dropped.length && this.onDiscrepancy) {
      this.onDiscrepancy(
        address,
        `higun lists spent/unknown utxos: n=${dropped.length} samples=[${dropped.slice(0, 4).join(',')}]`
      )
    }
    return kept
  }

  async getFastestFee(): Promise<number> {
    if (this.rpc) {
      try {
        const fee = await this.rpc.estimateSmartFeeSatPerVb(2)
        if (fee !== null && fee > 0) return fee
      } catch {
        // fall through to guarded mempool.space
      }
    }
    const fee = await this.mempool.getFastestFee()
    if (Number.isFinite(fee) && fee > 0) return fee
    throw new Error('fee unavailable from Bitcoin Core and mempool.space')
  }

  async broadcastTx(hex: string): Promise<string> {
    if (this.rpc) {
      try {
        return await this.rpc.sendRawTransaction(hex)
      } catch (e: any) {
        if (e instanceof RpcError) {
          const msg = e.message.toLowerCase()
          if (e.code === -27 || msg.includes('already in') || msg.includes('txn-already-in-mempool') || msg.includes('txn-already-known')) {
            return bitcoin.Transaction.fromHex(hex).getId()
          }
          throw e
        }
        // transport error: fall through to mempool.space
      }
    }
    return this.mempool.broadcastTx(hex)
  }

  async getSweptInputState(txid: string, vout: number, sweepTxid: string): Promise<SweptInputState> {
    if (!this.rpc) return { state: 'unknown' }
    try {
      const out = await this.rpc.getTxOut(txid, vout)
      if (out !== null) return { state: 'unspent' }
      const sweep = await this.rpc.getRawTransactionVerbose(sweepTxid)
      if (sweep) {
        const ours = sweep.vin.some((input) => input.txid === txid && input.vout === vout)
        return ours ? { state: 'ours', confirmations: sweep.confirmations || 0 } : { state: 'foreign' }
      }
      return (await this.rpc.txIndexReady()) ? { state: 'foreign' } : { state: 'unknown' }
    } catch {
      return { state: 'unknown' }
    }
  }
}
