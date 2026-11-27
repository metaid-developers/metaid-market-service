/**
 * idcoin-keeper test suite. All HTTP is mocked (fake ChainClient); keys are
 * generated test material only. Run: npm test
 */
import * as fs from 'fs'
import * as os from 'os'
import * as path from 'path'
import * as assert from 'assert'
import * as bip39 from 'bip39'
import { BIP32Factory } from 'bip32'
import * as ecc from '@bitcoin-js/tiny-secp256k1-asmjs'
import * as bitcoin from 'bitcoinjs-lib'
import { ECPairFactory } from 'ecpair'

import { loadConfig } from '../src/config'
import {
  loadLpKeys,
  loadLegacyKeys,
  deriveLpVault,
  deriveLpVaultFromXpub,
  LP_VAULT_ACCOUNT_PATH,
  MVC_LP_FUNDING_PATH,
} from '../src/keys'
import { KeeperDb } from '../src/db'
import { ChainUtxo } from '../src/chain'
import { refreshCoinRegistry, CoinListItem } from '../src/registry'
import { buildSweepTx, eligibleUtxos, validateSweepInvariants, RBF_SEQUENCE } from '../src/sweep'
import { Keeper } from '../src/keeper'
import { Logger, redactSecrets } from '../src/log'
import { startOpsServer } from '../src/ops'

const bip32 = BIP32Factory(ecc)
const ECPair = ECPairFactory(ecc)
const BTC = bitcoin.networks.bitcoin

let passed = 0
let failed = 0
function test(name: string, fn: () => void | Promise<void>): Promise<void> {
  return Promise.resolve()
    .then(fn)
    .then(() => {
      passed++
      console.log(`  PASS ${name}`)
    })
    .catch((e) => {
      failed++
      console.log(`  FAIL ${name}: ${e.message}`)
    })
}

function tmpdir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'idcoin-keeper-test-'))
}

function fakeTxid(seed: number): string {
  return seed.toString(16).padStart(64, '0').slice(0, 64)
}

function makeLegacyFileEntries(count: number) {
  const coins: Record<string, any> = {}
  const kps: any[] = []
  for (let i = 0; i < count; i++) {
    const kp = ECPair.makeRandom({ network: BTC })
    const address = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(kp.publicKey), network: BTC }).address!
    coins[`fakepin${i}i0`] = { tick: `TEST${i}`, address, source: 'test', wif: kp.toWIF() }
    kps.push(kp)
  }
  return { coins, kps }
}

function writeLegacyFile(dir: string, coins: Record<string, any>): string {
  const file = path.join(dir, 'legacy.json')
  fs.writeFileSync(
    file,
    JSON.stringify({ version: 1, generatedAt: '2026-01-01', note: 'test', coins })
  )
  return file
}

function writeMnemonicFile(dir: string, mnemonic: string): string {
  const file = path.join(dir, 'mnemonic.txt')
  fs.writeFileSync(file, mnemonic + '\n')
  return file
}

/** fully in-memory ChainClient test double */
class FakeChain {
  utxosByAddress = new Map<string, ChainUtxo[]>()
  fastestFee = 5
  broadcasts: string[] = []
  outspends = new Map<string, string>() // outpoint -> spending txid ('' = unspent)

  async getUtxos(address: string): Promise<ChainUtxo[]> {
    return this.utxosByAddress.get(address) || []
  }
  async getFastestFee(): Promise<number> {
    return this.fastestFee
  }
  async broadcastTx(hex: string): Promise<string> {
    this.broadcasts.push(hex)
    return bitcoin.Transaction.fromHex(hex).getId()
  }
  async getOutspend(txid: string, vout: number): Promise<string> {
    return this.outspends.get(`${txid}:${vout}`) || ''
  }
}

const silentLog = () => new Logger(() => {})

async function run() {
  // -----------------------------------------------------------------------
  console.log('\n[1] derivation')
  // account xpub from the public derivation facts file
  const XPUB = 'xpub6DDLXRDx6k34rNYcujmKp1xb92YSU4uAkEs1rhT6C3ytFbjU1YMaSt4gLQoB9cK9TTMRNrVVEZogsbPVxNQwui5TjKk7BMZrBwdBJdYktdg'
  await test('xpub SUNNY -> known vault address', () => {
    assert.strictEqual(deriveLpVaultFromXpub(XPUB, 'SUNNY'), 'bc1qg0lt0nc4uja6fyhp8f0n2lfupd3hxld3vfeayf')
  })
  await test('xpub METAID -> known vault address', () => {
    assert.strictEqual(deriveLpVaultFromXpub(XPUB, 'METAID'), 'bc1qlz9dfj0d3p8jl8870sdyhlnryhc9e7t37d3djd')
  })
  await test('xpub lowercase tick uppercased to same address', () => {
    assert.strictEqual(deriveLpVaultFromXpub(XPUB, 'sunny'), 'bc1qg0lt0nc4uja6fyhp8f0n2lfupd3hxld3vfeayf')
  })
  await test('private derivation equals xpub derivation (test mnemonic)', () => {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const file = writeMnemonicFile(dir, mnemonic)
    const keys = loadLpKeys(file)
    const xpub = keys.accountNode.neutered().toBase58()
    for (const tick of ['AAA', 'SUNNY', 'XY9Z']) {
      assert.strictEqual(deriveLpVaultFromXpub(xpub, tick), deriveLpVault(keys.accountNode, tick).address)
    }
  })
  await test('MVC funding path derives a P2PKH 1-prefix address', () => {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const keys = loadLpKeys(writeMnemonicFile(dir, mnemonic))
    assert.ok(keys.mvcFundingAddress.startsWith('1'), `expected 1-prefix, got ${keys.mvcFundingAddress}`)
    // verify path actually used: recompute independently
    const seed = bip39.mnemonicToSeedSync(mnemonic)
    const node = bip32.fromSeed(seed, BTC).derivePath(MVC_LP_FUNDING_PATH)
    const expect = bitcoin.payments.p2pkh({ pubkey: Buffer.from(node.publicKey), network: BTC }).address!
    assert.strictEqual(keys.mvcFundingAddress, expect)
  })
  await test('invalid mnemonic fails startup load', () => {
    const dir = tmpdir()
    const file = writeMnemonicFile(dir, 'not a valid mnemonic at all really truly honestly yes no')
    assert.throws(() => loadLpKeys(file), /invalid BIP39 mnemonic/)
  })

  // -----------------------------------------------------------------------
  console.log('\n[2] legacy keys file validation')
  await test('good file loads, WIF re-derivation passes', () => {
    const dir = tmpdir()
    const { coins } = makeLegacyFileEntries(3)
    const file = writeLegacyFile(dir, coins)
    const map = loadLegacyKeys(file)
    assert.strictEqual(map.size, 3)
    for (const [tickId, entry] of map) {
      assert.strictEqual(entry.address, coins[tickId].address)
    }
  })
  await test('tampered address -> startup fails', () => {
    const dir = tmpdir()
    const { coins } = makeLegacyFileEntries(1)
    const other = ECPair.makeRandom({ network: BTC })
    coins[Object.keys(coins)[0]].address = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(other.publicKey), network: BTC }).address!
    const file = writeLegacyFile(dir, coins)
    assert.throws(() => loadLegacyKeys(file), /does not match address/)
  })
  await test('tampered/invalid WIF -> startup fails', () => {
    const dir = tmpdir()
    const { coins } = makeLegacyFileEntries(1)
    coins[Object.keys(coins)[0]].wif = 'notawif'
    const file = writeLegacyFile(dir, coins)
    assert.throws(() => loadLegacyKeys(file), /invalid WIF/)
  })

  // -----------------------------------------------------------------------
  console.log('\n[3] registry classification')
  await test('classification: legacy / new / unknown against injected address maps', async () => {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const keys = loadLpKeys(writeMnemonicFile(dir, mnemonic))
    const db = new KeeperDb(path.join(dir, 'test.db'))
    const log = silentLog()

    // build a fake legacy file with entries for 3 coins
    const { coins: legacyEntries } = makeLegacyFileEntries(0)
    const mk = (tickId: string, tick: string) => {
      const kp = ECPair.makeRandom({ network: BTC })
      const address = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(kp.publicKey), network: BTC }).address!
      legacyEntries[tickId] = { tick, address, source: 'test', wif: kp.toWIF() }
      return address
    }
    const addrA = mk('pinAi0', 'ALPH')
    const addrB = mk('pinBi0', 'BETA')
    const legacyFile = writeLegacyFile(dir, legacyEntries)
    const legacyKeys = loadLegacyKeys(legacyFile)

    const vaultB = deriveLpVault(keys.accountNode, 'BETA').address
    const items: CoinListItem[] = [
      { tickId: 'pinAi0', tick: 'ALPH', decimals: '8', amtPerMint: '1000', liquidityPerMint: 10000, payCheck: { payTo: addrA } },
      // BETA's payTo is the LP vault -> 'new'
      { tickId: 'pinBi0', tick: 'BETA', decimals: '8', amtPerMint: '500', liquidityPerMint: 20000, payCheck: { payTo: vaultB } },
      // GAMA unknown
      { tickId: 'pinCi0', tick: 'GAMA', decimals: '8', amtPerMint: '100', liquidityPerMint: 30000, payCheck: { payTo: addrB } },
    ]
    const res = await refreshCoinRegistry({
      coinsUrl: 'unused',
      accountNode: keys.accountNode,
      legacyKeys,
      db,
      log,
      items,
    })
    assert.strictEqual(res.legacy, 1)
    assert.strictEqual(res.fresh, 1)
    assert.strictEqual(res.unknown, 1)
    const rows = db.listCoins()
    assert.strictEqual(rows.length, 3)
    const alph = rows.find((r) => r.tick === 'ALPH')!
    assert.strictEqual(alph.mode, 'legacy')
    assert.strictEqual(alph.payTo, addrA)
    const beta = rows.find((r) => r.tick === 'BETA')!
    assert.strictEqual(beta.mode, 'new')
    assert.strictEqual(beta.payTo, vaultB)
    assert.strictEqual(beta.lpVault, vaultB)
    assert.strictEqual(rows.find((r) => r.tick === 'GAMA')!.mode, 'unknown')
    db.close()
  })

  await test('golden coins-list sample: 32 entries all classify with matching injected legacy file', async () => {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const keys = loadLpKeys(writeMnemonicFile(dir, mnemonic))
    const db = new KeeperDb(path.join(dir, 'test.db'))
    const log = silentLog()

    const goldenPath = '/Users/tusm/Documents/MetaID_Projects/_tmp/idcoins-golden-20261128/coins-list_c0_s50.json'
    let items: CoinListItem[]
    if (fs.existsSync(goldenPath)) {
      const golden = JSON.parse(fs.readFileSync(goldenPath, 'utf8'))
      items = golden.data.list
    } else {
      // synthesize equivalent shape if golden file absent
      items = []
    }
    assert.ok(items.length >= 1, 'golden sample empty')

    // the golden payTos are real production addresses we can't sign for; per
    // the brief we test classification against injected address maps: build a
    // legacy file whose addresses EQUAL the golden payTos but with test WIFs
    // — impossible (WIF must re-derive to address). Instead, verify the
    // classifier logic on the golden shape: coins whose payTo is in the legacy
    // file classify legacy. We rewrite each item's payTo to a test-controlled
    // address and register it in the legacy file — preserving count/tick shape.
    const legacyEntries: Record<string, any> = {}
    const mapped: CoinListItem[] = items.map((it) => {
      const kp = ECPair.makeRandom({ network: BTC })
      const address = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(kp.publicKey), network: BTC }).address!
      legacyEntries[it.tickId] = { tick: it.tick, address, source: 'test', wif: kp.toWIF() }
      return { ...it, payCheck: { ...(typeof it.payCheck === 'object' ? it.payCheck : {}), payTo: address } }
    })
    const legacyFile = writeLegacyFile(dir, legacyEntries)
    const legacyKeys = loadLegacyKeys(legacyFile)

    const res = await refreshCoinRegistry({
      coinsUrl: 'unused',
      accountNode: keys.accountNode,
      legacyKeys,
      db,
      log,
      items: mapped,
    })
    assert.strictEqual(res.total, items.length)
    assert.strictEqual(res.legacy, items.length, `expected all ${items.length} legacy, got ${res.legacy}`)
    assert.strictEqual(res.unknown, 0)
    db.close()
  })

  // -----------------------------------------------------------------------
  console.log('\n[4] sweep build/sign/invariants')
  async function sweepFixture() {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const keys = loadLpKeys(writeMnemonicFile(dir, mnemonic))
    const kp = ECPair.makeRandom({ network: BTC })
    const address = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(kp.publicKey), network: BTC }).address!
    const legacyKey = {
      tick: 'SWPT',
      address,
      source: 'test',
      wif: kp.toWIF(),
      keyPair: kp,
    }
    const utxos: ChainUtxo[] = [
      { txid: fakeTxid(1), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
      { txid: fakeTxid(2), vout: 1, sats: 20000, confirmed: false, inscriptionBearing: false },
    ]
    return { dir, keys, kp, legacyKey, utxos, address }
  }

  await test('build: single vault output, RBF, correct fee, invariants pass', async () => {
    const { keys, legacyKey, utxos } = await sweepFixture()
    const outcome = buildSweepTx({
      tickId: 'pinXi0',
      tick: 'SWPT',
      legacyKey,
      accountNode: keys.accountNode,
      utxos,
      feeRateSatPerVb: 10,
      maxFeeRatio: 0.3,
    })
    assert.ok('built' in outcome && outcome.built, `unexpected skip: ${(outcome as any).reason}`)
    if (!('built' in outcome)) return
    const tx = bitcoin.Transaction.fromHex(outcome.hex)
    assert.strictEqual(tx.outs.length, 1)
    const vault = deriveLpVault(keys.accountNode, 'SWPT')
    const vaultScript = bitcoin.address.toOutputScript(vault.address, BTC)
    assert.ok(tx.outs[0].script.equals(vaultScript))
    assert.strictEqual(Number(tx.outs[0].value), outcome.totalIn - outcome.fee)
    assert.strictEqual(outcome.totalIn, 30000)
    // fee == rate x estVsize within rounding
    assert.ok(outcome.fee >= 10 * (outcome.vsize - 5) && outcome.fee <= 10 * (outcome.vsize + 20), `fee ${outcome.fee} vs vsize ${outcome.vsize}`)
    for (const inp of tx.ins) assert.strictEqual(inp.sequence, RBF_SEQUENCE)
    assert.strictEqual(validateSweepInvariants({
      hex: outcome.hex,
      inputOutpoints: outcome.inputOutpoints,
      vaultAddress: vault.address,
      totalIn: outcome.totalIn,
      fee: outcome.fee,
      outputSats: outcome.outputSats,
    }).ok, true)
  })

  await test('eligibility: inscription-bearing and <1000 sats excluded', async () => {
    const utxos: ChainUtxo[] = [
      { txid: fakeTxid(1), vout: 0, sats: 5000, confirmed: true, inscriptionBearing: false },
      { txid: fakeTxid(2), vout: 0, sats: 999, confirmed: true, inscriptionBearing: false },
      { txid: fakeTxid(3), vout: 0, sats: 50000, confirmed: true, inscriptionBearing: true },
    ]
    const el = eligibleUtxos(utxos, 1000)
    assert.strictEqual(el.length, 1)
    assert.strictEqual(el[0].txid, fakeTxid(1))
  })

  await test('dust skip: output below 546 not swept', async () => {
    const { keys, legacyKey } = await sweepFixture()
    const tiny: ChainUtxo[] = [{ txid: fakeTxid(9), vout: 0, sats: 1000, confirmed: true, inscriptionBearing: false }]
    const outcome = buildSweepTx({
      tickId: 'pinXi0', tick: 'SWPT', legacyKey, accountNode: keys.accountNode,
      utxos: tiny, feeRateSatPerVb: 10, maxFeeRatio: 0.9,
    })
    assert.ok('skipped' in outcome && outcome.skipped)
    assert.ok((outcome as any).reason.includes('dust') || (outcome as any).reason.includes('fee ratio'))
  })

  await test('fee-ratio cap: huge fee rate -> skip with cap reason', async () => {
    const { keys, legacyKey, utxos } = await sweepFixture()
    const outcome = buildSweepTx({
      tickId: 'pinXi0', tick: 'SWPT', legacyKey, accountNode: keys.accountNode,
      utxos, feeRateSatPerVb: 5000, maxFeeRatio: 0.3,
    })
    assert.ok('skipped' in outcome && outcome.skipped)
    assert.ok((outcome as any).reason.includes('fee ratio'))
  })

  await test('invariant validation rejects tampered tx', async () => {
    const { keys, legacyKey, utxos } = await sweepFixture()
    const outcome = buildSweepTx({
      tickId: 'pinXi0', tick: 'SWPT', legacyKey, accountNode: keys.accountNode,
      utxos, feeRateSatPerVb: 10, maxFeeRatio: 0.3,
    })
    assert.ok('built' in outcome)
    if (!('built' in outcome)) return
    const vault = deriveLpVault(keys.accountNode, 'SWPT')
    // wrong vault address
    const res = validateSweepInvariants({
      hex: outcome.hex,
      inputOutpoints: outcome.inputOutpoints,
      vaultAddress: deriveLpVault(keys.accountNode, 'OTHER').address,
      totalIn: outcome.totalIn,
      fee: outcome.fee,
      outputSats: outcome.outputSats,
    })
    assert.strictEqual(res.ok, false)
    assert.ok(res.reason!.includes('LP vault'))
  })

  // -----------------------------------------------------------------------
  console.log('\n[5] keeper integration: dry-run, idempotency, compromise')
  async function keeperFixture(opts?: { dryRun?: boolean }) {
    const dir = tmpdir()
    const mnemonic = bip39.generateMnemonic()
    const keys = loadLpKeys(writeMnemonicFile(dir, mnemonic))
    const kp = ECPair.makeRandom({ network: BTC })
    const payTo = bitcoin.payments.p2wpkh({ pubkey: Buffer.from(kp.publicKey), network: BTC }).address!
    const legacyEntries: Record<string, any> = {
      pinKi0: { tick: 'KEEP', address: payTo, source: 'test', wif: kp.toWIF() },
    }
    const legacyFile = writeLegacyFile(dir, legacyEntries)
    const legacyKeys = loadLegacyKeys(legacyFile)
    const db = new KeeperDb(path.join(dir, 'test.db'))
    const log = silentLog()
    const chain = new FakeChain()
    const vault = deriveLpVault(keys.accountNode, 'KEEP').address

    const config = loadConfig({
      IDCOIN_KEEPER_DRY_RUN: opts?.dryRun === false ? 'false' : 'true',
      IDCOIN_KEEPER_ENABLE_TICKS: 'KEEP',
      IDCOIN_KEEPER_FEE_FLOOR: '2',
      IDCOIN_KEEPER_MAX_FEE_RATIO: '0.3',
    } as any)

    const keeper = new Keeper({ config, db, chain: chain as any, log, lpKeys: keys, legacyKeys })
    // seed registry row directly
    db.upsertCoin({
      tickId: 'pinKi0', tick: 'KEEP', mode: 'legacy', payTo, lpVault: vault,
      liquidityPerMint: 10000, amtPerMint: '1000', decimals: '8', updatedAt: Date.now(),
    })
    return { dir, keys, kp, payTo, db, log, chain, vault, keeper, config }
  }

  await test('dry-run builds/signs/records but never broadcasts', async () => {
    const { chain, keeper, payTo } = await keeperFixture({ dryRun: true })
    chain.utxosByAddress.set(payTo, [
      { txid: fakeTxid(11), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    const reports = await keeper.sweepTick()
    const rep = reports[0]
    assert.strictEqual(rep.action, 'swept')
    assert.strictEqual(rep.broadcast, false)
    assert.strictEqual(chain.broadcasts.length, 0, 'dry-run must never broadcast')
  })

  await test('idempotency: same utxo never swept twice; restart-safe', async () => {
    const f = await keeperFixture({ dryRun: false })
    const { chain, payTo, db } = f
    chain.utxosByAddress.set(payTo, [
      { txid: fakeTxid(12), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    let reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept')
    assert.strictEqual(chain.broadcasts.length, 1)
    // second pass — same utxo still present (unconfirmed), must NOT sweep again
    reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'none')
    assert.strictEqual(chain.broadcasts.length, 1)
    // restart: brand-new Keeper over the same DB file
    const keeper2 = new Keeper({
      config: f.config, db, chain: chain as any, log: silentLog(), lpKeys: f.keys, legacyKeys: f.keeper.legacyKeys,
    })
    reports = await keeper2.sweepTick()
    assert.strictEqual(reports[0].action, 'none')
    assert.strictEqual(chain.broadcasts.length, 1)
  })

  await test('kill switch blocks all sweeping', async () => {
    const f = await keeperFixture({ dryRun: false })
    f.config.killSwitch = true
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(13), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    const reports = await f.keeper.sweepTick()
    assert.strictEqual(reports.length, 0)
    assert.strictEqual(f.chain.broadcasts.length, 0)
  })

  await test('disabled tick (not in allowlist) never swept', async () => {
    const f = await keeperFixture({ dryRun: false })
    f.config.enableTicks = new Set()
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(14), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    const reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'disabled')
    assert.strictEqual(f.chain.broadcasts.length, 0)
  })

  await test('double-spend detection: swept input spent by foreign tx -> alert persisted', async () => {
    const f = await keeperFixture({ dryRun: false })
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(15), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    await f.keeper.sweepTick()
    const ourTxid = bitcoin.Transaction.fromHex(f.chain.broadcasts[0]).getId()
    // attacker spends the same input with a different tx
    f.chain.outspends.set(`${fakeTxid(15)}:0`, fakeTxid(99))
    assert.notStrictEqual(ourTxid, fakeTxid(99))
    const before = (f.db.db.prepare('SELECT COUNT(*) AS n FROM alerts').get() as any).n
    await f.keeper.detectCompromise(f.db.listCoins()[0])
    const rows = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'compromise-double-spend'").all()
    assert.ok(rows.length >= 1, 'expected compromise alert persisted')
    const alerts = f.keeper.log.lines.filter((l) => l.includes('compromise'))
    assert.ok(f.db.counts().alerts > before)
  })

  await test('vanished unswept utxo -> compromise alert', async () => {
    const f = await keeperFixture({ dryRun: false })
    // first pass: utxo present but BELOW eligibility so never swept (999 sats < 1000)
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(16), vout: 0, sats: 5000, confirmed: true, inscriptionBearing: false },
    ])
    // make it ineligible by marking inscription-bearing so no sweep occurs
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(16), vout: 0, sats: 5000, confirmed: true, inscriptionBearing: true },
    ])
    const r1 = await f.keeper.sweepTick()
    assert.strictEqual(r1[0].action, 'none')
    // now it vanishes without any sweep
    f.chain.utxosByAddress.set(f.payTo, [])
    await f.keeper.sweepTick()
    const rows = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'compromise-vanished'").all()
    assert.ok(rows.length >= 1, 'expected vanished-compromise alert')
  })

  await test('dry-run then live restart: utxo swept+broadcast exactly once', async () => {
    // phase 1: dry-run keeper over a fresh DB — builds+records, never broadcasts
    const f = await keeperFixture({ dryRun: true })
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(31), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    let reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept')
    assert.strictEqual(f.chain.broadcasts.length, 0)
    // dry-run does not rebuild the identical tx on the next tick
    reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'none')
    assert.strictEqual(f.chain.broadcasts.length, 0)

    // phase 2: restart in LIVE mode with the same DB — the dry-run-recorded
    // utxo MUST be swept for real
    const liveConfig = loadConfig({
      IDCOIN_KEEPER_DRY_RUN: 'false',
      IDCOIN_KEEPER_ENABLE_TICKS: 'KEEP',
      IDCOIN_KEEPER_FEE_FLOOR: '2',
      IDCOIN_KEEPER_MAX_FEE_RATIO: '0.3',
    } as any)
    const liveKeeper = new Keeper({
      config: liveConfig, db: f.db, chain: f.chain as any, log: silentLog(),
      lpKeys: f.keys, legacyKeys: f.keeper.legacyKeys,
    })
    reports = await liveKeeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept')
    assert.strictEqual(reports[0].broadcast, true)
    assert.strictEqual(f.chain.broadcasts.length, 1, 'live sweep must broadcast exactly once')
    // second live tick: nothing more to do
    reports = await liveKeeper.sweepTick()
    assert.strictEqual(reports[0].action, 'none')
    assert.strictEqual(f.chain.broadcasts.length, 1)
  })

  await test('fee oracle fallback: sweep proceeds at max(floor, last-known, 10) with alert', async () => {
    const f = await keeperFixture({ dryRun: false })
    // prime last-known fee via one successful tick
    f.chain.fastestFee = 50
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(41), vout: 0, sats: 100000, confirmed: true, inscriptionBearing: false },
    ])
    let reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept')
    assert.strictEqual(f.db.getKv('lastFastestFee'), '50')

    // now the oracle fails — a NEW utxo must still be swept, no skip
    f.chain.getFastestFee = async () => { throw new Error('mempool.space down') }
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(41), vout: 0, sats: 100000, confirmed: true, inscriptionBearing: false },
      { txid: fakeTxid(42), vout: 0, sats: 100000, confirmed: true, inscriptionBearing: false },
    ])
    reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept', `expected sweep, got ${reports[0].action}: ${reports[0].detail}`)
    assert.strictEqual(f.chain.broadcasts.length, 2)
    // fallback rate = max(2, 50, 10) = 50 sat/vB — verify via the recorded tx fee
    const txid = bitcoin.Transaction.fromHex(f.chain.broadcasts[1]).getId()
    const row = f.db.getSweepTx(txid)
    const estVsize = Math.ceil(10.5 + 68 + 31)
    assert.strictEqual(row.fee, 50 * estVsize, `fee ${row.fee} != 50x${estVsize}`)
    // alert recorded
    const alerts = f.db.db.prepare("SELECT * FROM alerts WHERE kind = 'fee-oracle-fallback'").all()
    assert.ok(alerts.length >= 1, 'expected fee-oracle-fallback alert')
  })

  await test('fee oracle fallback with no history: uses max(floor, 10)', async () => {
    const f = await keeperFixture({ dryRun: false })
    f.config.feeFloorSatPerVb = 2
    f.chain.getFastestFee = async () => { throw new Error('down from the start') }
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(43), vout: 0, sats: 100000, confirmed: true, inscriptionBearing: false },
    ])
    const reports = await f.keeper.sweepTick()
    assert.strictEqual(reports[0].action, 'swept')
    const txid = bitcoin.Transaction.fromHex(f.chain.broadcasts[0]).getId()
    const row = f.db.getSweepTx(txid)
    const estVsize = Math.ceil(10.5 + 68 + 31)
    assert.strictEqual(row.fee, 10 * estVsize, `fee ${row.fee} != 10x${estVsize}`)
  })

  // -----------------------------------------------------------------------
  console.log('\n[6] vault ledger')
  await test('vault utxos tracked with sweep provenance and confirmed state', async () => {
    const f = await keeperFixture({ dryRun: false })
    f.chain.utxosByAddress.set(f.payTo, [
      { txid: fakeTxid(21), vout: 0, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    await f.keeper.sweepTick()
    const sweepTxid = bitcoin.Transaction.fromHex(f.chain.broadcasts[0]).getId()
    // vault now shows the sweep output (unconfirmed) plus a direct mint payment (confirmed)
    f.chain.utxosByAddress.set(f.vault, [
      { txid: sweepTxid, vout: 0, sats: 9500, confirmed: false, inscriptionBearing: false },
      { txid: fakeTxid(22), vout: 2, sats: 10000, confirmed: true, inscriptionBearing: false },
    ])
    await f.keeper.vaultLedgerTick()
    const rows = f.db.vaultUtxos('pinKi0')
    assert.strictEqual(rows.length, 2)
    const sweepRow = rows.find((r: any) => r.outpoint === `${sweepTxid}:0`)!
    assert.strictEqual(sweepRow.provenance, `sweep:${sweepTxid}`)
    assert.strictEqual(sweepRow.confirmed, 0)
    const directRow = rows.find((r: any) => r.outpoint === `${fakeTxid(22)}:2`)!
    assert.strictEqual(directRow.provenance, 'direct')
    assert.strictEqual(directRow.confirmed, 1)
    // spent: disappears from vault -> present=0
    f.chain.utxosByAddress.set(f.vault, [])
    await f.keeper.vaultLedgerTick()
    assert.strictEqual(f.db.vaultUtxos('pinKi0').length, 0)
  })

  // -----------------------------------------------------------------------
  console.log('\n[7] redaction')
  await test('logs never contain WIF or mnemonic words', () => {
    const mnemonic = bip39.generateMnemonic()
    const kp = ECPair.makeRandom({ network: BTC })
    const wif = kp.toWIF()
    const lines: string[] = []
    const log = new Logger((l) => lines.push(l))
    // simulate what happens at load time
    const { registerMnemonicForRedaction } = require('../src/log')
    registerMnemonicForRedaction(mnemonic)
    // try hard to leak
    log.error('failure processing key material', { wif, mnemonic, nested: { secret: wif } })
    log.info(`mnemonic was ${mnemonic} and wif ${wif}`)
    const joined = lines.join('\n')
    assert.ok(!joined.includes(wif), 'WIF leaked into logs')
    for (const word of mnemonic.split(' ')) {
      // individual mnemonic words must be redacted inside log payloads
      const occurrences = joined.split(word).length - 1
      assert.strictEqual(occurrences, 0, `mnemonic word "${word}" leaked into logs`)
    }
    assert.ok(joined.includes('[REDACTED-WIF]'))
    // redactSecrets standalone
    assert.ok(!redactSecrets(`err with ${wif}`).includes(wif))
  })

  await test('ops status payload contains no secret material', async () => {
    const f = await keeperFixture({ dryRun: true })
    const statusJson = JSON.stringify(f.keeper.status())
    for (const [, entry] of f.keeper.legacyKeys) {
      assert.ok(!statusJson.includes(entry.wif), 'WIF in ops status')
    }
    assert.ok(statusJson.includes('mvcFundingAddress'))
    assert.ok(statusJson.includes('dryRun'))
  })

  // -----------------------------------------------------------------------
  console.log('\n[8] ops endpoint')
  await test('ops endpoint binds 127.0.0.1 and serves status', async () => {
    const f = await keeperFixture({ dryRun: true })
    const server = startOpsServer(f.keeper, 0)
    await new Promise((r) => server.once('listening', r))
    const port = (server.address() as any).port
    const body: string = await new Promise((resolve, reject) => {
      import('http').then((http) => {
        http.get({ host: '127.0.0.1', port, path: '/status' }, (res) => {
          let data = ''
          res.on('data', (c) => (data += c))
          res.on('end', () => resolve(data))
        }).on('error', reject)
      })
    })
    const parsed = JSON.parse(body)
    assert.strictEqual(parsed.dryRun, true)
    assert.ok(parsed.counts)
    server.close()
  })

  // -----------------------------------------------------------------------
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
}

run().catch((e) => {
  console.error('test harness error:', e)
  process.exit(1)
})
