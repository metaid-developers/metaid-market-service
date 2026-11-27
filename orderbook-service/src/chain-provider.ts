import { axios, Common, scrypt, TokenProto } from './core-bridge'

const { mvc } = scrypt

function canonicalV10TxId(tx: any): string {
  if (tx.version === 10) {
    const info = Common.getTxidInfo(tx)
    return mvc.crypto.Hash.sha256sha256(Buffer.from(info.txHeader, 'hex')).reverse().toString('hex')
  }
  return tx.id
}

export interface MvcUtxo {
  txid: string
  vout: number
  satoshis: number
  scriptHex: string
}

export interface TokenUtxo {
  txid: string
  vout: number
  satoshis: number
  scriptHex: string
  tokenAmount: bigint
  tokenID: string
  tokenCodeHash: string
}

export interface ChainProvider {
  getRawTx(txid: string): Promise<string>
  broadcastTx(txHex: string): Promise<string>
  getMvcUtxos(address: string): Promise<MvcUtxo[]>
  getTokenUtxos(address: string, tokenCodeHash: string, tokenID: string): Promise<TokenUtxo[]>
}

/**
 * Production ChainProvider backed by Higun (/mvc/tx/:txid/raw, /mvc/broadcast, /address/:addr/utxo)
 * with Metalet Wallet API fallback for FT token UTXOs.
 */
export class HigunChainProvider implements ChainProvider {
  private readonly higunUrl: string
  private readonly metaletApiBase: string
  private readonly net: 'livenet' | 'testnet'
  private readonly txCache = new Map<string, string>()

  constructor(opts: { higunUrl: string; metaletApiBase: string; net: 'livenet' | 'testnet' }) {
    this.higunUrl = opts.higunUrl.replace(/\/+$/, '')
    this.metaletApiBase = opts.metaletApiBase.replace(/\/+$/, '')
    this.net = opts.net
  }

  cacheRawTx(txid: string, rawHex: string): void {
    this.txCache.set(txid.toLowerCase(), rawHex)
  }

  async getRawTx(txid: string): Promise<string> {
    const key = txid.toLowerCase()
    const hit = this.txCache.get(key)
    if (hit) return hit

    if (this.higunUrl) {
      try {
        const resp = await axios.get(`${this.higunUrl}/mvc/tx/${txid}/raw`, { timeout: 15000 })
        const hex = resp.data?.hex || resp.data?.rawTx || (typeof resp.data === 'string' ? resp.data : '')
        if (hex) {
          this.txCache.set(key, hex)
          return hex
        }
      } catch (_) {}
    }

    const resp = await axios.get(`${this.metaletApiBase}/v3/tx/raw`, {
      params: { net: this.net, txId: txid, chain: 'mvc' },
      timeout: 15000,
    })
    const hex = resp.data?.data?.hex || resp.data?.data || ''
    if (!hex || typeof hex !== 'string') {
      throw new Error(`Unable to fetch raw tx ${txid}`)
    }
    this.txCache.set(key, hex)
    return hex
  }

  async broadcastTx(txHex: string): Promise<string> {
    if (this.higunUrl) {
      const resp = await axios.post(
        `${this.higunUrl}/mvc/broadcast`,
        { hex: txHex },
        { timeout: 20000 }
      )
      const txid = resp.data?.txid || resp.data?.data
      if (txid && typeof txid === 'string') {
        this.txCache.set(txid.toLowerCase(), txHex)
        return txid
      }
      throw new Error(`Higun broadcast failed: ${JSON.stringify(resp.data)}`)
    }

    const resp = await axios.post(
      `${this.metaletApiBase}/v3/tx/broadcast`,
      { chain: 'mvc', net: this.net, rawTx: txHex },
      { timeout: 20000 }
    )
    const txid = resp.data?.data
    if (!txid || typeof txid !== 'string') {
      throw new Error(`Metalet broadcast failed: ${JSON.stringify(resp.data)}`)
    }
    this.txCache.set(txid.toLowerCase(), txHex)
    return txid
  }

  async getMvcUtxos(address: string): Promise<MvcUtxo[]> {
    const p2pkhHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(address, this.net)).toHex()
    if (this.higunUrl) {
      try {
        const resp = await axios.get(`${this.higunUrl}/address/${address}/utxo`, { timeout: 15000 })
        const list = Array.isArray(resp.data) ? resp.data : resp.data?.data || []
        return list
          .filter((u: any) => Number(u.satoshis ?? u.value ?? u.amount ?? 0) > 0)
          .map((u: any) => ({
            txid: String(u.txid || u.txId),
            vout: Number(u.outIndex ?? u.vout ?? u.txIndex ?? 0),
            satoshis: Number(u.satoshis ?? u.value ?? u.amount),
            scriptHex: p2pkhHex,
          }))
      } catch (_) {}
    }

    const resp = await axios.get(`${this.metaletApiBase}/v3/address/mvc-utxo`, {
      params: { net: this.net, address },
      timeout: 15000,
    })
    const list = Array.isArray(resp.data?.data) ? resp.data.data : []
    return list.map((u: any) => ({
      txid: String(u.txid || u.txId),
      vout: Number(u.outIndex ?? u.vout ?? 0),
      satoshis: Number(u.value ?? u.satoshis ?? 0),
      scriptHex: p2pkhHex,
    }))
  }

  async getTokenUtxos(address: string, tokenCodeHash: string, tokenID: string): Promise<TokenUtxo[]> {
    const resp = await axios.get(`${this.metaletApiBase}/v3/address/contract/ft/utxo-list`, {
      params: {
        net: this.net,
        address,
        codeHash: tokenCodeHash,
        genesis: tokenID,
      },
      timeout: 15000,
    })
    const list = Array.isArray(resp.data?.data) ? resp.data.data : []
    const out: TokenUtxo[] = []
    for (const u of list) {
      const txid = String(u.txid || u.txId)
      const vout = Number(u.txIndex ?? u.vout ?? u.outIndex ?? 0)
      const rawHex = await this.getRawTx(txid)
      const tx = new mvc.Transaction(rawHex)
      const scriptBuf = tx.outputs[vout].script.toBuffer()
      out.push({
        txid,
        vout,
        satoshis: tx.outputs[vout].satoshis,
        scriptHex: scriptBuf.toString('hex'),
        tokenAmount: TokenProto.getTokenAmount(scriptBuf),
        tokenID: TokenProto.getTokenID(scriptBuf).toString('hex'),
        tokenCodeHash: TokenProto.getScriptCodeHash(scriptBuf).toString('hex'),
      })
    }
    return out
  }
}

/**
 * In-memory ChainProvider used for deterministic unit/integration tests and local verification.
 */
export class InMemoryChainProvider implements ChainProvider {
  readonly rawTxs = new Map<string, string>()
  readonly broadcasted: Array<{ txid: string; txHex: string }> = []
  private readonly mvcUtxos = new Map<string, MvcUtxo[]>()
  private readonly tokenUtxos = new Map<string, TokenUtxo[]>()

  registerTx(tx: any): string {
    const txid = canonicalV10TxId(tx)
    const hex = tx.uncheckedSerialize()
    this.rawTxs.set(txid.toLowerCase(), hex)
    this.rawTxs.set(tx.id.toLowerCase(), hex)
    return txid
  }

  setMvcUtxos(address: string, utxos: MvcUtxo[]): void {
    this.mvcUtxos.set(address, utxos)
  }

  setTokenUtxos(address: string, utxos: TokenUtxo[]): void {
    this.tokenUtxos.set(address, utxos)
  }

  async getRawTx(txid: string): Promise<string> {
    const raw = this.rawTxs.get(txid.toLowerCase())
    if (!raw) throw new Error(`InMemoryChainProvider: unknown txid ${txid}`)
    return raw
  }

  async broadcastTx(txHex: string): Promise<string> {
    const tx = new mvc.Transaction(txHex)
    const txid = canonicalV10TxId(tx)
    this.rawTxs.set(txid.toLowerCase(), txHex)
    this.rawTxs.set(tx.id.toLowerCase(), txHex)
    this.broadcasted.push({ txid, txHex })
    return txid
  }

  async getMvcUtxos(address: string): Promise<MvcUtxo[]> {
    return this.mvcUtxos.get(address) || []
  }

  async getTokenUtxos(address: string, tokenCodeHash: string, tokenID: string): Promise<TokenUtxo[]> {
    return (this.tokenUtxos.get(address) || []).filter(
      (u) =>
        u.tokenCodeHash.toLowerCase() === tokenCodeHash.toLowerCase() &&
        u.tokenID.toLowerCase() === tokenID.toLowerCase()
    )
  }
}
