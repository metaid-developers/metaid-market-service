/**
 * P2 pipeline: batch planning -> bridge btc_mint -> mint tracking -> floor bid.
 * State machine persisted in SQLite; every step idempotent/resumable.
 *
 *   planned -> order_created -> deposit_signed -> deposit_submitted
 *     -> minted -> bid_placed    (| failed | held)
 */
import { KeeperDb, CoinRow } from './db'
import { KeeperConfig } from './config'
import { Logger } from './log'
import { BridgeClient, validateCreateOrderResponse, signMessageMvc } from './bridge'
import { planBatches, buildDepositTx, validateDepositInvariants, vaultNodeForTick, VaultUtxo } from './batch'
import { computeFloorPrice, floorBidParams, placeFloorBid, BidExpectation, WBTC_CODEHASH, WBTC_TOKEN_ID, batchSaltHex } from './bid'
import { OrderTokenProto } from '../../orderbook-service/src/core-bridge'
import { ChainClient } from './chain'
import { LpKeys } from './keys'
import { OrderbookTxBuilder } from '../../orderbook-service/src/tx-builder'
import { AssetPairInfo } from './bridge'

const ACTIVE_STATES = ['planned', 'order_created', 'deposit_signed', 'deposit_submitted', 'minted', 'bid_submitting']

export interface PipelineDeps {
  config: KeeperConfig
  db: KeeperDb
  log: Logger
  bridge: BridgeClient
  chain: ChainClient           // BTC chain (fee rates)
  lpKeys: LpKeys
  obBuilder?: OrderbookTxBuilder   // undefined in dry-run-only/test setups
  mvcFundingPrivKey?: any          // mvc-lib PrivateKey for the LP funding key
  mvcChain?: { getMvcUtxos: (address: string) => Promise<Array<{ satoshis: number }>> }
  pairs?: AssetPairInfo[]          // injected bridge asset list (or fetched)
  depthCheck?: (pairKey: string) => Promise<{ bidSeen: boolean }>
  contractFingerprints?: Buffer[]
  /** source of truth for live OrderTokenV2 orders: Higun /orderbook/utxos items */
  orderLookup?: (codehash: string) => Promise<Array<{ txid: string; vout: number; scriptHex: string }>>
  /** raw-unit WBTC balance of the funding address (sum of token utxos) */
  wbtcBalance?: () => Promise<bigint>
  tokenOrderCodeHash?: string
  /** broadcast a signed MVC tx hex (keeper broadcasts bid txs itself) */
  mvcBroadcast?: (hex: string) => Promise<string>
  /** true if the txid is retrievable on-chain (mempool or confirmed) */
  mvcTxExists?: (txid: string) => Promise<boolean>
  /** verify an unspent WBTC token UTXO locked to hash160(orderScript) carrying
   *  exactly remainAmount exists — required for salt-match adoption */
  verifyLockedTokens?: (orderScriptHex: string, remainAmount: bigint) => Promise<boolean>
  /** test seam: override the bid build/sign call (never broadcasts) */
  placeBid?: typeof placeFloorBid
}

export class Pipeline {
  constructor(private d: PipelineDeps) {}

  pairFor(tickId: string): AssetPairInfo | undefined {
    return (this.d.pairs || []).find((p) => p.originTokenId === tickId)
  }

  /** one advancement pass over everything: plan new batches + advance active ones */
  async tick(now = Date.now()): Promise<void> {
    if (this.d.config.killSwitch) return
    await this.planNewBatches(now)
    for (const batch of this.d.db.batchesByState(ACTIVE_STATES)) {
      try {
        await this.advance(batch, now)
      } catch (e: any) {
        this.d.log.error('batch advance error', { batchId: batch.id, error: e.message })
      }
    }
  }

  // -------------------------------------------------------------------------
  async planNewBatches(now: number): Promise<void> {
    const coins = this.d.db.listCoins()
    for (const coin of coins) {
      if (!this.d.config.enableTicks.has(coin.tick.toUpperCase())) continue
      if (coin.mode === 'unknown') continue
      if (!this.pairFor(coin.tickId)) continue // not bridgeable — BTC stays in the vault
      const vaultUtxos: VaultUtxo[] = this.d.db
        .vaultUtxos(coin.tickId)
        .filter((v: any) => v.confirmed === 1 && !this.d.db.isBatched(v.outpoint))
        .map((v: any) => {
          const [txid, vout] = v.outpoint.split(':')
          return { outpoint: v.outpoint, txid, vout: Number(vout), sats: v.sats, confirmed: true }
        })
      if (vaultUtxos.length === 0) continue

      let feeQuote
      try {
        feeQuote = await this.d.bridge.feeQuote()
      } catch (e: any) {
        this.d.log.warn('feeQuote unavailable, skipping batch planning', { tick: coin.tick, error: e.message })
        continue
      }
      const fastest = await this.d.chain.getFastestFee().catch(() => 0)
      const feeRate = Math.max(fastest, this.d.config.feeFloorSatPerVb)
      const res = planBatches({
        utxos: vaultUtxos,
        btcFeeSats: feeQuote.btcFeeSats,
        feeRateSatPerVb: feeRate,
        minSats: this.d.config.bridgeMinSats,
        maxSats: this.d.config.bridgeMaxSats,
        haircutBps: this.d.config.batchHaircutBps,
      })
      if (res.kind === 'wait') {
        this.d.log.info('batch waiting', { tick: coin.tick, reason: res.reason })
        continue
      }
      for (const plan of res.plans) {
        const id = this.d.db.createBatch({
          tickId: coin.tickId,
          inputs: plan.inputs.map((u) => ({ outpoint: u.outpoint, sats: u.sats })),
          totalIn: plan.totalIn,
          estTxFee: plan.estTxFee,
          btcFeeSats: plan.btcFeeSats,
          amount: plan.amount,
          now,
        })
        this.d.log.info('batch planned', {
          batchId: id, tick: coin.tick, inputs: plan.inputs.length,
          totalIn: plan.totalIn, amount: plan.amount, btcFeeSats: plan.btcFeeSats,
        })
      }
    }
  }

  // -------------------------------------------------------------------------
  async advance(batch: any, now: number): Promise<void> {
    switch (batch.state) {
      case 'planned':
        return this.advancePlanned(batch, now)
      case 'order_created':
        return this.advanceOrderCreated(batch, now)
      case 'deposit_signed':
        return this.advanceDepositSigned(batch, now)
      case 'deposit_submitted':
        return this.advanceDepositSubmitted(batch, now)
      case 'minted':
        return this.advanceMinted(batch, now)
      case 'bid_submitting':
        return this.advanceBidSubmitting(batch, now)
    }
  }

  private coinFor(batch: any): CoinRow | undefined {
    return this.d.db.listCoins().find((c) => c.tickId === batch.tick_id)
  }

  /** planned -> order_created (validate response BEFORE signing anything) */
  private async advancePlanned(batch: any, now: number): Promise<void> {
    if (this.d.config.dryRun) return // dry-run never calls createPrepayOrder
    const coin = this.coinFor(batch)
    if (!coin) return this.fail(batch, 'coin missing from registry', now)
    const vaultNode = vaultNodeForTick(this.d.lpKeys.accountNode, coin.tick)
    const publicKey = Buffer.from(vaultNode.publicKey).toString('hex')
    const publicKeyReceive = Buffer.from(this.d.lpKeys.mvcFundingNode.publicKey).toString('hex')

    const res = await this.d.bridge.createMintBtcOrder({
      amountSats: batch.amount,
      publicKey,
      publicKeyReceive,
      sign: (text) => signMessageMvc({ sign: (h) => vaultNode.sign(h) }, text),
      receiveSign: (text) => signMessageMvc({ sign: (h) => this.d.lpKeys.mvcFundingNode.sign(h) }, text),
    })
    const check = validateCreateOrderResponse({
      res,
      expectedAmountSats: batch.amount,
      pinnedBridgeVault: this.d.config.bridgeVaultAddress,
      feeAllowlist: this.d.config.bridgeFeeAllowlist,
      maxBtcFeeSats: batch.btc_fee_sats * 2 + 1000, // quote drift tolerance
    })
    if (!check.ok) {
      return this.fail(batch, `create-order response rejected: ${check.reason}`, now)
    }
    this.d.db.updateBatch(batch.id, {
      state: 'order_created',
      order_id: res.orderId,
      bridge_address: res.bridgeAddress,
      fee_address: res.btcFeeAddress,
      btc_fee_sats: res.btcFeeSats, // authoritative fee from the order itself
    }, now)
    this.d.log.info('bridge order created', { batchId: batch.id, orderId: res.orderId })
  }

  /** order_created -> deposit_signed (deterministic rebuild; persist hex BEFORE submit) */
  private async advanceOrderCreated(batch: any, now: number): Promise<void> {
    const coin = this.coinFor(batch)
    if (!coin) return this.fail(batch, 'coin missing', now)
    const vaultNode = vaultNodeForTick(this.d.lpKeys.accountNode, coin.tick)
    const inputs: VaultUtxo[] = JSON.parse(batch.input_outpoints).map((op: string) => {
      const [txid, vout] = op.split(':')
      const row = this.d.db.db.prepare('SELECT sats FROM batch_inputs WHERE outpoint = ?').get(op) as any
      return { outpoint: op, txid, vout: Number(vout), sats: row.sats, confirmed: true }
    })
    const built = buildDepositTx({
      vaultNode,
      inputs,
      bridgeAddress: batch.bridge_address,
      feeAddress: batch.fee_address,
      amount: batch.amount,
      btcFeeSats: batch.btc_fee_sats,
    })
    const vaultAddress = coin.lpVault
    const check = validateDepositInvariants({
      hex: built.hex,
      inputs,
      vaultAddress,
      bridgeAddress: batch.bridge_address,
      feeAddress: batch.fee_address,
      amount: batch.amount,
      btcFeeSats: batch.btc_fee_sats,
    })
    if (!check.ok) return this.fail(batch, `deposit invariant violation: ${check.reason}`, now)
    this.d.db.updateBatch(batch.id, {
      state: 'deposit_signed',
      deposit_txid: built.txid,
      deposit_hex: built.hex,
    }, now)
    this.d.log.info('deposit signed+persisted', { batchId: batch.id, txid: built.txid })
  }

  /** deposit_signed -> deposit_submitted (the BRIDGE broadcasts) */
  private async advanceDepositSigned(batch: any, now: number): Promise<void> {
    if (this.d.config.dryRun) return
    const res = await this.d.bridge.submitMintBtcOrder(batch.order_id, batch.deposit_hex)
    if (res.ok) {
      this.d.db.updateBatch(batch.id, { state: 'deposit_submitted' }, now)
      this.d.log.info('deposit submitted', { batchId: batch.id, orderId: batch.order_id })
      return
    }
    // idempotent resume: a lost response leaves the order already submitted
    if (/already submit|same tx already/i.test(res.msg)) {
      this.d.db.updateBatch(batch.id, { state: 'deposit_submitted' }, now)
      this.d.log.info('deposit already submitted (resume)', { batchId: batch.id })
      return
    }
    return this.fail(batch, `submit rejected: ${res.msg}`, now)
  }

  /** deposit_submitted -> minted (poll orderStatus) */
  private async advanceDepositSubmitted(batch: any, now: number): Promise<void> {
    const st = await this.d.bridge.orderStatus(batch.order_id)
    if (st.failureReason) {
      return this.fail(batch, `bridge order failed: ${st.failureReason}`, now)
    }
    if (st.stage === 'completed' && st.targetTxids.length > 0) {
      this.d.db.updateBatch(batch.id, {
        state: 'minted',
        mint_txid: st.targetTxids[st.targetTxids.length - 1],
        minted_wbtc: batch.amount, // 1:1
      }, now)
      this.d.log.info('batch minted on MVC', { batchId: batch.id, mintTxid: st.targetTxids[0] })
    }
  }

  /**
   * On-chain idempotency lookup: scan live OrderTokenV2 UTXOs (Higun
   * /orderbook/utxos, the orderbook contract-family index) for an order whose
   * tail carries this batch's deterministic salt + burn maker + the expected
   * tokens and price. The tail salt makes the order script unique per batch.
   */
  async findExistingBid(batch: any, exp: BidExpectation, saltHex: string): Promise<{ txid: string; vout: number; scriptHex: string; remainAmount: bigint } | null> {
    if (!this.d.orderLookup || !this.d.tokenOrderCodeHash) return null
    const saltBuf = Buffer.from(saltHex, 'hex')
    const zeroHash = Buffer.alloc(20, 0)
    let items: Array<{ txid: string; vout: number; scriptHex: string }>
    try {
      items = await this.d.orderLookup(this.d.tokenOrderCodeHash)
    } catch (e: any) {
      this.d.log.warn('order lookup failed — treating as unknown, not placing', { batchId: batch.id, error: e.message })
      throw e
    }
    for (const it of items) {
      let tail: any = null
      try {
        tail = OrderTokenProto.parseOrderTokenScript(Buffer.from(it.scriptHex, 'hex'))
      } catch {
        continue
      }
      if (!tail) continue
      if (!Buffer.from(tail.salt).equals(saltBuf)) continue
      if (!Buffer.from(tail.makerAddr).equals(zeroHash)) continue
      if (Buffer.from(tail.sellTokenCodeHash).toString('hex') !== exp.sellTokenCodeHash) continue
      if (Buffer.from(tail.sellTokenID).toString('hex') !== exp.sellTokenID) continue
      if (Buffer.from(tail.buyTokenCodeHash).toString('hex') !== exp.buyTokenCodeHash) continue
      if (Buffer.from(tail.buyTokenID).toString('hex') !== exp.buyTokenID) continue
      if (BigInt(tail.priceNum) !== exp.priceNum || BigInt(tail.priceDen) !== exp.priceDen) continue
      return { txid: it.txid, vout: it.vout, scriptHex: it.scriptHex, remainAmount: BigInt(tail.remainAmount) }
    }
    return null
  }

  /** per-coin WBTC accounting: minted minus already-placed. */
  coinWbtcAccounting(tickId: string): { minted: number; inBids: number; unplaced: number } {
    const batches = this.d.db.batchesByState()
    let minted = 0
    let inBids = 0
    for (const b of batches) {
      if (b.tick_id !== tickId) continue
      if (['minted', 'bid_submitting', 'bid_placed', 'held'].includes(b.state)) {
        minted += b.minted_wbtc || 0
      }
      if (b.state === 'bid_placed') inBids += b.minted_wbtc || 0
    }
    return { minted, inBids, unplaced: minted - inBids }
  }

  /** minted -> bid_placed. Fuel check first; insufficient SPACE = held. */
  private async advanceMinted(batch: any, now: number): Promise<void> {
    const coin = this.coinFor(batch)
    if (!coin) return this.fail(batch, 'coin missing', now)
    const pair = this.pairFor(batch.tick_id)
    if (!pair) return this.fail(batch, 'bridge pair missing at bid time', now)

    // SPACE fuel gate — never fail half-way: hold and wait instead
    if (this.d.mvcChain) {
      const utxos = await this.d.mvcChain.getMvcUtxos(this.d.lpKeys.mvcFundingAddress)
      const fuel = utxos.reduce((a, u) => a + u.satoshis, 0)
      if (fuel < this.d.config.fuelThresholdSats) {
        const msg = `SPACE fuel ${fuel} below threshold ${this.d.config.fuelThresholdSats} — bid held`
        this.d.log.alert(msg, { batchId: batch.id, tick: coin.tick })
        this.d.db.recordAlert('fuel-hold', batch.tick_id, msg, now)
        this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'fuel' }, now)
        return
      }
    }

    const price = computeFloorPrice({
      amtPerMint: BigInt(coin.amtPerMint || '0'),
      decimals: Number(coin.decimals || '8'),
      liquidityPerMint: BigInt(coin.liquidityPerMint),
      haircutBps: this.d.config.floorHaircutBps,
    })
    const params = floorBidParams({
      fundingAddress: this.d.lpKeys.mvcFundingAddress,
      buyTokenCodeHash: pair.targetTokenCodeHash,
      buyTokenID: pair.targetTokenGenesis,
      sellAmountWbtc: BigInt(batch.minted_wbtc),
      priceNum: price.priceNum,
      priceDen: price.priceDen,
      saltHex: batchSaltHex(batch.id),
    })
    const expectation: BidExpectation = {
      fundingAddress: this.d.lpKeys.mvcFundingAddress,
      sellTokenCodeHash: WBTC_CODEHASH,
      sellTokenID: WBTC_TOKEN_ID,
      buyTokenCodeHash: pair.targetTokenCodeHash,
      buyTokenID: pair.targetTokenGenesis,
      sellAmount: BigInt(batch.minted_wbtc),
      priceNum: price.priceNum,
      priceDen: price.priceDen,
      minFillAmount: BigInt(params.minFillAmount),
    }

    if (!this.d.obBuilder || !this.d.mvcFundingPrivKey) {
      return this.fail(batch, 'orderbook builder or MVC key not configured', now)
    }

    // ---- accounting guards (before anything else) ----
    const acct = this.coinWbtcAccounting(batch.tick_id)
    if (Number(batch.minted_wbtc) > acct.minted - acct.inBids) {
      const msg = `accounting mismatch: batch sellAmount ${batch.minted_wbtc} exceeds coin unplaced WBTC ${acct.minted - acct.inBids} (minted ${acct.minted}, in bids ${acct.inBids})`
      this.d.log.alert(msg, { batchId: batch.id, tick: coin.tick })
      this.d.db.recordAlert('accounting-mismatch', batch.tick_id, msg, now)
      this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'accounting' }, now)
      return
    }
    if (this.d.wbtcBalance) {
      // global: funding must hold at least all minted-not-yet-placed WBTC (all coins)
      let globalUnplaced = 0
      for (const b of this.d.db.batchesByState()) {
        if (['minted', 'bid_submitting', 'held'].includes(b.state)) globalUnplaced += b.minted_wbtc || 0
      }
      const balance = await this.d.wbtcBalance()
      if (balance < BigInt(globalUnplaced)) {
        const msg = `accounting mismatch: funding WBTC balance ${balance} below global unplaced ${globalUnplaced} — possible double placement or loss`
        this.d.log.alert(msg, { batchId: batch.id, tick: coin.tick })
        this.d.db.recordAlert('accounting-mismatch', batch.tick_id, msg, now)
        this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'accounting' }, now)
        return
      }
    }

    const saltHex = batchSaltHex(batch.id)

    // ---- on-chain adoption (DB-restore path only): a salt match counts as
    // placed ONLY when the tokens are verifiably locked — a bare prep shell
    // (contract UTXO without locked WBTC) must never count.
    if (this.d.orderLookup && this.d.tokenOrderCodeHash && !this.d.config.dryRun) {
      let existing = null
      try {
        existing = await this.findExistingBid(batch, expectation, saltHex)
      } catch {
        // lookup failed: hold rather than risk double placement
        this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'lookup' }, now)
        return
      }
      if (existing) {
        let locked = false
        let verifyFailed = false
        if (this.d.verifyLockedTokens) {
          try {
            locked = await this.d.verifyLockedTokens(existing.scriptHex, existing.remainAmount)
          } catch {
            verifyFailed = true
          }
        }
        if (locked) {
          this.d.db.updateBatch(batch.id, {
            state: 'bid_placed',
            price_num: price.priceNum.toString(),
            price_den: price.priceDen.toString(),
            bid_outpoint: `${existing.txid}:${existing.vout}`,
          }, now)
          this.d.log.info('bid adopted from chain (salt + locked tokens verified) — marked placed, NOT re-placing', {
            batchId: batch.id, tick: coin.tick, outpoint: `${existing.txid}:${existing.vout}`,
          })
          return
        }
        // a salt-matching contract UTXO whose locked WBTC cannot be verified
        // (prep shell, unverifiable index, or lookup error) is ambiguous: the
        // order may be real. NEVER place fresh — hold for manual review.
        const outpoint = `${existing.txid}:${existing.vout}`
        const msg = `batch ${batch.id}: salt-matching order contract at ${outpoint} but locked WBTC unverified` +
          (verifyFailed ? ' (token lookup error)' : ' (no locked tokens — prep shell or filled?)') +
          ' — NOT placing, manual review required'
        this.d.log.alert(msg, { batchId: batch.id, outpoint })
        this.d.db.recordAlert('salt-match-unverified', batch.tick_id, msg, now)
        this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'salt-match-unverified' }, now)
        return
      }
    }

    if (this.d.config.dryRun) {
      // dry-run: compute price + store, but never build/sign/finalize
      this.d.log.info('DRY-RUN floor bid computed (not built)', {
        batchId: batch.id, tick: coin.tick,
        priceNum: price.priceNum.toString(), priceDen: price.priceDen.toString(),
        sellAmount: batch.minted_wbtc,
      })
      this.d.db.updateBatch(batch.id, {
        price_num: price.priceNum.toString(),
        price_den: price.priceDen.toString(),
      }, now)
      return
    }

    if (!this.d.mvcBroadcast) {
      return this.fail(batch, 'MVC broadcast not configured', now)
    }

    // build + validate + sign both steps WITHOUT broadcasting — nothing is on
    // chain yet, so a crash here simply restarts from 'minted' next tick.
    const placeFn = this.d.placeBid || placeFloorBid
    const signed = await placeFn({
      builder: this.d.obBuilder,
      privKey: this.d.mvcFundingPrivKey,
      params,
      expectation,
      contractFingerprints: this.d.contractFingerprints,
    })
    if (!signed.completed || !signed.prepHex || !signed.lockHex) {
      return this.fail(batch, 'bid draft did not complete', now)
    }

    // persist the fully signed transactions BEFORE the first broadcast — a
    // crash after this point resumes through advanceBidSubmitting, which
    // rebroadcasts THESE hexes and never builds a fresh draft.
    const summary = {
      saltHex,
      sellAmount: batch.minted_wbtc,
      prepHex: signed.prepHex,
      prepTxid: signed.prepTxid,
      lockHex: signed.lockHex,
      lockTxid: signed.lockTxid,
      orderVout: signed.orderVout,
    }
    this.d.db.updateBatch(batch.id, {
      state: 'bid_submitting',
      price_num: price.priceNum.toString(),
      price_den: price.priceDen.toString(),
      bid_txids: JSON.stringify(summary),
    }, now)

    await this.broadcastSignedBid(batch, summary, now)
  }

  /**
   * Broadcast the persisted signed prep + lock txs and mark the batch placed.
   * Shared by the initial placement and the crash-resume path. Throws are
   * classified: input-conflict -> held for manual review (never auto-rebuild);
   * already-known is tolerated (idempotent rebroadcast); anything else
   * propagates so the batch stays in bid_submitting and retries next tick.
   */
  private async broadcastSignedBid(batch: any, summary: any, now: number): Promise<void> {
    const outpoint = `${summary.prepTxid}:${summary.orderVout ?? 0}`
    for (const [label, hex] of [['prep', summary.prepHex], ['lock', summary.lockHex]] as Array<[string, string]>) {
      try {
        await this.d.mvcBroadcast!(hex)
      } catch (e: any) {
        const msg = String(e && e.message || e)
        if (/already.*(known|mempool)|txn-already|duplicate/i.test(msg)) {
          // idempotent rebroadcast — fine
        } else if (/inputs?[- ]spent|missing inputs|bad-txns-inputs|already spent|conflict/i.test(msg)) {
          const alertMsg = `batch ${batch.id}: ${label} tx rejected — inputs spent by another transaction; manual review required, NOT rebuilding`
          this.d.log.alert(alertMsg, { batchId: batch.id })
          this.d.db.recordAlert('bid-input-conflict', batch.tick_id, alertMsg, now)
          this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'input-conflict' }, now)
          return
        } else {
          throw e // transient — retry next tick from bid_submitting
        }
      }
    }
    this.d.db.updateBatch(batch.id, {
      state: 'bid_placed',
      bid_outpoint: outpoint,
    }, now)
    this.d.log.info('floor bid placed', {
      batchId: batch.id, prepTxid: summary.prepTxid, lockTxid: summary.lockTxid, outpoint,
    })

    // best-effort depth confirmation
    const pair = this.pairFor(batch.tick_id)
    if (this.d.depthCheck && pair) {
      try {
        const pairKey = `${pair.targetTokenCodeHash}:${pair.targetTokenGenesis}/${WBTC_CODEHASH}:${WBTC_TOKEN_ID}`
        const depth = await this.d.depthCheck(pairKey)
        if (!depth.bidSeen) {
          this.d.log.warn('bid not yet visible in public depth (indexing lag?)', { batchId: batch.id })
        }
      } catch (e: any) {
        this.d.log.warn('depth check failed (best-effort)', { error: e.message })
      }
    }
  }

  /**
   * bid_submitting resume — NEVER builds a fresh draft. The signed hexes were
   * persisted before the first broadcast, so placement is resumed exactly:
   *  - lock txid already on-chain (mempool or confirmed) -> bid_placed; this
   *    also covers a fully-filled order, which leaves no UTXO at all.
   *  - otherwise rebroadcast the persisted prep then lock (idempotent).
   *  - rejection because inputs were spent elsewhere -> held + alert, manual.
   */
  private async advanceBidSubmitting(batch: any, now: number): Promise<void> {
    if (this.d.config.dryRun) return
    let summary: any = null
    try {
      summary = JSON.parse(batch.bid_txids || '{}')
    } catch {
      summary = null
    }
    if (!summary || !summary.prepHex || !summary.lockHex || !summary.lockTxid) {
      const msg = `batch ${batch.id}: bid_submitting without persisted signed txs (legacy/corrupt row) — manual review, NOT rebuilding`
      this.d.log.alert(msg, { batchId: batch.id })
      this.d.db.recordAlert('bid-submitting-no-hexes', batch.tick_id, msg, now)
      this.d.db.updateBatch(batch.id, { state: 'held', hold_reason: 'manual-review' }, now)
      return
    }
    if (!this.d.mvcBroadcast) {
      return this.fail(batch, 'MVC broadcast not configured', now)
    }

    // authoritative check: is the lock tx on-chain already? covers both the
    // live-order case and the fully-filled case (no UTXO either way).
    if (this.d.mvcTxExists) {
      let onChain = false
      try {
        onChain = await this.d.mvcTxExists(summary.lockTxid)
      } catch {
        return // lookup down — retry next tick
      }
      if (onChain) {
        this.d.db.updateBatch(batch.id, {
          state: 'bid_placed',
          bid_outpoint: `${summary.prepTxid}:${summary.orderVout ?? 0}`,
        }, now)
        this.d.log.info('crash-resume: lock tx already on-chain — marked placed, zero rebroadcast', {
          batchId: batch.id, lockTxid: summary.lockTxid,
        })
        return
      }
    }

    await this.broadcastSignedBid(batch, summary, now)
  }

  /** held batches retry when their hold reason clears (called each tick) */
  async retryHeld(now = Date.now()): Promise<void> {
    for (const batch of this.d.db.batchesByState(['held'])) {
      if (batch.hold_reason === 'fuel' || batch.hold_reason === 'accounting' || batch.hold_reason === 'lookup') {
        this.d.db.updateBatch(batch.id, { state: 'minted', hold_reason: null }, now)
      }
    }
  }

  private fail(batch: any, reason: string, now: number): void {
    this.d.log.error('batch failed', { batchId: batch.id, reason })
    this.d.db.updateBatch(batch.id, { state: 'failed', fail_reason: reason }, now)
  }

  status(): Record<string, any> {
    const batches = this.d.db.batchesByState()
    const byState: Record<string, number> = {}
    let totalWbtcBid = 0
    for (const b of batches) {
      byState[b.state] = (byState[b.state] || 0) + 1
      if (b.state === 'bid_placed') totalWbtcBid += b.minted_wbtc || 0
    }
    // per-coin pending (confirmed, unbatched) vault sats + batch detail
    const perCoin: Record<string, any> = {}
    for (const c of this.d.db.listCoins()) {
      const pending = this.d.db.vaultUtxos(c.tickId)
        .filter((v: any) => v.confirmed === 1 && !this.d.db.isBatched(v.outpoint))
        .reduce((a: number, v: any) => a + v.sats, 0)
      const coinBatches = batches.filter((b) => b.tick_id === c.tickId)
      perCoin[c.tick] = {
        pendingVaultSats: pending,
        batches: coinBatches.length,
        bidsPlaced: coinBatches.filter((b) => b.state === 'bid_placed').length,
        wbtcInBids: coinBatches.filter((b) => b.state === 'bid_placed').reduce((a, b) => a + (b.minted_wbtc || 0), 0),
        holds: coinBatches.filter((b) => b.state === 'held').map((b) => b.hold_reason),
        bridgeable: !!this.pairFor(c.tickId),
      }
    }
    return {
      batchesByState: byState,
      bidsPlaced: byState['bid_placed'] || 0,
      totalWbtcInBids: totalWbtcBid,
      perCoin,
    }
  }
}
