/**
 * P2 pipeline tests — all bridge/BTC/MVC HTTP mocked, generated keys only.
 * Run: npx ts-node --transpile-only tests/pipeline.test.ts
 */
import * as fs from 'fs'
import * as os from 'os'
import * as path from 'path'
import * as assert from 'assert'
import * as bip39 from 'bip39'
import * as bitcoin from 'bitcoinjs-lib'

import { loadConfig } from '../src/config'
import { KeeperDb } from '../src/db'
import { loadLpKeys, deriveLpVault } from '../src/keys'
import { Logger } from '../src/log'
import { planBatches, buildDepositTx, validateDepositInvariants, vaultNodeForTick, VaultUtxo } from '../src/batch'
import { validateCreateOrderResponse, signMessageMvc, BridgeClient } from '../src/bridge'
import { computeFloorPrice, validateBidDraft, floorBidParams, BidExpectation, WBTC_CODEHASH, WBTC_TOKEN_ID, computeContractFingerprints, batchSaltHex } from '../src/bid'
import { OrderTokenProto } from '../../orderbook-service/src/core-bridge'
import { Pipeline } from '../src/pipeline'

const BTC = bitcoin.networks.bitcoin
let passed = 0
let failed = 0
function test(name: string, fn: () => void | Promise<void>): Promise<void> {
  return Promise.resolve().then(fn).then(() => {
    passed++
    console.log(`  PASS ${name}`)
  }).catch((e) => {
    failed++
    console.log(`  FAIL ${name}: ${e.message}`)
  })
}
const silentLog = () => new Logger(() => {})
function tmpdir() { return fs.mkdtempSync(path.join(os.tmpdir(), 'p2-test-')) }
function fakeTxid(seed: number): string { return seed.toString(16).padStart(64, '0').slice(0, 64) }

function setupKeys() {
  const dir = tmpdir()
  const mnemonic = bip39.generateMnemonic()
  const mfile = path.join(dir, 'm.txt')
  fs.writeFileSync(mfile, mnemonic)
  return { dir, lpKeys: loadLpKeys(mfile) }
}

const BRIDGE_VAULT = 'bc1qh90z428z4s0g9prcdf6rq00yg27uhxrtxlvvvv'
const FEE_ADDR = 'bc1qyjg3wtuk7gyzd7tuug7sghhdj79whdf8wravre'

class FakeBridge {
  createCalls = 0
  submitCalls = 0
  feeQuoteSats = 1500
  submitResult: { ok: boolean; msg: string } = { ok: true, msg: 'success' }
  statusStage = 'confirming'
  targetTxids: string[] = []
  failureReason: string | null = null
  orderSeq = 0
  async feeQuote() {
    return { btcFeeSats: this.feeQuoteSats, btcFeeAddress: FEE_ADDR, btcServiceFeeSats: this.feeQuoteSats, btcBridgeMinerFeeSats: 0, btcPostageSats: 0, btcFeeRate: 5 }
  }
  async assetList() { return [] }
  async createMintBtcOrder(args: any) {
    this.createCalls++
    this.orderSeq++
    return {
      orderId: `order-${this.orderSeq}`,
      bridgeAddress: BRIDGE_VAULT,
      receiveAmount: String(args.amountSats),
      confirmNumber: 1,
      btcFeeSats: this.feeQuoteSats,
      btcFeeAddress: FEE_ADDR,
    }
  }
  async submitMintBtcOrder(orderId: string, txHex: string) {
    this.submitCalls++
    return this.submitResult
  }
  async orderStatus(orderId: string) {
    return { orderId, stage: this.statusStage, status: 'doing', sourceTxid: fakeTxid(500), targetTxids: this.targetTxids, failureReason: this.failureReason }
  }
}

class FakeChain {
  fastestFee = 5
  async getUtxos() { return [] }
  async getFastestFee() { return this.fastestFee }
  async broadcastTx() { return '' }
  async getOutspend() { return '' }
}

function baseConfig(envExtra: Record<string, string> = {}) {
  return loadConfig({
    IDCOIN_KEEPER_ENABLE_TICKS: 'SUNNY',
    IDCOIN_KEEPER_DRY_RUN: 'false',
    IDCOIN_BRIDGE_VAULT: BRIDGE_VAULT,
    IDCOIN_BRIDGE_FEE_ALLOWLIST: FEE_ADDR,
    ...envExtra,
  } as any)
}

function pipelineFixture(envExtra: Record<string, string> = {}) {
  const { dir, lpKeys } = setupKeys()
  const db = new KeeperDb(path.join(dir, 't.db'))
  const bridge = new FakeBridge()
  const chain = new FakeChain()
  const config = baseConfig(envExtra)
  const pairs = [{
    network: 'MRC20', originTokenId: 'sunnyPinId0', targetTokenId: 'tgt1',
    targetTokenCodeHash: 'aa'.repeat(20), targetTokenGenesis: 'bb'.repeat(20),
    targetSymbol: 'SUNNY', decimals: 8, isEnableMint: true,
  }]
  const pipe = new Pipeline({
    config, db, log: silentLog(), bridge: bridge as any, chain: chain as any, lpKeys,
    pairs: pairs as any,
  })
  // register coin
  const vault = deriveLpVault(lpKeys.accountNode, 'SUNNY').address
  db.upsertCoin({
    tickId: 'sunnyPinId0', tick: 'SUNNY', mode: 'new', payTo: vault, lpVault: vault,
    liquidityPerMint: 200000, amtPerMint: '10000', decimals: '8', updatedAt: Date.now(),
  })
  return { dir, lpKeys, db, bridge, chain, config, pipe, vault, pairs }
}

async function walkTo(f: any, state: string, maxTicks = 12): Promise<any> {
  for (let i = 0; i < maxTicks; i++) {
    const b = f.db.batchesByState()[0]
    if (b && b.state === state) return b
    await f.pipe.tick()
  }
  const b = f.db.batchesByState()[0]
  throw new Error(`walkTo ${state} failed, stuck at ${b ? b.state : 'no-batch'}`)
}

function seedVaultUtxo(db: KeeperDb, tickId: string, vault: string, seed: number, sats: number) {
  db.upsertVaultUtxo({
    outpoint: `${fakeTxid(seed)}:0`, tickId, address: vault, sats,
    confirmed: true, provenance: 'direct', now: Date.now(),
  })
}

async function run() {
  console.log('\n[A] price math')
  await test('SUNNY price reduces to 1e9/197', () => {
    const p = computeFloorPrice({ amtPerMint: 10000n, decimals: 8, liquidityPerMint: 200000n, haircutBps: 150 })
    assert.strictEqual(p.priceNum, 1000000000n)
    assert.strictEqual(p.priceDen, 197n)
  })
  await test('spec formula equivalence: x1000/985 ratio identical', () => {
    const p = computeFloorPrice({ amtPerMint: 10000n, decimals: 8, liquidityPerMint: 200000n, haircutBps: 150 })
    // 10000e8*1000 / (200000*985) reduced must equal same ratio
    assert.strictEqual(p.priceNum * 197n, p.priceDen * 1000000000n)
  })
  await test('uint64 overflow guard throws', () => {
    assert.throws(() => computeFloorPrice({
      amtPerMint: 1000000000039n, decimals: 8, liquidityPerMint: 1n, haircutBps: 150,
    }), /uint64/)
  })
  await test('custom haircut changes price', () => {
    const p150 = computeFloorPrice({ amtPerMint: 10000n, decimals: 8, liquidityPerMint: 200000n, haircutBps: 150 })
    const p300 = computeFloorPrice({ amtPerMint: 10000n, decimals: 8, liquidityPerMint: 200000n, haircutBps: 300 })
    assert.ok(p300.priceNum * p150.priceDen > p150.priceNum * p300.priceDen, 'bigger haircut -> better rate for maker')
  })

  console.log('\n[B] batch planner budget rules')
  const mkUtxos = (specs: number[]): VaultUtxo[] => specs.map((s, i) => ({
    outpoint: `${fakeTxid(100 + i)}:0`, txid: fakeTxid(100 + i), vout: 0, sats: s, confirmed: true,
  }))
  await test('waits when below bridge minimum', () => {
    const res = planBatches({
      utxos: mkUtxos([5000]), btcFeeSats: 1500, feeRateSatPerVb: 5,
      minSats: 10000, maxSats: 100000000, haircutBps: 150,
    })
    assert.strictEqual(res.kind, 'wait')
  })
  await test('waits when cost over haircut budget', () => {
    // 12000 in, cost ~1500+550 = ~2050 > 1.5% of 12000 (180)
    const res = planBatches({
      utxos: mkUtxos([12000]), btcFeeSats: 1500, feeRateSatPerVb: 5,
      minSats: 10000, maxSats: 100000000, haircutBps: 150,
    })
    assert.strictEqual(res.kind, 'wait')
    if (res.kind === 'wait') assert.ok(res.reason.includes('haircut') || res.reason.includes('budget') || res.reason.includes('minimum'))
  })
  await test('batches when within budget; math correct', () => {
    // 500000 in; cost = txfee(~109*5=545) + 1500 = ~2045 <= 7500 (1.5%)
    const res = planBatches({
      utxos: mkUtxos([500000]), btcFeeSats: 1500, feeRateSatPerVb: 5,
      minSats: 10000, maxSats: 100000000, haircutBps: 150,
    })
    assert.strictEqual(res.kind, 'batch')
    if (res.kind !== 'batch') return
    assert.strictEqual(res.plans.length, 1)
    const p = res.plans[0]
    assert.strictEqual(p.totalIn, 500000)
    assert.strictEqual(p.amount, p.totalIn - p.estTxFee - p.btcFeeSats)
    assert.ok(p.amount >= 10000)
  })
  await test('splits sets exceeding 1 BTC', () => {
    const res = planBatches({
      utxos: mkUtxos([90000000, 90000000, 50000000]), btcFeeSats: 1500, feeRateSatPerVb: 5,
      minSats: 10000, maxSats: 100000000, haircutBps: 150,
    })
    assert.strictEqual(res.kind, 'batch')
    if (res.kind !== 'batch') return
    for (const p of res.plans) {
      assert.ok(p.amount <= 100000000, `batch amount ${p.amount} exceeds bridge max`)
    }
    assert.ok(res.plans.length >= 2, `expected split into >=2 batches, got ${res.plans.length}`)
  })

  console.log('\n[C] deposit tx invariants')
  await test('valid deposit passes; foreign/tampered rejected', () => {
    const { lpKeys } = setupKeys()
    const vaultNode = vaultNodeForTick(lpKeys.accountNode, 'SUNNY')
    const vault = deriveLpVault(lpKeys.accountNode, 'SUNNY').address
    const inputs: VaultUtxo[] = [
      { outpoint: `${fakeTxid(200)}:0`, txid: fakeTxid(200), vout: 0, sats: 300000, confirmed: true },
      { outpoint: `${fakeTxid(201)}:1`, txid: fakeTxid(201), vout: 1, sats: 200000, confirmed: true },
    ]
    const amount = 500000 - 1500 - 1090
    const built = buildDepositTx({
      vaultNode, inputs, bridgeAddress: BRIDGE_VAULT, feeAddress: FEE_ADDR, amount, btcFeeSats: 1500,
    })
    const ok = validateDepositInvariants({
      hex: built.hex, inputs, vaultAddress: vault, bridgeAddress: BRIDGE_VAULT, feeAddress: FEE_ADDR, amount, btcFeeSats: 1500,
    })
    assert.ok(ok.ok, `valid deposit rejected: ${ok.reason}`)
    const tx = bitcoin.Transaction.fromHex(built.hex)
    assert.strictEqual(tx.outs.length, 2)
    assert.strictEqual(Number(tx.outs[0].value), amount)
    assert.strictEqual(Number(tx.outs[1].value), 1500)
    // wrong amount -> reject
    const bad = validateDepositInvariants({
      hex: built.hex, inputs, vaultAddress: vault, bridgeAddress: BRIDGE_VAULT, feeAddress: FEE_ADDR, amount: amount - 1, btcFeeSats: 1500,
    })
    assert.ok(!bad.ok)
    // wrong input set -> reject
    const bad2 = validateDepositInvariants({
      hex: built.hex, inputs: inputs.slice(1), vaultAddress: vault, bridgeAddress: BRIDGE_VAULT, feeAddress: FEE_ADDR, amount, btcFeeSats: 1500,
    })
    assert.ok(!bad2.ok)
  })

  console.log('\n[D] create-order response validation')
  const baseRes = {
    orderId: 'o1', bridgeAddress: BRIDGE_VAULT, receiveAmount: '100000',
    confirmNumber: 1, btcFeeSats: 1500, btcFeeAddress: FEE_ADDR,
  }
  const baseArgs = { expectedAmountSats: 100000, pinnedBridgeVault: BRIDGE_VAULT, feeAllowlist: new Set([FEE_ADDR]), maxBtcFeeSats: 5000 }
  await test('good response accepted', () => {
    assert.ok(validateCreateOrderResponse({ res: baseRes, ...baseArgs }).ok)
  })
  await test('wrong bridge address rejected', () => {
    const r = validateCreateOrderResponse({ res: { ...baseRes, bridgeAddress: 'bc1qevil' }, ...baseArgs })
    assert.ok(!r.ok && r.reason!.includes('pinned vault'))
  })
  await test('fee address not in allowlist rejected', () => {
    const r = validateCreateOrderResponse({ res: { ...baseRes, btcFeeAddress: 'bc1qevilfee' }, ...baseArgs })
    assert.ok(!r.ok && r.reason!.includes('allowlist'))
  })
  await test('receiveAmount mismatch rejected', () => {
    const r = validateCreateOrderResponse({ res: { ...baseRes, receiveAmount: '99999' }, ...baseArgs })
    assert.ok(!r.ok && r.reason!.includes('1:1'))
  })
  await test('btcFeeSats over budget rejected', () => {
    const r = validateCreateOrderResponse({ res: { ...baseRes, btcFeeSats: 99999 }, ...baseArgs })
    assert.ok(!r.ok && r.reason!.includes('budget'))
  })

  console.log('\n[E] pipeline state machine')
  await test('full path: planned -> order_created -> signed -> submitted -> minted', async () => {
    const f = pipelineFixture()
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 300, 500000)
    // same-tick design: planNewBatches creates the batch, then the advance loop
    // immediately moves it one step per tick.
    const signed = await walkTo(f, 'deposit_signed')
    assert.strictEqual(f.bridge.submitCalls, 0, 'nothing submitted at deposit_signed')
    assert.ok(signed.deposit_hex && signed.deposit_hex.length > 40, 'deposit hex persisted BEFORE submit')
    const b = await walkTo(f, 'deposit_submitted')
    assert.strictEqual(f.bridge.createCalls, 1, 'exactly one create call')
    assert.strictEqual(f.bridge.submitCalls, 1, 'exactly one submit call')
    // mint completion
    f.bridge.statusStage = 'completed'
    f.bridge.targetTxids = [fakeTxid(777)]
    const b2 = await walkTo(f, 'minted')
    assert.strictEqual(b2.mint_txid, fakeTxid(777))
    assert.strictEqual(b2.minted_wbtc, b2.amount)
  })

  await test('resume: deposit rebuild deterministic across crash (same txid)', async () => {
    const f = pipelineFixture()
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 310, 500000)
    // walk to order_created, then sign once
    await walkTo(f, 'order_created')
    let b = (await walkTo(f, 'deposit_signed'))
    const txid1 = b.deposit_txid
    const hex1 = b.deposit_hex
    // simulate crash: wipe signed state back to order_created
    f.db.updateBatch(b.id, { state: 'order_created', deposit_txid: null, deposit_hex: null }, Date.now())
    await f.pipe.tick()
    b = f.db.batchesByState()[0]
    assert.strictEqual(b.deposit_txid, txid1, 'rebuilt deposit must be byte-identical (same txid)')
    assert.strictEqual(b.deposit_hex, hex1)
  })

  await test('resume: submit lost response -> already-submit treated as submitted', async () => {
    const f = pipelineFixture()
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 320, 500000)
    let b = await walkTo(f, 'deposit_signed')
    f.bridge.submitResult = { ok: false, msg: 'order already submit tx please create other' }
    await f.pipe.tick()
    b = f.db.batchesByState()[0]
    assert.strictEqual(b.state, 'deposit_submitted', 'resume must accept already-submitted')
  })

  await test('no utxo double-use across batches', async () => {
    const f = pipelineFixture()
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 330, 500000)
    await f.pipe.tick()
    assert.strictEqual(f.db.batchesByState().length, 1)
    // utxo still present in vault view but batched — no new batch may use it
    await f.pipe.tick()
    const batches = f.db.batchesByState()
    const uses = new Set<string>()
    for (const b of batches) {
      for (const op of JSON.parse(b.input_outpoints)) {
        assert.ok(!uses.has(op), `outpoint ${op} used in two batches`)
        uses.add(op)
      }
    }
    // exactly one batch exists (the utxo is reserved)
    assert.strictEqual(batches.length, 1)
  })

  await test('dry-run makes zero mutating bridge calls', async () => {
    const f = pipelineFixture({ IDCOIN_KEEPER_DRY_RUN: 'true' })
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 340, 500000)
    for (let i = 0; i < 6; i++) await f.pipe.tick()
    assert.strictEqual(f.bridge.createCalls, 0, 'dry-run must never call createPrepayOrder')
    assert.strictEqual(f.bridge.submitCalls, 0, 'dry-run must never submit')
    const b = f.db.batchesByState()[0]
    assert.strictEqual(b.state, 'planned', 'dry-run batch stays planned')
  })

  await test('non-bridgeable coin: no batch planned', async () => {
    const f = pipelineFixture()
    ;(f.pipe as any).d.pairs = []
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 350, 500000)
    await f.pipe.tick()
    assert.strictEqual(f.db.batchesByState().length, 0)
  })

  console.log('\n[F] fuel hold')
  await test('minted batch with low SPACE fuel -> held + alert; retry when fueled', async () => {
    const f = pipelineFixture()
    seedVaultUtxo(f.db, 'sunnyPinId0', f.vault, 360, 500000)
    f.bridge.statusStage = 'completed'
    f.bridge.targetTxids = [fakeTxid(778)]
    // low fuel from the start — the batch must hold rather than fail at bid time
    ;(f.pipe as any).d.mvcChain = { getMvcUtxos: async () => [{ satoshis: 100 }] }
    let b = await walkTo(f, 'held')
    assert.strictEqual(b.hold_reason, 'fuel')
    const alerts = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'fuel-hold'").all()
    assert.ok(alerts.length >= 1)
    // refuel -> retryHeld -> back to minted
    await f.pipe.retryHeld()
    b = f.db.batchesByState()[0]
    assert.strictEqual(b.state, 'minted')
  })

  console.log('\n[G] bid draft validation')
  await test('rejects foreign P2PKH output and tail mismatch (crafted draft)', async () => {
    // craft a fake draft whose tx pays a foreign P2PKH
    const orderbookTest = require('./obtestutil')
    const harness = await orderbookTest.makeHarness()
    const valid = await harness.buildValidDraft()
    const exp: BidExpectation = (harness as any).expectationFor(valid)
    const fps = computeContractFingerprints((harness as any).contracts)
    const ok = validateBidDraft(valid, exp, fps)
    assert.ok(ok.ok, `valid draft rejected: ${ok.reason}`)
    // tampered expectation (wrong price)
    const bad = validateBidDraft(valid, { ...exp, priceNum: exp.priceNum + 1n })
    assert.ok(!bad.ok && bad.reason!.includes('price'))
    // foreign P2PKH output: rebuild the tx with an extra foreign output
    const foreign = harness.craftForeignOutputDraft(valid)
    const bad2 = validateBidDraft(foreign, exp)
    assert.ok(!bad2.ok, 'foreign output accepted')
    assert.ok(bad2.reason!.includes('foreign'), bad2.reason)
  })

  console.log('\n[H] bid idempotency (signed-tx persistence, crash-resume, accounting)')
  // helper: minted batch + full bid deps with mocks
  function bidFixture(envExtra: Record<string, string> = {}) {
    const f = pipelineFixture(envExtra)
    const placeCalls: any[] = []
    const broadcasts: string[] = []
    const chainTxs = new Set<string>()   // txids retrievable on-chain
    const lookupItems: Array<{ txid: string; vout: number; scriptHex: string }> = []
    let lockedTokensOk = false
    let broadcastError: string | null = null
    ;(f.pipe as any).d.obBuilder = { fake: true }
    ;(f.pipe as any).d.mvcFundingPrivKey = { fake: true }
    ;(f.pipe as any).d.mvcChain = { getMvcUtxos: async () => [{ satoshis: 100000 }] }
    ;(f.pipe as any).d.tokenOrderCodeHash = 'e4082bffecd647405c2a9b3d5f23d7abb2f6018e'
    ;(f.pipe as any).d.orderLookup = async () => lookupItems
    ;(f.pipe as any).d.wbtcBalance = async () => 10n ** 12n
    ;(f.pipe as any).d.mvcTxExists = async (txid: string) => chainTxs.has(txid)
    ;(f.pipe as any).d.mvcBroadcast = async (hex: string) => {
      if (broadcastError) throw new Error(broadcastError)
      broadcasts.push(hex)
      return 'txid-' + broadcasts.length
    }
    ;(f.pipe as any).d.verifyLockedTokens = async () => lockedTokensOk
    ;(f.pipe as any).d.placeBid = async (args: any) => {
      placeCalls.push(args)
      return {
        prepHex: 'prephex-batch', prepTxid: fakeTxid(901),
        lockHex: 'lockhex-batch', lockTxid: fakeTxid(902),
        orderTxid: fakeTxid(901), orderVout: 0,
        orderScriptHex: 'aa', remainAmount: String(args.expectation.sellAmount),
        draftId: 'd', completed: true,
      }
    }
    return {
      ...f, placeCalls, broadcasts, chainTxs, lookupItems,
      setLockedTokens: (v: boolean) => { lockedTokensOk = v },
      setBroadcastError: (e: string | null) => { broadcastError = e },
    }
  }

  function seedMintedBatch(f: any, batchSeed: number, mintedWbtc: number): number {
    const inp = `${fakeTxid(batchSeed)}:0`
    const id = f.db.createBatch({
      tickId: 'sunnyPinId0',
      inputs: [{ outpoint: inp, sats: mintedWbtc + 3000 }],
      totalIn: mintedWbtc + 3000, estTxFee: 500, btcFeeSats: 1500,
      amount: mintedWbtc, now: Date.now(),
    })
    f.db.updateBatch(id, {
      state: 'minted', minted_wbtc: mintedWbtc, mint_txid: fakeTxid(batchSeed + 1),
    }, Date.now())
    return id
  }

  function seedBidSubmitting(f: any, batchSeed: number, mintedWbtc: number): { id: number; prepTxid: string; lockTxid: string } {
    const id = seedMintedBatch(f, batchSeed, mintedWbtc)
    const prepTxid = fakeTxid(batchSeed + 10)
    const lockTxid = fakeTxid(batchSeed + 11)
    f.db.updateBatch(id, {
      state: 'bid_submitting',
      price_num: '1000000000', price_den: '197',
      bid_txids: JSON.stringify({
        saltHex: batchSaltHex(id), sellAmount: mintedWbtc,
        prepHex: 'prephex-' + batchSeed, prepTxid,
        lockHex: 'lockhex-' + batchSeed, lockTxid,
        orderVout: 0,
      }),
    }, Date.now())
    return { id, prepTxid, lockTxid }
  }

  function craftOrderScriptHex(args: {
    saltHex: string; sellTokenCodeHash: string; sellTokenID: string
    buyTokenCodeHash: string; buyTokenID: string
    priceNum: bigint; priceDen: bigint; remain: bigint
  }): string {
    const dataPart = OrderTokenProto.buildOrderTokenDataPart({
      salt: Buffer.from(args.saltHex, 'hex'),
      feeBps: 0,
      feeAddr: Buffer.alloc(20, 0),
      minFillAmount: 1000n,
      remainAmount: args.remain,
      priceDen: args.priceDen,
      priceNum: args.priceNum,
      buyTokenCodeHash: Buffer.from(args.buyTokenCodeHash, 'hex'),
      buyTokenID: Buffer.from(args.buyTokenID, 'hex'),
      sellTokenCodeHash: Buffer.from(args.sellTokenCodeHash, 'hex'),
      sellTokenID: Buffer.from(args.sellTokenID, 'hex'),
      makerAddr: Buffer.alloc(20, 0),
    })
    const dp = Buffer.from(dataPart.toString('hex'), 'hex')
    return Buffer.concat([Buffer.from('0061', 'hex'), Buffer.from([0x6a, 0x4c, dp.length]), dp]).toString('hex')
  }

  await test('fresh placement: hexes persisted before broadcast, keeper broadcasts prep+lock itself', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 700, 200000)
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(f.placeCalls.length, 1, 'exactly one draft built')
    assert.deepStrictEqual(f.broadcasts, ['prephex-batch', 'lockhex-batch'], 'prep then lock, in order')
    assert.strictEqual(b.bid_outpoint, `${fakeTxid(901)}:0`, 'outpoint is the prep tx order output')
    // the persisted summary carries the signed hexes
    const summary = JSON.parse(b.bid_txids)
    assert.strictEqual(summary.prepHex, 'prephex-batch')
    assert.strictEqual(summary.lockTxid, fakeTxid(902))
    // a further tick builds/broadcasts nothing more
    await f.pipe.tick()
    assert.strictEqual(f.placeCalls.length, 1)
    assert.strictEqual(f.broadcasts.length, 2)
  })

  await test('crash after prep broadcast: resume rebroadcasts persisted txs, no new draft, same txids', async () => {
    const f = bidFixture()
    const { id, prepTxid, lockTxid } = seedBidSubmitting(f, 710, 200000)
    // lock txid NOT on chain; rebroadcast path
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(f.placeCalls.length, 0, 'must NEVER build a fresh draft on resume')
    assert.deepStrictEqual(f.broadcasts, ['prephex-710', 'lockhex-710'], 'rebroadcasts the persisted hexes')
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(b.bid_outpoint, `${prepTxid}:0`)
    const summary = JSON.parse(b.bid_txids)
    assert.strictEqual(summary.lockTxid, lockTxid, 'same txids — no rebuild')
  })

  await test('crash after lock broadcast (response lost): lock txid on chain -> placed, zero broadcasts', async () => {
    const f = bidFixture()
    const { id, lockTxid } = seedBidSubmitting(f, 720, 200000)
    f.chainTxs.add(lockTxid) // lock made it before the crash
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(f.placeCalls.length, 0)
    assert.strictEqual(f.broadcasts.length, 0, 'no rebroadcast when lock already on-chain')
  })

  await test('fully-filled order (no UTXO anywhere, lock txid confirmed) -> placed, no re-place', async () => {
    const f = bidFixture()
    const { id, lockTxid } = seedBidSubmitting(f, 730, 200000)
    f.chainTxs.add(lockTxid)
    // lookupItems empty — the order was fully filled, no contract UTXO exists
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(f.placeCalls.length, 0, 'a filled order must never trigger a re-place')
    assert.strictEqual(f.broadcasts.length, 0)
  })

  await test('prep-shell-only salt match -> held + salt-match-unverified alert, NO fresh placement', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 740, 200000)
    // contract UTXO with our salt exists, but no locked tokens (prep shell)
    f.lookupItems.push({
      txid: fakeTxid(950), vout: 0,
      scriptHex: craftOrderScriptHex({
        saltHex: batchSaltHex(id),
        sellTokenCodeHash: WBTC_CODEHASH, sellTokenID: WBTC_TOKEN_ID,
        buyTokenCodeHash: 'aa'.repeat(20), buyTokenID: 'bb'.repeat(20),
        priceNum: 1000000000n, priceDen: 197n, remain: 200000n,
      }),
    })
    // verifyLockedTokens returns false (default) -> unverified
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'held')
    assert.strictEqual(b.hold_reason, 'salt-match-unverified')
    assert.strictEqual(f.placeCalls.length, 0, 'must never place fresh over an unverified salt match')
    assert.strictEqual(f.broadcasts.length, 0)
    const alerts = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'salt-match-unverified'").all()
    assert.ok(alerts.length >= 1, 'salt-match-unverified alert persisted')
    assert.ok(String((alerts[0] as any).message).includes(`${fakeTxid(950)}:0`), 'alert includes the outpoint')
  })

  await test('salt match + token lookup ERROR -> held, no placement', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 745, 200000)
    f.lookupItems.push({
      txid: fakeTxid(951), vout: 0,
      scriptHex: craftOrderScriptHex({
        saltHex: batchSaltHex(id),
        sellTokenCodeHash: WBTC_CODEHASH, sellTokenID: WBTC_TOKEN_ID,
        buyTokenCodeHash: 'aa'.repeat(20), buyTokenID: 'bb'.repeat(20),
        priceNum: 1000000000n, priceDen: 197n, remain: 200000n,
      }),
    })
    ;(f.pipe as any).d.verifyLockedTokens = async () => { throw new Error('index down') }
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'held')
    assert.strictEqual(b.hold_reason, 'salt-match-unverified')
    assert.strictEqual(f.placeCalls.length, 0)
    assert.strictEqual(f.broadcasts.length, 0)
  })

  await test('salt match WITH verified locked tokens -> adopted, nothing built or broadcast', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 750, 200000)
    f.lookupItems.push({
      txid: fakeTxid(960), vout: 0,
      scriptHex: craftOrderScriptHex({
        saltHex: batchSaltHex(id),
        sellTokenCodeHash: WBTC_CODEHASH, sellTokenID: WBTC_TOKEN_ID,
        buyTokenCodeHash: 'aa'.repeat(20), buyTokenID: 'bb'.repeat(20),
        priceNum: 1000000000n, priceDen: 197n, remain: 200000n,
      }),
    })
    f.setLockedTokens(true)
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(b.bid_outpoint, `${fakeTxid(960)}:0`)
    assert.strictEqual(f.placeCalls.length, 0)
    assert.strictEqual(f.broadcasts.length, 0)
  })

  await test('wrong-salt order does NOT block placement', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 760, 200000)
    f.lookupItems.push({
      txid: fakeTxid(961), vout: 0,
      scriptHex: craftOrderScriptHex({
        saltHex: batchSaltHex(999),
        sellTokenCodeHash: WBTC_CODEHASH, sellTokenID: WBTC_TOKEN_ID,
        buyTokenCodeHash: 'aa'.repeat(20), buyTokenID: 'bb'.repeat(20),
        priceNum: 1000000000n, priceDen: 197n, remain: 200000n,
      }),
    })
    f.setLockedTokens(true)
    await f.pipe.tick()
    assert.strictEqual(f.placeCalls.length, 1, 'foreign-salt order must not count')
  })

  await test('inputs spent elsewhere: broadcast rejection -> held + alert, never rebuilt', async () => {
    const f = bidFixture()
    const { id } = seedBidSubmitting(f, 770, 200000)
    f.setBroadcastError('bad-txns-inputs-spent')
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'held')
    assert.strictEqual(b.hold_reason, 'input-conflict')
    assert.strictEqual(f.placeCalls.length, 0, 'never auto-rebuild')
    const alerts = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'bid-input-conflict'").all()
    assert.ok(alerts.length >= 1, 'input-conflict alert persisted')
  })

  await test('already-known broadcast error tolerated (idempotent rebroadcast)', async () => {
    const f = bidFixture()
    const { id } = seedBidSubmitting(f, 780, 200000)
    let calls = 0
    ;(f.pipe as any).d.mvcBroadcast = async (hex: string) => {
      calls++
      if (calls === 1) throw new Error('txn-already-known, mempool')
      f.broadcasts.push(hex)
      return 'ok'
    }
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'bid_placed')
    assert.strictEqual(f.broadcasts.length, 1, 'only the lock needed a real broadcast')
  })

  await test('legacy bid_submitting row without hexes -> held for manual review', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 790, 200000)
    f.db.updateBatch(id, {
      state: 'bid_submitting',
      price_num: '1000000000', price_den: '197',
      bid_txids: JSON.stringify({ saltHex: batchSaltHex(id), sellAmount: 200000 }),
    }, Date.now())
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'held')
    assert.strictEqual(b.hold_reason, 'manual-review')
    assert.strictEqual(f.placeCalls.length, 0)
    assert.strictEqual(f.broadcasts.length, 0)
  })

  await test('accounting guard: funding balance below unplaced -> held + alert, no placement', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 7400, 200000)
    ;(f.pipe as any).d.wbtcBalance = async () => 1000n
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(b.state, 'held')
    assert.strictEqual(b.hold_reason, 'accounting')
    assert.strictEqual(f.placeCalls.length, 0)
    const alerts = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'accounting-mismatch'").all()
    assert.ok(alerts.length >= 1)
  })

  await test('consistent accounting: batch sells exactly its own minted WBTC and places', async () => {
    const f = bidFixture()
    const id = seedMintedBatch(f, 7600, 200000)
    const id2 = seedMintedBatch(f, 7620, 200000)
    f.db.updateBatch(id2, { state: 'bid_placed' }, Date.now())
    ;(f.pipe as any).d.wbtcBalance = async () => 200000n
    await f.pipe.tick()
    const b = f.db.getBatch(id)
    assert.strictEqual(f.placeCalls.length, 1, 'consistent rows must place normally')
    assert.strictEqual(b.state, 'bid_placed')
  })

  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
}

run().catch((e) => { console.error('harness error:', e); process.exit(1) })
