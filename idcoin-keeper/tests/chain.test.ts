/** Chain/RPC/guard tests. All network traffic is local httptest-style servers. */
import * as assert from 'assert'
import * as http from 'http'
import axios from 'axios'
import * as bitcoin from 'bitcoinjs-lib'
import { BitcoinRpcClient, RpcError } from '../src/btcrpc'
import { CompositeChainClient, MempoolClient, ChainUtxo } from '../src/chain'
import { createSerializedTickRunner } from '../src/tick'

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

type Handler = (req: http.IncomingMessage, res: http.ServerResponse, body: string) => void
async function localServer(handler: Handler): Promise<{ url: string; close: () => Promise<void> }> {
  const server = http.createServer((req, res) => {
    let body = ''
    req.on('data', (c) => (body += c))
    req.on('end', () => handler(req, res, body))
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const port = (server.address() as any).port
  return { url: `http://127.0.0.1:${port}`, close: () => new Promise((resolve) => server.close(() => resolve())) }
}

function rpcReply(res: http.ServerResponse, result: any, error: any = null, status = 200) {
  res.statusCode = status
  res.setHeader('content-type', 'application/json')
  res.end(JSON.stringify({ result, error, id: 'x' }))
}

function fakeTxid(seed: number): string {
  return seed.toString(16).padStart(64, '0').slice(0, 64)
}

function utxo(seed: number, vout = 0): ChainUtxo {
  return { txid: fakeTxid(seed), vout, sats: 10000, confirmed: true, inscriptionBearing: false }
}

function sampleTxHex(): string {
  const tx = new bitcoin.Transaction()
  tx.version = 2
  tx.addInput(Buffer.alloc(32), 0)
  tx.addOutput(Buffer.alloc(22, 1), 1000)
  return tx.toHex()
}

class FakeHigun {
  constructor(public utxos: ChainUtxo[]) {}
  async getUtxos() { return this.utxos }
}

class FakeRpc {
  txOuts = new Map<string, any>()
  txOutError: any = null
  verbose: any = null
  verboseError: any = null
  txIndex = true
  fee: number | null = null
  feeError: any = null
  sendError: any = null
  sendTxid = 'rpc-txid'
  async getTxOut(txid: string, vout: number) {
    if (this.txOutError) throw this.txOutError
    return this.txOuts.has(`${txid}:${vout}`) ? this.txOuts.get(`${txid}:${vout}`) : null
  }
  async estimateSmartFeeSatPerVb() {
    if (this.feeError) throw this.feeError
    return this.fee
  }
  async sendRawTransaction() {
    if (this.sendError) throw this.sendError
    return this.sendTxid
  }
  async getRawTransactionVerbose() {
    if (this.verboseError) throw this.verboseError
    return this.verbose
  }
  async txIndexReady() { return this.txIndex }
}

class FakeMempool {
  fee = 7
  feeCalls = 0
  broadcasts: string[] = []
  broadcastResult = 'mempool-txid'
  async getFastestFee() { this.feeCalls++; return this.fee }
  async broadcastTx(hex: string) { this.broadcasts.push(hex); return this.broadcastResult }
}

async function run() {
  console.log('\n[rpc]')
  await test('BitcoinRpcClient sends basic auth and parses result', async () => {
    let auth = ''
    const srv = await localServer((req, res) => {
      auth = String(req.headers.authorization || '')
      rpcReply(res, { bestblockhash: 'abc' })
    })
    const client = new BitcoinRpcClient(srv.url, 'user', 'pass')
    const out = await (client as any).call('getbestblockhash')
    assert.strictEqual(out.bestblockhash, 'abc')
    assert.strictEqual(auth, `Basic ${Buffer.from('user:pass').toString('base64')}`)
    await srv.close()
  })

  await test('BitcoinRpcClient RPC error becomes RpcError with code; -5 is null', async () => {
    let code = -8
    const srv = await localServer((req, res) => rpcReply(res, null, { code, message: 'bad parameter' }, 500))
    const client = new BitcoinRpcClient(srv.url, 'u', 'p')
    await assert.rejects(client.getRawTransactionVerbose(fakeTxid(1)), (e: any) => e instanceof RpcError && e.code === -8)
    code = -5
    assert.strictEqual(await client.getRawTransactionVerbose(fakeTxid(1)), null)
    await srv.close()
  })

  await test('BitcoinRpcClient transport failure is a plain Error', async () => {
    const srv = await localServer((req, res) => rpcReply(res, null))
    const url = srv.url
    await srv.close()
    const client = new BitcoinRpcClient(url, 'u', 'p')
    await assert.rejects((client as any).call('getbestblockhash'), (e: any) => !(e instanceof RpcError) && e instanceof Error)
  })

  await test('BitcoinRpcClient fee conversion rounds up with floor 1', async () => {
    const rates = [0.0000331, 0.000000001]
    const srv = await localServer((req, res) => rpcReply(res, { feerate: rates.shift() }))
    const client = new BitcoinRpcClient(srv.url, 'u', 'p')
    assert.strictEqual(await client.estimateSmartFeeSatPerVb(2), 4)
    assert.strictEqual(await client.estimateSmartFeeSatPerVb(2), 1)
    await srv.close()
  })

  console.log('\n[composite]')
  await test('getUtxos drops gettxout-null utxos and reports discrepancy', async () => {
    const rpc = new FakeRpc()
    rpc.txOuts.set(`${fakeTxid(1)}:0`, { value: 1 })
    const discrepancies: string[] = []
    const c = new CompositeChainClient(new FakeHigun([utxo(1), utxo(2)]) as any, rpc as any, new FakeMempool() as any, (_a, detail) => discrepancies.push(detail))
    const out = await c.getUtxos('addr')
    assert.deepStrictEqual(out.map((u) => u.txid), [fakeTxid(1)])
    assert.ok(discrepancies[0].includes('n=1'))
    assert.ok(discrepancies[0].includes(`${fakeTxid(2)}:0`))
  })

  await test('getUtxos RPC failure returns Higun list unfiltered', async () => {
    const rpc = new FakeRpc()
    rpc.txOutError = new Error('connection refused')
    const list = [utxo(1), utxo(2)]
    const c = new CompositeChainClient(new FakeHigun(list) as any, rpc as any, new FakeMempool() as any)
    assert.deepStrictEqual(await c.getUtxos('addr'), list)
  })

  await test('fee uses Core estimate first and mempool only when estimate absent', async () => {
    const rpc = new FakeRpc()
    const mempool = new FakeMempool()
    const c = new CompositeChainClient(new FakeHigun([]) as any, rpc as any, mempool as any)
    rpc.fee = 4
    assert.strictEqual(await c.getFastestFee(), 4)
    assert.strictEqual(mempool.feeCalls, 0)
    rpc.fee = null
    assert.strictEqual(await c.getFastestFee(), 7)
    assert.strictEqual(mempool.feeCalls, 1)
  })

  await test('broadcast invalid RPC tx rethrows without mempool fallback', async () => {
    const rpc = new FakeRpc()
    rpc.sendError = new RpcError(-26, 'bad-txns-inputs-missingorspent')
    const mempool = new FakeMempool()
    const c = new CompositeChainClient(new FakeHigun([]) as any, rpc as any, mempool as any)
    await assert.rejects(c.broadcastTx(sampleTxHex()), (e: any) => e instanceof RpcError && e.code === -26)
    assert.strictEqual(mempool.broadcasts.length, 0)
  })

  await test('broadcast transport error falls back to mempool', async () => {
    const rpc = new FakeRpc()
    rpc.sendError = new Error('socket hangup')
    const mempool = new FakeMempool()
    const c = new CompositeChainClient(new FakeHigun([]) as any, rpc as any, mempool as any)
    assert.strictEqual(await c.broadcastTx(sampleTxHex()), 'mempool-txid')
    assert.strictEqual(mempool.broadcasts.length, 1)
  })

  await test('broadcast already-in-chain returns computed txid', async () => {
    const rpc = new FakeRpc()
    rpc.sendError = new RpcError(-27, 'Transaction already in block chain')
    const c = new CompositeChainClient(new FakeHigun([]) as any, rpc as any, new FakeMempool() as any)
    const hex = sampleTxHex()
    assert.strictEqual(await c.broadcastTx(hex), bitcoin.Transaction.fromHex(hex).getId())
  })

  await test('swept-input states use Core and txindex readiness', async () => {
    const txid = fakeTxid(10)
    const sweepTxid = fakeTxid(11)
    const rpc = new FakeRpc()
    const c = new CompositeChainClient(new FakeHigun([]) as any, rpc as any, new FakeMempool() as any)
    rpc.txOuts.set(`${txid}:0`, { value: 1 })
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'unspent' })

    rpc.txOuts.clear()
    rpc.verbose = { txid: sweepTxid, vin: [{ txid, vout: 0 }], confirmations: 3 }
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'ours', confirmations: 3 })
    rpc.verbose = { txid: sweepTxid, vin: [{ txid: fakeTxid(12), vout: 1 }], confirmations: 3 }
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'foreign' })
    rpc.verbose = null
    rpc.txIndex = true
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'foreign' })
    rpc.txIndex = false
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'unknown' })
    rpc.verboseError = new Error('rpc down')
    assert.deepStrictEqual(await c.getSweptInputState(txid, 0, sweepTxid), { state: 'unknown' })
  })

  console.log('\n[mempool guard]')
  const guardOpts = { intervalMs: 1, maxWaitMs: 5, openDurationMs: 25, timeoutMs: 100 }
  await test('breaker opens after 3 failures and stops hitting server', async () => {
    let hits = 0
    const srv = await localServer((req, res) => { hits++; res.statusCode = 500; res.end('bad') })
    const m = new MempoolClient(srv.url, axios.create(), guardOpts)
    for (let i = 0; i < 3; i++) await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    assert.strictEqual(hits, 3)
    await srv.close()
  })

  await test('429 opens breaker immediately', async () => {
    let hits = 0
    const srv = await localServer((req, res) => { hits++; res.statusCode = 429; res.end('limited') })
    const m = new MempoolClient(srv.url, axios.create(), guardOpts)
    await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    assert.strictEqual(hits, 1)
    await srv.close()
  })

  await test('half-open allows exactly one probe and then closes', async () => {
    let hits = 0
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const srv = await localServer((req, res) => {
      hits++
      if (hits <= 3) { res.statusCode = 500; res.end('bad'); return }
      if (hits === 4) {
        gate.then(() => { res.setHeader('content-type', 'application/json'); res.end('{"fastestFee":9}') })
      } else {
        res.end('{"fastestFee":9}')
      }
    })
    const m = new MempoolClient(srv.url, axios.create(), guardOpts)
    for (let i = 0; i < 3; i++) await assert.rejects(m.getFastestFee())
    await new Promise((r) => setTimeout(r, 30))
    const probe = m.getFastestFee()
    while (hits < 4) await new Promise((r) => setTimeout(r, 1))
    await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    assert.strictEqual(hits, 4)
    release()
    assert.strictEqual(await probe, 9)
    assert.strictEqual(await m.getFastestFee(), 9)
    await srv.close()
  })

  await test('limiter rejection does not hit server', async () => {
    let hits = 0
    const srv = await localServer((req, res) => { hits++; res.end('{"fastestFee":5}') })
    const m = new MempoolClient(srv.url, axios.create(), { ...guardOpts, intervalMs: 50, maxWaitMs: 0 })
    assert.strictEqual(await m.getFastestFee(), 5)
    await assert.rejects(m.getFastestFee(), /mempool.space unavailable/)
    assert.strictEqual(hits, 1)
    await srv.close()
  })

  console.log('\n[tick serialization]')
  await test('overlapping invocation is skipped with one warning per streak', async () => {
    let running = 0
    let completed = 0
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const warnings: string[] = []
    const runner = createSerializedTickRunner(async () => {
      running++
      await gate
      completed++
    }, (m) => warnings.push(m))
    const first = runner()
    await new Promise((r) => setTimeout(r, 1))
    await runner()
    await runner()
    assert.strictEqual(running, 1)
    assert.strictEqual(warnings.length, 1)
    release()
    await first
    assert.strictEqual(completed, 1)
  })

  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
}

run().catch((e) => {
  console.error('chain test harness error:', e)
  process.exit(1)
})
