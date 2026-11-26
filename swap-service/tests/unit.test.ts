import * as assert from 'assert'
import * as fs from 'fs'
import * as os from 'os'
import * as path from 'path'
import { SwapEngine, axios, loadSwapContractsFast, scrypt } from '../src/core-bridge'
import {
  quoteAddLiquidity,
  quoteRemoveLiquidity,
  quoteSwapMvcToToken,
  quoteSwapTokenToMvc,
} from '../src/quote'
import { AsyncKeyedQueue } from '../src/pool-store'
import { deriveContractHashArray } from '../src/pool-admin'
import { createSwapServer } from '../src/server'

async function main() {
  console.log('=== swap-service Unit & Regression Tests ===')

  const basePool = {
    r1: BigInt(120100),
    r2: BigInt(2000000000),
    lp: BigInt(120100),
    feeRate: 25,
    projFeeRate: 5,
    minProjFee: BigInt(1000),
  }

  // 1. Mandatory §9 Phase 1 regression check
  const q1 = quoteSwapMvcToToken(basePool, BigInt(10000), 100)
  assert.strictEqual(q1.amountOut, '153343581', `Expected 153343581, got ${q1.amountOut}`)
  assert.strictEqual(q1.projFee, '0', `Expected projFee=0, got ${q1.projFee}`)
  assert.strictEqual(q1.reservesAfter.r1, '130100')
  assert.strictEqual(q1.reservesAfter.r2, '1846656419')
  console.log('  [PASS] Quote regression (120,100 / 2,000,000,000 + 10,000 -> 153,343,581, projFee=0)')

  // 2. projFee > 0 threshold check (2,000,000 * 5 / 10000 = 1000 >= minProjFee 1000)
  const q2 = quoteSwapMvcToToken(basePool, BigInt(2000000), 100)
  assert.strictEqual(q2.projFee, '1000')
  assert.strictEqual(q2.reservesAfter.r1, String(120100 + 2000000 - 1000))
  console.log('  [PASS] Quote projFee > 0 threshold (2,000,000 sats -> projFee=1000)')

  // 3. Reverse swap (token -> SPACE)
  const q3 = quoteSwapTokenToMvc(
    { ...basePool, r1: BigInt(130100), r2: BigInt(1846656419) },
    BigInt(153343581),
    50
  )
  assert.ok(BigInt(q3.amountOut) > BigInt(0) && BigInt(q3.amountOut) < BigInt(10000))
  assert.strictEqual(q3.projFee, '0')
  console.log(`  [PASS] Reverse quote Token->SPACE (153,343,581 -> ${q3.amountOut} sats)`)

  // 4. AddLiquidity proportional quote & [100%, 105%] bounds
  const qAdd = quoteAddLiquidity(basePool, BigInt(10000))
  assert.strictEqual(qAdd.lpMinted, '10000')
  assert.ok(BigInt(qAdd.tokenAddAmount) >= BigInt(qAdd.minTokenRequired))
  assert.ok(BigInt(qAdd.tokenAddAmount) <= BigInt(qAdd.maxTokenAllowed))
  assert.throws(() => quoteAddLiquidity(basePool, BigInt(10000), BigInt(1)), /out of allowed range/)
  console.log(`  [PASS] AddLiquidity quote (10,000 SPACE + ${qAdd.tokenAddAmount} Token -> ${qAdd.lpMinted} LP)`)

  // 5. RemoveLiquidity proportional quote
  const qRem = quoteRemoveLiquidity(
    { ...basePool, r1: BigInt(183535), r2: BigInt(1560360178), lp: BigInt(127020) },
    BigInt(6920)
  )
  assert.strictEqual(qRem.mvcOut, '9998')
  assert.strictEqual(qRem.tokenOut, '85007813')
  console.log(`  [PASS] RemoveLiquidity quote (6,920 LP -> ${qRem.mvcOut} SPACE + ${qRem.tokenOut} Token)`)

  // 6. Per-pool FIFO serial queue concurrency test
  const queue = new AsyncKeyedQueue()
  const events: string[] = []
  const task = async (id: string, holdMs: number) => {
    const release = await queue.acquire('pool-A', 5000)
    events.push(`start-${id}`)
    await new Promise((r) => setTimeout(r, holdMs))
    events.push(`end-${id}`)
    release()
  }
  await Promise.all([task('1', 60), task('2', 30), task('3', 10)])
  assert.deepStrictEqual(events, ['start-1', 'end-1', 'start-2', 'end-2', 'start-3', 'end-3'])
  console.log('  [PASS] Per-pool AsyncKeyedQueue FIFO serialization (no overlapping execution)')

  // 7. Admin contractHashArray derivation matches verified mainnet SPACE/MVST pool byte-for-byte
  const poolsJson = JSON.parse(fs.readFileSync(path.resolve(__dirname, '..', 'pools.json'), 'utf8'))
  const mvstPool = poolsJson.pools[0]
  const contracts = loadSwapContractsFast()
  const chain = new SwapEngine.MetaletChainClient({
    apiBase: 'https://www.metalet.space/wallet-api/v4/mvc',
    net: 'livenet',
  })
  const liveState = await SwapEngine.loadPoolState(
    chain,
    mvstPool.bootstrapHead.poolTxid,
    mvstPool.bootstrapHead.lpGenesisTxid,
    mvstPool.bootstrapHead.lpGenesisVout
  )
  const derived = deriveContractHashArray({
    contracts,
    swapContractID: mvstPool.contract.swapContractID,
    tokenID: mvstPool.contract.tokenID,
    tokenScriptCodeHash: mvstPool.contract.tokenScriptCodeHash,
    ownerAddressHashHex: liveState.ownerAddress.toString('hex'),
  })
  assert.strictEqual(derived.contractHashRootHex, mvstPool.contract.contractHashRoot)
  assert.strictEqual(derived.contractHashArrayHex, mvstPool.contract.contractHashArrayHex)
  console.log('  [PASS] Admin deriveContractHashArray matches live mainnet SPACE/MVST contractHashRoot byte-for-byte')

  // 8. Admin HTTP endpoints: auth guard (503/401/403), whitelist rejection, on-chain auto-import, hot-register & hot-remove
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'swap-admin-test-'))
  const tmpPoolsFile = path.join(tmpDir, 'pools.json')
  const tmpDataDir = path.join(tmpDir, 'data')
  fs.writeFileSync(
    tmpPoolsFile,
    JSON.stringify({ whitelistSwapCodeHashes: poolsJson.whitelistSwapCodeHashes, pools: [] }, null, 2)
  )

  const prevEnv = {
    SWAP_POOLS_FILE: process.env.SWAP_POOLS_FILE,
    SWAP_DATA_DIR: process.env.SWAP_DATA_DIR,
    SWAP_ADMIN_TOKEN: process.env.SWAP_ADMIN_TOKEN,
  }
  try {
    // 8a. Disabled when SWAP_ADMIN_TOKEN is unset -> 503
    process.env.SWAP_POOLS_FILE = tmpPoolsFile
    process.env.SWAP_DATA_DIR = tmpDataDir
    delete process.env.SWAP_ADMIN_TOKEN
    const srvDisabled = createSwapServer()
    await new Promise<void>((r) => srvDisabled.server.listen(0, '127.0.0.1', r))
    const port1 = (srvDisabled.server.address() as any).port
    const res503 = await axios.get(`http://127.0.0.1:${port1}/internal/swap/admin/pools`, {
      validateStatus: () => true,
    })
    assert.strictEqual(res503.status, 503, `Expected 503 without SWAP_ADMIN_TOKEN, got ${res503.status}`)
    await new Promise<void>((r) => srvDisabled.server.close(() => r()))

    // 8b. Enabled with token -> 401/403 checks + import + hot activation + whitelist reject + delete
    const testToken = 'test-secret-admin-token-9988'
    process.env.SWAP_ADMIN_TOKEN = testToken
    const srv = createSwapServer()
    await new Promise<void>((r) => srv.server.listen(0, '127.0.0.1', r))
    const port = (srv.server.address() as any).port
    const base = `http://127.0.0.1:${port}`

    const res401 = await axios.get(`${base}/internal/swap/admin/pools`, { validateStatus: () => true })
    assert.strictEqual(res401.status, 401)
    const res403 = await axios.get(`${base}/internal/swap/admin/pools`, {
      headers: { 'X-Swap-Admin-Token': 'wrong-token' },
      validateStatus: () => true,
    })
    assert.strictEqual(res403.status, 403)

    // Import real mainnet SPACE/MVST pool with ONLY poolTxid + lpGenesisTxid (zero manual contract fields!)
    const impRes = await axios.post(
      `${base}/internal/swap/admin/pools/import`,
      {
        poolId: 'space-mvst-imported',
        poolTxid: mvstPool.bootstrapHead.poolTxid,
        lpGenesisTxid: mvstPool.bootstrapHead.lpGenesisTxid,
        lpGenesisVout: mvstPool.bootstrapHead.lpGenesisVout,
      },
      { headers: { 'X-Swap-Admin-Token': testToken } }
    )
    assert.strictEqual(impRes.status, 200)
    assert.strictEqual(impRes.data.code, 0)
    assert.strictEqual(impRes.data.data.entry.contract.contractHashRoot, mvstPool.contract.contractHashRoot)
    assert.strictEqual(impRes.data.data.entry.token2.symbol, 'MVST')
    assert.strictEqual(impRes.data.data.entry.token2.decimals, 8)

    // Public GET /internal/swap/pools immediately serves the hot-imported pool without restart
    const pubList = await axios.get(`${base}/internal/swap/pools`)
    assert.strictEqual(pubList.data.data.pools.length, 1)
    assert.strictEqual(pubList.data.data.pools[0].poolId, 'space-mvst-imported')

    // Reject unwhitelisted pool registration
    const badEntry = JSON.parse(JSON.stringify(impRes.data.data.entry))
    badEntry.poolId = 'bad-pool'
    badEntry.contract.swapCodeHash = '0000000000000000000000000000000000000000'
    const badRes = await axios.post(
      `${base}/internal/swap/admin/pools/register`,
      { entry: badEntry },
      { headers: { 'X-Swap-Admin-Token': testToken }, validateStatus: () => true }
    )
    assert.strictEqual(badRes.status, 500)
    assert.ok(String(badRes.data.message).includes('not in whitelistSwapCodeHashes'))

    // Hot-remove SPACE/MVST pool and verify persisted pools.json is clean
    const delRes = await axios.delete(`${base}/internal/swap/admin/pools/space-mvst-imported`, {
      headers: { 'X-Swap-Admin-Token': testToken },
    })
    assert.strictEqual(delRes.status, 200)
    const pubListAfter = await axios.get(`${base}/internal/swap/pools`)
    assert.strictEqual(pubListAfter.data.data.pools.length, 0)
    const persisted = JSON.parse(fs.readFileSync(tmpPoolsFile, 'utf8'))
    assert.strictEqual(persisted.pools.length, 0)

    // 8c. Native Token/Token pool auto-import, live state, quote aliases, and dual-FT positions
    const ttImpRes = await axios.post(
      `${base}/internal/swap/admin/pools/import`,
      {
        poolId: 'tmva-tmvb-imported',
        poolTxid: '322ec5ff69fdb38e24cabfe6ef5ac1d454db846ee749f556d507bbc443d23805',
        lpGenesisTxid: 'ea442a7e7a9b01423e5465b4963a260ffeec2e0ba1acfa89f891a216673a6868',
        lpGenesisVout: 2,
      },
      { headers: { 'X-Swap-Admin-Token': testToken } }
    )
    assert.strictEqual(ttImpRes.status, 200)
    assert.strictEqual(ttImpRes.data.data.entry.poolType, 'token-token')
    assert.strictEqual(ttImpRes.data.data.entry.token1.symbol, 'TMVA')
    assert.strictEqual(ttImpRes.data.data.entry.token2.symbol, 'TMVB')
    assert.strictEqual(ttImpRes.data.data.pool.poolType, 'token-token')
    assert.strictEqual(ttImpRes.data.data.pool.poolTokenVout, 4)
    assert.strictEqual(ttImpRes.data.data.pool.poolToken2Vout, 5)
    assert.strictEqual(ttImpRes.data.data.pool.r1, '104251020')
    assert.strictEqual(ttImpRes.data.data.pool.r2, '530789317')
    assert.strictEqual(ttImpRes.data.data.pool.lp, '105177797')

    const ttPools = await axios.get(`${base}/internal/swap/pools`)
    assert.strictEqual(ttPools.data.data.pools.length, 1)
    assert.strictEqual(ttPools.data.data.pools[0].poolType, 'token-token')

    const ttQ1 = await axios.post(`${base}/internal/swap/quote`, {
      poolId: 'tmva-tmvb-imported',
      operation: 'swapToken1ToToken2',
      amountIn: '1000000',
      slippageBps: 100,
    })
    assert.strictEqual(ttQ1.status, 200)
    assert.ok(BigInt(ttQ1.data.data.amountOut) > BigInt(0))

    const ttQ2 = await axios.post(`${base}/internal/swap/quote`, {
      poolId: 'tmva-tmvb-imported',
      operation: 'swapToken2ToToken1',
      amountIn: '5000000',
      slippageBps: 100,
    })
    assert.strictEqual(ttQ2.status, 200)
    assert.ok(BigInt(ttQ2.data.data.amountOut) > BigInt(0))

    const ttQAdd = await axios.post(`${base}/internal/swap/quote`, {
      poolId: 'tmva-tmvb-imported',
      operation: 'addLiquidity',
      token1AddAmount: '1000000',
    })
    assert.strictEqual(ttQAdd.status, 200)
    assert.ok(BigInt(ttQAdd.data.data.lpMinted) > BigInt(0))

    const validTestAddress = scrypt.mvc.Address.fromPublicKeyHash(liveState.ownerAddress, 'livenet').toString()
    const ttPos = await axios.get(`${base}/internal/swap/positions`, {
      params: { address: validTestAddress },
    })
    assert.strictEqual(ttPos.status, 200)
    assert.ok(ttPos.data.data.tokenBalances[ttImpRes.data.data.entry.token1.tokenID] !== undefined)
    assert.ok(ttPos.data.data.tokenBalances[ttImpRes.data.data.entry.token2.tokenID] !== undefined)
    assert.strictEqual(ttPos.data.data.positions[0].poolType, 'token-token')
    assert.ok(BigInt(ttPos.data.data.positions[0].lpBalance) > BigInt(0))

    await new Promise<void>((r) => srv.server.close(() => r()))
    console.log('  [PASS] Admin HTTP endpoints + Token/Token pool auto-import, quotes & dual-FT positions')
  } finally {
    if (prevEnv.SWAP_POOLS_FILE === undefined) delete process.env.SWAP_POOLS_FILE
    else process.env.SWAP_POOLS_FILE = prevEnv.SWAP_POOLS_FILE
    if (prevEnv.SWAP_DATA_DIR === undefined) delete process.env.SWAP_DATA_DIR
    else process.env.SWAP_DATA_DIR = prevEnv.SWAP_DATA_DIR
    if (prevEnv.SWAP_ADMIN_TOKEN === undefined) delete process.env.SWAP_ADMIN_TOKEN
    else process.env.SWAP_ADMIN_TOKEN = prevEnv.SWAP_ADMIN_TOKEN
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }

  void scrypt
  console.log('=== ALL UNIT TESTS PASSED ===')
}

main().catch((e) => {
  console.error('UNIT TEST FAILED:', e)
  process.exit(1)
})
