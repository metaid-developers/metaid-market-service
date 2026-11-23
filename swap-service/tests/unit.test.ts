import * as assert from 'assert'
import {
  quoteAddLiquidity,
  quoteRemoveLiquidity,
  quoteSwapMvcToToken,
  quoteSwapTokenToMvc,
} from '../src/quote'
import { AsyncKeyedQueue } from '../src/pool-store'

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

  console.log('=== ALL UNIT TESTS PASSED ===')
}

main().catch((e) => {
  console.error('UNIT TEST FAILED:', e)
  process.exit(1)
})
