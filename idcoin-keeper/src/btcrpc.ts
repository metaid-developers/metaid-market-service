import axios, { AxiosInstance } from 'axios'

export class RpcError extends Error {
  constructor(public code: number, message: string) {
    super(message)
    this.name = 'RpcError'
  }
}

export interface RawTransactionVerbose {
  txid: string
  vin: { txid: string; vout: number }[]
  confirmations: number
}

function rpcErrorFromBody(data: any): RpcError | null {
  const err = data?.error
  if (!err || typeof err !== 'object') return null
  const code = Number(err.code)
  if (!Number.isFinite(code)) return null
  return new RpcError(code, String(err.message || `bitcoin rpc error ${code}`))
}

export class BitcoinRpcClient {
  private http: AxiosInstance
  private txindexSynced = false
  private nextId = 0

  constructor(public url: string, private user: string, private pass: string, http?: AxiosInstance) {
    this.http = http || axios.create({ timeout: 10_000 })
  }

  private async call(method: string, params: any[] = []): Promise<any> {
    const id = `idcoin-keeper-${++this.nextId}`
    let resp
    try {
      resp = await this.http.post(
        this.url,
        { jsonrpc: '1.0', id, method, params },
        { auth: { username: this.user, password: this.pass }, timeout: 10_000 }
      )
    } catch (e: any) {
      const rpcErr = rpcErrorFromBody(e?.response?.data)
      if (rpcErr) throw rpcErr
      throw new Error(`bitcoin rpc transport error: ${e.message || e}`)
    }
    const rpcErr = rpcErrorFromBody(resp?.data)
    if (rpcErr) throw rpcErr
    if (!resp?.data || typeof resp.data !== 'object' || !('result' in resp.data)) {
      throw new Error('bitcoin rpc transport error: malformed response')
    }
    return resp.data.result
  }

  async getTxOut(txid: string, vout: number): Promise<any | null> {
    const result = await this.call('gettxout', [txid, vout, true])
    return result === null ? null : result
  }

  async getRawTransactionVerbose(txid: string): Promise<RawTransactionVerbose | null> {
    try {
      const result = await this.call('getrawtransaction', [txid, true])
      if (!result || typeof result !== 'object') return null
      return {
        txid: String(result.txid || txid),
        vin: Array.isArray(result.vin)
          ? result.vin.map((v: any) => ({ txid: String(v.txid || ''), vout: Number(v.vout) }))
          : [],
        confirmations: Number(result.confirmations || 0),
      }
    } catch (e: any) {
      if (e instanceof RpcError && e.code === -5) return null
      throw e
    }
  }

  async sendRawTransaction(hex: string): Promise<string> {
    return String(await this.call('sendrawtransaction', [hex]))
  }

  async estimateSmartFeeSatPerVb(target = 2): Promise<number | null> {
    const result = await this.call('estimatesmartfee', [target])
    const feerate = Number(result?.feerate)
    if (!Number.isFinite(feerate) || feerate <= 0) return null
    return Math.max(1, Math.ceil(Math.round(feerate * 1e8) / 1000))
  }

  async txIndexReady(): Promise<boolean> {
    if (this.txindexSynced) return true
    try {
      const result = await this.call('getindexinfo')
      if (result?.txindex?.synced === true) {
        this.txindexSynced = true
        return true
      }
    } catch {
      // txindex readiness is best-effort; a later call retries
    }
    return false
  }
}
