import * as fs from 'fs'
import * as path from 'path'
import { PoolRegistryEntry, ServiceConfig, savePoolsRegistry } from './config'
import { SwapEngine, TokenSwapProto, axios, loadSwapContractsFast, scrypt } from './core-bridge'

export interface PersistedPoolHead {
  poolTxid: string
  lpGenesisTxid: string
  lpGenesisVout: number
  updatedAt: string
}

export interface TxRecord {
  txid: string
  prepTxid: string
  depositTxid?: string
  poolId: string
  operation: 'swapMvcToToken' | 'swapTokenToMvc' | 'addLiquidity' | 'removeLiquidity'
  userAddress: string
  status: 'broadcasted' | 'confirmed' | 'failed'
  details: Record<string, string | number>
  reservesAfter: { r1: string; r2: string; lp: string }
  createdAt: string
  error?: string
}

export interface PoolLiveView {
  poolId: string
  pairName: string
  verified: boolean
  token1: PoolRegistryEntry['token1']
  token2: PoolRegistryEntry['token2']
  lpToken: PoolRegistryEntry['lpToken']
  swapCodeHash: string
  swapContractID: string
  poolTxid: string
  poolVout: number
  poolTokenVout: number
  lpGenesisTxid: string
  lpGenesisVout: number
  r1: string
  r2: string
  lp: string
  feeRateBps: number
  projFeeRateBps: number
  minProjFee: string
  spotPriceTokenPerSpace: string
  updatedAt: string
}

/**
 * Per-key FIFO async mutex with lease timeout so abandoned signing sessions never deadlock a pool.
 */
export class AsyncKeyedQueue {
  private readonly queues: Map<string, Array<() => void>> = new Map()
  private readonly active: Map<string, { release: () => void; timer: NodeJS.Timeout }> = new Map()

  async acquire(key: string, timeoutMs: number): Promise<() => void> {
    if (this.active.has(key)) {
      await new Promise<void>((resolve) => {
        const list = this.queues.get(key) || []
        list.push(resolve)
        this.queues.set(key, list)
      })
    }

    let released = false
    const release = () => {
      if (released) return
      released = true
      const entry = this.active.get(key)
      if (entry) {
        clearTimeout(entry.timer)
        this.active.delete(key)
      }
      const list = this.queues.get(key)
      if (list && list.length > 0) {
        const next = list.shift()!
        if (list.length === 0) this.queues.delete(key)
        next()
      }
    }

    const timer = setTimeout(() => {
      release()
    }, timeoutMs)

    this.active.set(key, { release, timer })
    return release
  }

  getQueueDepth(key: string): number {
    return (this.active.has(key) ? 1 : 0) + (this.queues.get(key)?.length || 0)
  }
}

export class PoolStore {
  public readonly config: ServiceConfig
  public readonly chain: any
  public readonly contracts: any
  public readonly poolQueue: AsyncKeyedQueue
  private readonly heads: Map<string, PersistedPoolHead> = new Map()
  private readonly cachedStates: Map<string, any> = new Map()
  private readonly txHistory: Map<string, TxRecord> = new Map()
  // Track recently spent / created outputs in memory for instant 0-conf chaining before Metalet indexer catches up
  private readonly spentOutpoints: Set<string> = new Set()
  private readonly localSpaceUtxos: Map<string, Array<{ txid: string; vout: number; satoshis: number; height: number }>> = new Map()
  private readonly localTokenUtxos: Map<string, any[]> = new Map()
  private readonly headsFile: string
  private readonly historyFile: string

  constructor(config: ServiceConfig) {
    this.config = config
    this.chain = new SwapEngine.MetaletChainClient({
      apiBase: config.metaletApiBase,
      net: config.net,
      higunBase: config.higunUrl,
    })
    this.contracts = loadSwapContractsFast()
    this.poolQueue = new AsyncKeyedQueue()
    this.headsFile = path.join(config.dataDir, 'pool-heads.json')
    this.historyFile = path.join(config.dataDir, 'tx-history.json')
    this.loadPersisted()
  }

  private loadPersisted() {
    if (fs.existsSync(this.headsFile)) {
      try {
        const raw = JSON.parse(fs.readFileSync(this.headsFile, 'utf8'))
        for (const [k, v] of Object.entries(raw)) {
          this.heads.set(k, v as PersistedPoolHead)
        }
      } catch (_) {}
    }
    for (const p of this.config.pools) {
      if (!this.heads.has(p.poolId)) {
        this.heads.set(p.poolId, {
          poolTxid: p.bootstrapHead.poolTxid,
          lpGenesisTxid: p.bootstrapHead.lpGenesisTxid,
          lpGenesisVout: p.bootstrapHead.lpGenesisVout,
          updatedAt: new Date().toISOString(),
        })
      }
    }
    if (fs.existsSync(this.historyFile)) {
      try {
        const list: TxRecord[] = JSON.parse(fs.readFileSync(this.historyFile, 'utf8'))
        for (const r of list) {
          this.txHistory.set(r.txid, r)
        }
      } catch (_) {}
    }
  }

  private persistHeads() {
    const obj: Record<string, PersistedPoolHead> = {}
    for (const [k, v] of this.heads.entries()) obj[k] = v
    fs.writeFileSync(this.headsFile, JSON.stringify(obj, null, 2))
  }

  private persistHistory() {
    const list = Array.from(this.txHistory.values()).sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
    fs.writeFileSync(this.historyFile, JSON.stringify(list.slice(0, 500), null, 2))
  }

  getPoolEntry(poolId: string): PoolRegistryEntry {
    const entry = this.config.pools.find((p) => p.poolId === poolId)
    if (!entry) throw new Error(`Pool not found or not whitelist-verified: ${poolId}`)
    return entry
  }

  /**
   * Optional Phase 4 higun discovery: queries GET /swap/utxos?codehash=...
   * and refreshes any matching pool's head txid automatically.
   */
  async syncFromHigunIfConfigured(): Promise<void> {
    if (!this.config.higunUrl) return
    const base = this.config.higunUrl.replace(/\/$/, '')
    for (const codehash of this.config.whitelistSwapCodeHashes) {
      try {
        const r = await axios.get(`${base}/swap/utxos`, {
          params: { codehash, limit: 100 },
          timeout: 10000,
        })
        const utxos = r.data && Array.isArray(r.data.utxos) ? r.data.utxos : []
        for (const u of utxos) {
          const scriptBuf = Buffer.from(u.script_hex, 'hex')
          const parsed = SwapEngine.parseSwapMainScript(scriptBuf)
          const cid = parsed.swapContractID.toString('hex')
          const matched = this.config.pools.find((p) => p.contract.swapContractID === cid)
          if (matched) {
            const prev = this.heads.get(matched.poolId)
            if (!prev || prev.poolTxid !== u.txid) {
              this.heads.set(matched.poolId, {
                poolTxid: u.txid,
                lpGenesisTxid: prev ? prev.lpGenesisTxid : matched.bootstrapHead.lpGenesisTxid,
                lpGenesisVout: prev ? prev.lpGenesisVout : matched.bootstrapHead.lpGenesisVout,
                updatedAt: new Date().toISOString(),
              })
              this.cachedStates.delete(matched.poolId)
              this.persistHeads()
            }
          }
        }
      } catch (_) {
        // non-fatal; keep tracked head
      }
    }
  }

  async getPoolState(poolId: string, forceRefresh = false): Promise<any> {
    this.getPoolEntry(poolId)
    if (!forceRefresh && this.cachedStates.has(poolId)) {
      return this.cachedStates.get(poolId)
    }
    const head = this.heads.get(poolId)!
    const state = await SwapEngine.loadPoolState(
      this.chain,
      head.poolTxid,
      head.lpGenesisTxid,
      head.lpGenesisVout
    )
    this.cachedStates.set(poolId, state)
    return state
  }

  async advancePoolHead(
    poolId: string,
    newHead: { poolTxid: string; lpGenesisTxid: string; lpGenesisVout: number },
    rawTxsToSeed: Array<{ txid: string; rawHex: string }>
  ): Promise<any> {
    for (const item of rawTxsToSeed) {
      this.chain.seedRawTx(item.txid, item.rawHex)
      this.recordBroadcastedTxUtxoChanges(item.rawHex)
    }
    this.heads.set(poolId, {
      ...newHead,
      updatedAt: new Date().toISOString(),
    })
    this.persistHeads()
    this.cachedStates.delete(poolId)
    return this.getPoolState(poolId, true)
  }

  /**
   * Tracks spent outpoints and newly created P2PKH/FT outputs from our just-broadcasted txs
   * so consecutive/concurrent operations chain seamlessly on 0-conf without waiting for indexer lag.
   */
  recordBroadcastedTxUtxoChanges(rawHex: string): void {
    const tx = new scrypt.mvc.Transaction(rawHex)
    for (const inp of tx.inputs) {
      const key = `${inp.prevTxId.toString('hex')}:${inp.outputIndex}`
      this.spentOutpoints.add(key)
    }
    for (let vout = 0; vout < tx.outputs.length; vout++) {
      const out = tx.outputs[vout]
      if (out.script.isPublicKeyHashOut()) {
        const addr = out.script.toAddress(scrypt.mvc.Networks.livenet).toString()
        const existing = this.localSpaceUtxos.get(addr) || []
        const key = `${tx.id}:${vout}`
        if (!existing.some((u) => `${u.txid}:${u.vout}` === key)) {
          existing.push({ txid: tx.id, vout, satoshis: out.satoshis, height: 0 })
          this.localSpaceUtxos.set(addr, existing)
        }
      } else {
        const buf: Buffer = out.script.toBuffer()
        if (buf.length > 200) {
          try {
            const TokenProto = require('./core-bridge').TokenProto
            const tokId = TokenProto.getTokenID(buf).toString('hex')
            const codeHash = TokenProto.getScriptCodeHash(buf).toString('hex')
            const addrHash = TokenProto.getTokenAddress(buf)
            const addr = scrypt.mvc.Address.fromPublicKeyHash(addrHash, scrypt.mvc.Networks.livenet).toString()
            const mapKey = `${addr}:${codeHash}:${tokId}`
            const list = this.localTokenUtxos.get(mapKey) || []
            const outKey = `${tx.id}:${vout}`
            if (!list.some((u) => `${u.txid}:${u.vout}` === outKey)) {
              list.push({
                txid: tx.id,
                vout,
                satoshis: out.satoshis,
                amount: TokenProto.getTokenAmount(buf),
                tokenID: tokId,
                codeHash,
                scriptBuf: buf,
                tx,
              })
              this.localTokenUtxos.set(mapKey, list)
            }
          } catch (_) {}
        }
      }
    }
  }

  /**
   * Fetch live SPACE UTXOs for an address merged with our local 0-conf cache and filtered against spent outpoints.
   */
  async getFreshSpaceUtxos(address: string): Promise<Array<{ txid: string; vout: number; satoshis: number; height: number }>> {
    const chainUtxos = await this.chain.getSpaceUtxos(address)
    const merged = new Map<string, { txid: string; vout: number; satoshis: number; height: number }>()
    for (const u of chainUtxos) {
      const k = `${u.txid}:${u.vout}`
      if (!this.spentOutpoints.has(k)) merged.set(k, u)
    }
    const local = this.localSpaceUtxos.get(address) || []
    for (const u of local) {
      const k = `${u.txid}:${u.vout}`
      if (!this.spentOutpoints.has(k)) merged.set(k, u)
    }
    return Array.from(merged.values()).sort((a, b) => b.satoshis - a.satoshis)
  }

  /**
   * Fetch live unspent FT UTXOs for address + codeHash + tokenID merged with local 0-conf cache.
   */
  async getFreshTokenUtxos(address: string, codeHash: string, tokenID: string): Promise<any[]> {
    const chainUtxos = await this.chain.resolveUserTokenUtxos(address, codeHash, tokenID)
    const merged = new Map<string, any>()
    for (const u of chainUtxos) {
      const k = `${u.txid}:${u.vout}`
      if (!this.spentOutpoints.has(k)) merged.set(k, u)
    }
    const local = this.localTokenUtxos.get(`${address}:${codeHash}:${tokenID}`) || []
    for (const u of local) {
      const k = `${u.txid}:${u.vout}`
      if (!this.spentOutpoints.has(k)) merged.set(k, u)
    }
    return Array.from(merged.values()).sort((a, b) => (b.amount > a.amount ? 1 : -1))
  }

  formatPoolView(entry: PoolRegistryEntry, state: any): PoolLiveView {
    const scale = BigInt(100000000)
    const spotScaled = state.r1 > BigInt(0) ? (state.r2 * scale) / state.r1 : BigInt(0)
    const whole = spotScaled / scale
    const frac = (spotScaled % scale).toString().padStart(8, '0').replace(/0+$/, '')
    const spotStr = frac ? `${whole}.${frac}` : whole.toString()

    return {
      poolId: entry.poolId,
      pairName: entry.pairName,
      verified: entry.verified,
      token1: entry.token1,
      token2: entry.token2,
      lpToken: entry.lpToken,
      swapCodeHash: entry.contract.swapCodeHash,
      swapContractID: entry.contract.swapContractID,
      poolTxid: state.poolTxid,
      poolVout: state.poolVout,
      poolTokenVout: state.poolTokenVout,
      lpGenesisTxid: state.lpGenesisTxid,
      lpGenesisVout: state.lpGenesisVout,
      r1: state.r1.toString(),
      r2: state.r2.toString(),
      lp: state.lp.toString(),
      feeRateBps: state.feeRate,
      projFeeRateBps: state.projFeeRate,
      minProjFee: state.minProjFee.toString(),
      spotPriceTokenPerSpace: spotStr,
      updatedAt: this.heads.get(entry.poolId)?.updatedAt || new Date().toISOString(),
    }
  }

  async listPools(forceRefresh = false): Promise<PoolLiveView[]> {
    await this.syncFromHigunIfConfigured()
    const out: PoolLiveView[] = []
    for (const entry of this.config.pools) {
      const state = await this.getPoolState(entry.poolId, forceRefresh)
      out.push(this.formatPoolView(entry, state))
    }
    return out
  }

  recordTx(rec: TxRecord): void {
    this.txHistory.set(rec.txid, rec)
    this.persistHistory()
  }

  getTx(txid: string): TxRecord | undefined {
    return this.txHistory.get(txid)
  }

  listTxHistory(filter?: { address?: string; poolId?: string; limit?: number }): TxRecord[] {
    let list = Array.from(this.txHistory.values())
    if (filter?.address) list = list.filter((r) => r.userAddress === filter.address)
    if (filter?.poolId) list = list.filter((r) => r.poolId === filter.poolId)
    list.sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
    return list.slice(0, filter?.limit || 50)
  }

  listRegistryEntries(): { whitelistSwapCodeHashes: string[]; pools: PoolRegistryEntry[] } {
    return {
      whitelistSwapCodeHashes: [...this.config.whitelistSwapCodeHashes],
      pools: [...this.config.pools],
    }
  }

  /**
   * Hot-registers (or updates when overwrite=true) a whitelist-verified pool in memory
   * and atomically persists it to pools.json + pool-heads.json without requiring a restart.
   */
  async registerPool(
    entry: PoolRegistryEntry,
    options?: {
      overwrite?: boolean
      rawTxsToSeed?: Array<{ txid: string; rawHex: string }>
    }
  ): Promise<PoolLiveView> {
    if (!entry.verified) {
      throw new Error(`Pool ${entry.poolId} must have verified=true`)
    }
    if (!this.config.whitelistSwapCodeHashes.includes(entry.contract.swapCodeHash)) {
      throw new Error(
        `Pool ${entry.poolId} swapCodeHash ${entry.contract.swapCodeHash} is not in whitelistSwapCodeHashes`
      )
    }

    const existingIdx = this.config.pools.findIndex((p) => p.poolId === entry.poolId)
    if (existingIdx >= 0 && !options?.overwrite) {
      throw new Error(`Pool "${entry.poolId}" already exists (pass overwrite=true to update)`)
    }
    const dupContract = this.config.pools.find(
      (p) => p.contract.swapContractID === entry.contract.swapContractID && p.poolId !== entry.poolId
    )
    if (dupContract) {
      throw new Error(
        `swapContractID ${entry.contract.swapContractID} is already registered under poolId "${dupContract.poolId}"`
      )
    }

    if (options?.rawTxsToSeed) {
      for (const item of options.rawTxsToSeed) {
        this.chain.seedRawTx(item.txid, item.rawHex)
        this.recordBroadcastedTxUtxoChanges(item.rawHex)
      }
    }

    // Verify pool head is readable and live before committing registry state
    const state = await SwapEngine.loadPoolState(
      this.chain,
      entry.bootstrapHead.poolTxid,
      entry.bootstrapHead.lpGenesisTxid,
      entry.bootstrapHead.lpGenesisVout
    )

    if (existingIdx >= 0) {
      this.config.pools[existingIdx] = entry
    } else {
      this.config.pools.push(entry)
    }

    this.heads.set(entry.poolId, {
      poolTxid: entry.bootstrapHead.poolTxid,
      lpGenesisTxid: entry.bootstrapHead.lpGenesisTxid,
      lpGenesisVout: entry.bootstrapHead.lpGenesisVout,
      updatedAt: new Date().toISOString(),
    })
    this.cachedStates.set(entry.poolId, state)

    savePoolsRegistry(this.config.poolsFile, this.config.whitelistSwapCodeHashes, this.config.pools)
    this.persistHeads()

    return this.formatPoolView(entry, state)
  }

  /**
   * Hot-removes a pool from memory and atomically updates pools.json.
   */
  removePool(poolId: string): PoolRegistryEntry {
    const idx = this.config.pools.findIndex((p) => p.poolId === poolId)
    if (idx < 0) {
      throw new Error(`Pool "${poolId}" not found`)
    }
    const [removed] = this.config.pools.splice(idx, 1)
    this.cachedStates.delete(poolId)
    this.heads.delete(poolId)
    savePoolsRegistry(this.config.poolsFile, this.config.whitelistSwapCodeHashes, this.config.pools)
    this.persistHeads()
    return removed
  }
}
