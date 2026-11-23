import { TokenSwapProto } from './core-bridge'

export interface PoolReserveSnapshot {
  r1: bigint
  r2: bigint
  lp: bigint
  feeRate: number
  projFeeRate: number
  minProjFee: bigint
}

export interface SwapQuoteResult {
  direction: 'mvcToToken' | 'tokenToMvc'
  amountIn: string
  amountOut: string
  minAmountOut: string
  slippageBps: number
  projFee: string
  lpFee: string
  feeRateBps: number
  priceImpactBps: number
  spotPriceTokenPerSpace: string
  executionPriceTokenPerSpace: string
  reservesBefore: { r1: string; r2: string; lp: string }
  reservesAfter: { r1: string; r2: string; lp: string }
}

export interface AddLiquidityQuoteResult {
  mvcAddAmount: string
  tokenAddAmount: string
  minTokenRequired: string
  maxTokenAllowed: string
  lpMinted: string
  shareOfPoolBps: number
  reservesBefore: { r1: string; r2: string; lp: string }
  reservesAfter: { r1: string; r2: string; lp: string }
}

export interface RemoveLiquidityQuoteResult {
  lpBurnAmount: string
  mvcOut: string
  tokenOut: string
  shareRemovedBps: number
  reservesBefore: { r1: string; r2: string; lp: string }
  reservesAfter: { r1: string; r2: string; lp: string }
}

function formatRatio(num: bigint, den: bigint, precision = 8): string {
  if (den === BigInt(0)) return '0'
  const scale = BigInt(10) ** BigInt(precision)
  const scaled = (num * scale) / den
  const whole = scaled / scale
  const frac = (scaled % scale).toString().padStart(precision, '0').replace(/0+$/, '')
  return frac ? `${whole}.${frac}` : whole.toString()
}

export function quoteSwapMvcToToken(
  pool: PoolReserveSnapshot,
  amountIn: bigint,
  slippageBps = 100
): SwapQuoteResult {
  if (amountIn <= BigInt(0)) throw new Error('amountIn must be > 0')
  if (pool.r1 <= BigInt(0) || pool.r2 <= BigInt(0)) throw new Error('Pool has zero liquidity')

  const [amountOut, projFee, lpFee] = TokenSwapProto.swapToken1ToToken2(
    amountIn,
    pool.r1,
    pool.r2,
    pool.feeRate,
    pool.projFeeRate,
    pool.minProjFee
  ) as [bigint, bigint, bigint]

  if (amountOut <= BigInt(0) || amountOut >= pool.r2) {
    throw new Error('Insufficient pool token liquidity for requested input amount')
  }

  const minAmountOut = (amountOut * BigInt(10000 - slippageBps)) / BigInt(10000)
  // Ideal output with zero slippage & zero fee = amountIn * r2 / r1
  const idealOut = (amountIn * pool.r2) / pool.r1
  const priceImpactBps =
    idealOut > BigInt(0) && idealOut > amountOut
      ? Number(((idealOut - amountOut) * BigInt(10000)) / idealOut)
      : 0

  const newR1 = pool.r1 + amountIn - projFee
  const newR2 = pool.r2 - amountOut

  return {
    direction: 'mvcToToken',
    amountIn: amountIn.toString(),
    amountOut: amountOut.toString(),
    minAmountOut: minAmountOut.toString(),
    slippageBps,
    projFee: projFee.toString(),
    lpFee: lpFee.toString(),
    feeRateBps: pool.feeRate,
    priceImpactBps,
    spotPriceTokenPerSpace: formatRatio(pool.r2, pool.r1),
    executionPriceTokenPerSpace: formatRatio(amountOut, amountIn),
    reservesBefore: { r1: pool.r1.toString(), r2: pool.r2.toString(), lp: pool.lp.toString() },
    reservesAfter: { r1: newR1.toString(), r2: newR2.toString(), lp: pool.lp.toString() },
  }
}

export function quoteSwapTokenToMvc(
  pool: PoolReserveSnapshot,
  amountIn: bigint,
  slippageBps = 100
): SwapQuoteResult {
  if (amountIn <= BigInt(0)) throw new Error('amountIn must be > 0')
  if (pool.r1 <= BigInt(0) || pool.r2 <= BigInt(0)) throw new Error('Pool has zero liquidity')

  const [amountOut, projFee, lpFee] = TokenSwapProto.swapToken2ToToken1(
    amountIn,
    pool.r1,
    pool.r2,
    pool.feeRate,
    pool.projFeeRate,
    pool.minProjFee
  ) as [bigint, bigint, bigint]

  if (amountOut <= BigInt(0) || amountOut + projFee >= pool.r1) {
    throw new Error('Insufficient pool SPACE liquidity for requested token input amount')
  }

  const minAmountOut = (amountOut * BigInt(10000 - slippageBps)) / BigInt(10000)
  const idealOut = (amountIn * pool.r1) / pool.r2
  const priceImpactBps =
    idealOut > BigInt(0) && idealOut > amountOut
      ? Number(((idealOut - amountOut) * BigInt(10000)) / idealOut)
      : 0

  const newR1 = pool.r1 - amountOut - projFee
  const newR2 = pool.r2 + amountIn

  return {
    direction: 'tokenToMvc',
    amountIn: amountIn.toString(),
    amountOut: amountOut.toString(),
    minAmountOut: minAmountOut.toString(),
    slippageBps,
    projFee: projFee.toString(),
    lpFee: lpFee.toString(),
    feeRateBps: pool.feeRate,
    priceImpactBps,
    spotPriceTokenPerSpace: formatRatio(pool.r2, pool.r1),
    executionPriceTokenPerSpace: formatRatio(amountIn, amountOut),
    reservesBefore: { r1: pool.r1.toString(), r2: pool.r2.toString(), lp: pool.lp.toString() },
    reservesAfter: { r1: newR1.toString(), r2: newR2.toString(), lp: pool.lp.toString() },
  }
}

export function quoteAddLiquidity(
  pool: PoolReserveSnapshot,
  mvcAddAmount: bigint,
  customTokenAddAmount?: bigint
): AddLiquidityQuoteResult {
  if (mvcAddAmount <= BigInt(0)) throw new Error('mvcAddAmount must be > 0')
  if (pool.r1 <= BigInt(0) || pool.r2 <= BigInt(0) || pool.lp <= BigInt(0)) {
    throw new Error('Pool must have non-zero initial liquidity')
  }
  const minTokenRequired = (pool.r2 * mvcAddAmount) / pool.r1
  const recommendedToken =
    (pool.r2 * mvcAddAmount) % pool.r1 === BigInt(0) ? minTokenRequired : minTokenRequired + BigInt(1)
  const maxTokenAllowed = (pool.r2 * mvcAddAmount * BigInt(105)) / (pool.r1 * BigInt(100))
  const tokenAddAmount = customTokenAddAmount !== undefined ? customTokenAddAmount : recommendedToken

  if (tokenAddAmount < minTokenRequired || tokenAddAmount > maxTokenAllowed) {
    throw new Error(`tokenAddAmount ${tokenAddAmount} out of allowed range [${minTokenRequired}, ${maxTokenAllowed}]`)
  }
  const lpMinted = (mvcAddAmount * pool.lp) / pool.r1
  if (lpMinted <= BigInt(0)) throw new Error('mvcAddAmount too small to mint >= 1 LP unit')

  const newLp = pool.lp + lpMinted
  const shareOfPoolBps = Number((lpMinted * BigInt(10000)) / newLp)

  return {
    mvcAddAmount: mvcAddAmount.toString(),
    tokenAddAmount: tokenAddAmount.toString(),
    minTokenRequired: minTokenRequired.toString(),
    maxTokenAllowed: maxTokenAllowed.toString(),
    lpMinted: lpMinted.toString(),
    shareOfPoolBps,
    reservesBefore: { r1: pool.r1.toString(), r2: pool.r2.toString(), lp: pool.lp.toString() },
    reservesAfter: {
      r1: (pool.r1 + mvcAddAmount).toString(),
      r2: (pool.r2 + tokenAddAmount).toString(),
      lp: newLp.toString(),
    },
  }
}

export function quoteRemoveLiquidity(
  pool: PoolReserveSnapshot,
  lpBurnAmount: bigint
): RemoveLiquidityQuoteResult {
  if (lpBurnAmount <= BigInt(0) || lpBurnAmount > pool.lp) {
    throw new Error(`Invalid lpBurnAmount=${lpBurnAmount} (pool lp=${pool.lp})`)
  }
  const [mvcOut, tokenOut] = TokenSwapProto.countLpRemoveAmount(
    lpBurnAmount,
    pool.r1,
    pool.r2,
    pool.lp
  ) as [bigint, bigint]

  const shareRemovedBps = Number((lpBurnAmount * BigInt(10000)) / pool.lp)
  return {
    lpBurnAmount: lpBurnAmount.toString(),
    mvcOut: mvcOut.toString(),
    tokenOut: tokenOut.toString(),
    shareRemovedBps,
    reservesBefore: { r1: pool.r1.toString(), r2: pool.r2.toString(), lp: pool.lp.toString() },
    reservesAfter: {
      r1: (pool.r1 - mvcOut).toString(),
      r2: (pool.r2 - tokenOut).toString(),
      lp: (pool.lp - lpBurnAmount).toString(),
    },
  }
}
