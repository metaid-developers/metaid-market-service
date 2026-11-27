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
 * Normalize Metalet API base so both:
 *   - https://www.metalet.space/wallet-api
 *   - https://www.metalet.space/wallet-api/v4/mvc
 * resolve consistently to .../v4/mvc.
 */
function normalizeMetaletMvcBase(rawBase: string): string {
  const trimmed = rawBase.replace(/\/+$/, '')
  if (!trimmed) return 'https://www.metalet.space/wallet-api/v4/mvc'
  if (/\/v4\/mvc$/i.test(trimmed)) return trimmed
  if (/\/v3$/i.test(trimmed)) return trimmed.replace(/\/v3$/i, '/v4/mvc')
  return `${trimmed}/v4/mvc`
}

async function runWithConcurrency<T, R>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<R>
): Promise<R[]> {
  const results: R[] = new Array(items.length)
  let nextIdx = 0
  const runners = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (true) {
      const idx = nextIdx++
      if (idx >= items.length) break
      results[idx] = await worker(items[idx])
    }
  })
  await Promise.all(runners)
  return results
}

/**
 * Production ChainProvider backed by Higun (/mvc/tx/:txid/raw, /mvc/broadcast, /utxos)
 * and Metalet Wallet API v4 (/v4/mvc/address/utxo-list, /tx/raw, /tx/broadcast, /address/contract/ft/tx-list).
 */
export class HigunChainProvider implements ChainProvider {
  private readonly higunUrl: string
  private readonly metaletMvcBase: string
  private readonly net: 'livenet' | 'testnet'
  private readonly txCache = new Map<string, string>()

  constructor(opts: { higunUrl: string; metaletApiBase: string; net: 'livenet' | 'testnet' }) {
    this.higunUrl = opts.higunUrl.replace(/\/+$/, '')
    this.metaletMvcBase = normalizeMetaletMvcBase(opts.metaletApiBase)
    this.net = opts.net
  }

  cacheRawTx(txid: string, rawHex: string): void {
    this.txCache.set(txid.toLowerCase(), rawHex)
  }

  private async retry<T>(label: string, fn: () => Promise<T>, maxRetries = 4, delayMs = 600): Promise<T> {
    let lastErr: any
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await fn()
      } catch (e) {
        lastErr = e
        if (attempt < maxRetries) {
          await new Promise((r) => setTimeout(r, delayMs * attempt))
        }
      }
    }
    const detail = lastErr?.response?.data?.message || lastErr?.message || String(lastErr)
    throw new Error(`[ChainProvider] ${label} failed: ${detail}`)
  }

  async getRawTx(txid: string): Promise<string> {
    const key = txid.toLowerCase()
    const hit = this.txCache.get(key)
    if (hit) return hit

    if (this.higunUrl) {
      try {
        const resp = await axios.get(`${this.higunUrl}/mvc/tx/${txid}/raw`, { timeout: 15000 })
        const hex =
          resp.data?.txHex ||
          resp.data?.hex ||
          resp.data?.rawTx ||
          (typeof resp.data === 'string' && /^[0-9a-fA-F]+$/.test(resp.data.trim()) ? resp.data.trim() : '')
        if (hex && typeof hex === 'string') {
          this.txCache.set(key, hex)
          return hex
        }
      } catch (_) {
        // fall back to Metalet v4
      }
    }

    const hex = await this.retry(`getRawTx(${txid})`, async () => {
      const resp = await axios.get(`${this.metaletMvcBase}/tx/raw`, {
        params: { net: this.net, txId: txid },
        timeout: 20000,
      })
      const raw = resp.data?.data?.hex || (typeof resp.data?.data === 'string' ? resp.data.data : '')
      if (!raw || typeof raw !== 'string' || (resp.data?.code !== undefined && resp.data.code !== 0)) {
        throw new Error(`Unable to fetch raw tx ${txid}: ${JSON.stringify(resp.data)}`)
      }
      return raw
    })
    this.txCache.set(key, hex)
    return hex
  }

  async broadcastTx(txHex: string): Promise<string> {
    const parsed = new mvc.Transaction(txHex)
    const expectedTxid = canonicalV10TxId(parsed)

    const txid = await this.retry(`broadcastTx(${expectedTxid})`, async () => {
      if (this.higunUrl) {
        try {
          const resp = await axios.post(
            `${this.higunUrl}/mvc/broadcast`,
            { hex: txHex },
            { timeout: 25000 }
          )
          const id =
            resp.data?.txid ||
            resp.data?.txId ||
            resp.data?.TxId ||
            (typeof resp.data?.data === 'string' ? resp.data.data : '')
          if (id && typeof id === 'string') {
            return id
          }
          const bodyStr = JSON.stringify(resp.data || {})
          if (
            bodyStr.includes('already in block chain') ||
            bodyStr.includes('txn-already-known') ||
            bodyStr.includes('txn-already-in-mempool')
          ) {
            return expectedTxid
          }
        } catch (err: any) {
          const errStr = JSON.stringify(err?.response?.data || err?.message || '')
          if (
            errStr.includes('already in block chain') ||
            errStr.includes('txn-already-known') ||
            errStr.includes('txn-already-in-mempool')
          ) {
            return expectedTxid
          }
          // fall back to Metalet v4 broadcast
        }
      }

      const resp = await axios.post(
        `${this.metaletMvcBase}/tx/broadcast`,
        { net: this.net, rawTx: txHex },
        { timeout: 25000 }
      )
      if (resp.data && resp.data.code === 0 && resp.data.data) {
        const id =
          resp.data.data.TxId ||
          resp.data.data.txid ||
          resp.data.data.txId ||
          (typeof resp.data.data === 'string' ? resp.data.data : '')
        if (id && typeof id === 'string') {
          return id
        }
      }
      const msg = JSON.stringify(resp.data)
      if (
        msg.includes('already in block chain') ||
        msg.includes('txn-already-known') ||
        msg.includes('txn-already-in-mempool')
      ) {
        return expectedTxid
      }
      throw new Error(`Metalet broadcast rejected: ${msg}`)
    })

    this.txCache.set(txid.toLowerCase(), txHex)
    this.txCache.set(expectedTxid.toLowerCase(), txHex)
    return txid
  }

  async getMvcUtxos(address: string): Promise<MvcUtxo[]> {
    const p2pkhHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(address, this.net)).toHex()

    try {
      const all: MvcUtxo[] = []
      const seen = new Set<string>()
      let flag = ''
      for (let page = 0; page < 20; page++) {
        const list = await this.retry(`getMvcUtxos(${address},flag=${flag})`, async () => {
          const params: Record<string, any> = { net: this.net, address }
          if (flag) params.flag = flag
          const resp = await axios.get(`${this.metaletMvcBase}/address/utxo-list`, {
            params,
            timeout: 20000,
          })
          if (!resp.data || resp.data.code !== 0 || !resp.data.data) {
            throw new Error(`Unexpected utxo-list response: ${JSON.stringify(resp.data)}`)
          }
          return Array.isArray(resp.data.data.list) ? resp.data.data.list : []
        })
        if (list.length === 0) break
        for (const u of list) {
          const txid = String(u.txid || u.txId || '')
          const vout = Number(u.outIndex ?? u.vout ?? 0)
          const satoshis = Number(u.value ?? u.satoshis ?? 0)
          if (!txid || satoshis <= 0) continue
          const key = `${txid.toLowerCase()}:${vout}`
          if (seen.has(key)) continue
          seen.add(key)
          all.push({ txid, vout, satoshis, scriptHex: p2pkhHex })
        }
        const nextFlag = String(list[list.length - 1]?.flag || '')
        if (!nextFlag || nextFlag === '0' || nextFlag === flag || list.length < 10) break
        flag = nextFlag
      }
      return all
    } catch (metaletErr) {
      if (!this.higunUrl) throw metaletErr
      const resp = await axios.get(`${this.higunUrl}/utxos`, {
        params: { address },
        timeout: 15000,
      })
      const list = Array.isArray(resp.data?.utxos)
        ? resp.data.utxos
        : Array.isArray(resp.data)
          ? resp.data
          : resp.data?.data || []
      return list
        .filter((u: any) => Number(u.amount ?? u.satoshis ?? u.value ?? 0) > 0)
        .map((u: any) => ({
          txid: String(u.tx_id || u.txid || u.txId),
          vout: Number(u.index ?? u.outIndex ?? u.vout ?? u.txIndex ?? 0),
          satoshis: Number(u.amount ?? u.satoshis ?? u.value),
          scriptHex: p2pkhHex,
        }))
    }
  }

  async getTokenUtxos(address: string, tokenCodeHash: string, tokenID: string): Promise<TokenUtxo[]> {
    const addrHashHex = new mvc.Address(address, this.net).hashBuffer.toString('hex').toLowerCase()
    const wantCodeHash = tokenCodeHash.toLowerCase()
    const wantTokenID = tokenID.toLowerCase()

    // Paginate Metalet v4 ft/tx-list until flag === '0' (same battle-tested strategy as swap-engine.ts)
    const txIds: string[] = []
    const txIdSet = new Set<string>()
    let flag = ''
    for (let page = 0; page < 50; page++) {
      const list = await this.retry(`getFtTxList(${address},${tokenID},flag=${flag})`, async () => {
        const params: Record<string, any> = {
          net: this.net,
          address,
          codeHash: tokenCodeHash,
          genesis: tokenID,
          size: 20,
        }
        if (flag) params.flag = flag
        const resp = await axios.get(`${this.metaletMvcBase}/address/contract/ft/tx-list`, {
          params,
          timeout: 20000,
        })
        if (!resp.data || resp.data.code !== 0 || !resp.data.data) {
          throw new Error(`Unexpected ft tx-list response: ${JSON.stringify(resp.data)}`)
        }
        return Array.isArray(resp.data.data.list) ? resp.data.data.list : []
      })
      if (list.length === 0) break
      for (const item of list) {
        const id = String(item.txid || item.txId || '')
        if (id && !txIdSet.has(id.toLowerCase())) {
          txIdSet.add(id.toLowerCase())
          txIds.push(id)
        }
      }
      const nextFlag = String(list[list.length - 1]?.flag || '')
      if (!nextFlag || nextFlag === '0' || nextFlag === flag) break
      flag = nextFlag
    }

    if (txIds.length === 0) return []

    const rawHexes = await runWithConcurrency(txIds, 6, (id) => this.getRawTx(id))
    const parsedTxs = rawHexes.map((hex, idx) => {
      const tx = new mvc.Transaction(hex)
      const canonId = canonicalV10TxId(tx).toLowerCase()
      const legacyId = String(tx.id || '').toLowerCase()
      const apiId = txIds[idx].toLowerCase()
      return { tx, canonId, legacyId, apiId }
    })

    const spent = new Set<string>()
    for (const { tx } of parsedTxs) {
      for (const inp of tx.inputs) {
        spent.add(`${inp.prevTxId.toString('hex').toLowerCase()}:${inp.outputIndex}`)
      }
    }

    const out: TokenUtxo[] = []
    const seenOut = new Set<string>()
    for (const { tx, canonId, legacyId, apiId } of parsedTxs) {
      for (let vout = 0; vout < tx.outputs.length; vout++) {
        const output = tx.outputs[vout]
        const scriptBuf: Buffer = output.script.toBuffer()
        if (scriptBuf.length < TokenProto.DATA_LEN + 20) continue
        try {
          const outTokenID = TokenProto.getTokenID(scriptBuf).toString('hex').toLowerCase()
          const outCodeHash = TokenProto.getScriptCodeHash(scriptBuf).toString('hex').toLowerCase()
          const outAddr = TokenProto.getTokenAddress(scriptBuf).toString('hex').toLowerCase()
          if (outTokenID !== wantTokenID || outCodeHash !== wantCodeHash || outAddr !== addrHashHex) {
            continue
          }
          if (
            spent.has(`${canonId}:${vout}`) ||
            spent.has(`${legacyId}:${vout}`) ||
            spent.has(`${apiId}:${vout}`)
          ) {
            continue
          }
          const outKey = `${canonId}:${vout}`
          if (seenOut.has(outKey)) continue
          seenOut.add(outKey)
          out.push({
            txid: canonId,
            vout,
            satoshis: output.satoshis,
            scriptHex: scriptBuf.toString('hex'),
            tokenAmount: TokenProto.getTokenAmount(scriptBuf),
            tokenID: outTokenID,
            tokenCodeHash: outCodeHash,
          })
        } catch (_) {
          // non-token output
        }
      }
    }

    // Largest-first minimizes multi-UTXO merges when placing asks / token orders
    out.sort((a, b) => (a.tokenAmount > b.tokenAmount ? -1 : a.tokenAmount < b.tokenAmount ? 1 : 0))
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
