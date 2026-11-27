import axios, { AxiosInstance } from 'axios'

export interface ChainUtxo {
  txid: string
  vout: number
  sats: number
  confirmed: boolean
  inscriptionBearing: boolean
}

export interface ChainClient {
  getUtxos(address: string): Promise<ChainUtxo[]>
  getFastestFee(): Promise<number>
  broadcastTx(hex: string): Promise<string>
  /** returns the spending txid of outpoint txid:vout, or '' if unspent/unknown */
  getOutspend(txid: string, vout: number): Promise<string>
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

/** mempool.space — cross-check, fee rates, broadcast, outspend verification. */
export class MempoolClient {
  private http: AxiosInstance
  constructor(public baseUrl: string, http?: AxiosInstance) {
    this.http = http || axios.create({ timeout: 15_000 })
  }

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    const resp = await this.http.get(`${this.baseUrl}/api/address/${address}/utxo`)
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
    const resp = await this.http.get(`${this.baseUrl}/api/v1/fees/recommended`)
    const fee = Number(resp.data?.fastestFee)
    return Number.isFinite(fee) && fee > 0 ? fee : 1
  }

  async broadcastTx(hex: string): Promise<string> {
    const resp = await this.http.post(`${this.baseUrl}/api/tx`, hex, {
      headers: { 'Content-Type': 'text/plain' },
    })
    return String(resp.data)
  }

  async getOutspend(txid: string, vout: number): Promise<string> {
    const resp = await this.http.get(`${this.baseUrl}/api/tx/${txid}/outspends`)
    const arr: any[] = Array.isArray(resp.data) ? resp.data : []
    const rec = arr[vout]
    if (!rec || !rec.spent) return ''
    return String(rec.txid || '')
  }
}

/** Composite: Higun primary with mempool.space cross-check. */
export class CompositeChainClient implements ChainClient {
  constructor(
    public higun: HigunBtcClient,
    public mempool: MempoolClient,
    private onDiscrepancy?: (address: string, detail: string) => void
  ) {}

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    const primary = await this.higun.getUtxos(address)
    try {
      const cross = await this.mempool.getUtxos(address)
      const key = (u: { txid: string; vout: number }) => `${u.txid}:${u.vout}`
      const pSet = new Set(primary.map(key))
      const cSet = new Set(cross.map(key))
      const onlyHigun = [...pSet].filter((k) => !cSet.has(k))
      const onlyMempool = [...cSet].filter((k) => !pSet.has(k))
      if ((onlyHigun.length || onlyMempool.length) && this.onDiscrepancy) {
        this.onDiscrepancy(
          address,
          `higun/mempool utxo mismatch: only-higun=${onlyHigun.length} only-mempool=${onlyMempool.length} ` +
            `samples=[${[...onlyHigun, ...onlyMempool].slice(0, 4).join(',')}]`
        )
      }
    } catch {
      // cross-check is best-effort; primary already answered
    }
    return primary
  }

  getFastestFee(): Promise<number> {
    return this.mempool.getFastestFee()
  }

  broadcastTx(hex: string): Promise<string> {
    return this.mempool.broadcastTx(hex)
  }

  getOutspend(txid: string, vout: number): Promise<string> {
    return this.mempool.getOutspend(txid, vout)
  }
}
