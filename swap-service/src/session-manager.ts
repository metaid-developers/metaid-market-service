import * as crypto from 'crypto'
import { PoolStore } from './pool-store'
import { getPoolType } from './config'
import { SwapEngine, TokenSwapEngine } from './core-bridge'
import {
  quoteAddLiquidity,
  quoteRemoveLiquidity,
  quoteSwapMvcToToken,
  quoteSwapTokenToMvc,
} from './quote'

export type SwapOperation =
  | 'swapMvcToToken'
  | 'swapTokenToMvc'
  | 'swapToken1ToToken2'
  | 'swapToken2ToToken1'
  | 'addLiquidity'
  | 'removeLiquidity'

export interface StartSessionParams {
  poolId: string
  operation: SwapOperation
  userAddress: string
  amountIn?: string        // sats or raw token1 units for token1->token2; raw token2 units for token2->token1
  slippageBps?: number
  minAmountOut?: string
  mvcAddAmount?: string    // sats (space-token) or raw token1 units (token-token) for addLiquidity
  tokenAddAmount?: string  // optional raw token2 units for addLiquidity
  token1AddAmount?: string // alias of mvcAddAmount for token-token pools
  token2AddAmount?: string // alias of tokenAddAmount for token-token pools
  lpBurnAmount?: string    // raw LP units for removeLiquidity
  broadcast?: boolean      // defaults to true
}

export interface SignRequestPayload {
  txHex: string
  scriptHex: string
  inputIndex: number
  satoshis: number
  sigtype: number
  message?: string
}

export interface SessionStepResponse {
  sessionId: string
  poolId: string
  operation: SwapOperation
  status: 'awaiting_signature' | 'completed' | 'failed' | 'cancelled'
  stepIndex: number
  signRequest?: SignRequestPayload
  quote?: any
  result?: {
    prepTxid: string
    depositTxid?: string
    deposit2Txid?: string
    execTxid: string
    execSizeBytes: number
    execFee: number
    details: Record<string, string | number>
    newPoolState: {
      poolTxid: string
      poolVout: number
      poolTokenVout: number
      poolToken2Vout?: number
      lpGenesisTxid: string
      lpGenesisVout: number
      r1: string
      r2: string
      lp: string
    }
    broadcasted: boolean
  }
  error?: string
}

interface InternalSession {
  id: string
  poolId: string
  operation: SwapOperation
  userAddress: string
  stepIndex: number
  quote: any
  createdAt: number
  status: SessionStepResponse['status']
  pendingSignResolve?: (sigRes: any) => void
  pendingSignReject?: (err: Error) => void
  onNextEvent?: (ev: SessionStepResponse) => void
  lastResponse?: SessionStepResponse
  releaseLock?: () => void
  ttlTimer?: NodeJS.Timeout
}

export class SwapSessionManager {
  private readonly store: PoolStore
  private readonly sessions: Map<string, InternalSession> = new Map()

  constructor(store: PoolStore) {
    this.store = store
  }

  getSession(sessionId: string): SessionStepResponse | undefined {
    return this.sessions.get(sessionId)?.lastResponse
  }

  async startSession(params: StartSessionParams): Promise<SessionStepResponse> {
    const entry = this.store.getPoolEntry(params.poolId)
    if (!params.userAddress) throw new Error('userAddress is required')

    const sessionId = crypto.randomBytes(16).toString('hex')
    // Acquire per-pool FIFO serial lock BEFORE reading pool head & user UTXOs
    const releaseLock = await this.store.poolQueue.acquire(
      params.poolId,
      this.store.config.poolLockTimeoutMs
    )

    const session: InternalSession = {
      id: sessionId,
      poolId: params.poolId,
      operation: params.operation,
      userAddress: params.userAddress,
      stepIndex: 0,
      quote: null,
      createdAt: Date.now(),
      status: 'awaiting_signature',
      releaseLock,
    }
    this.sessions.set(sessionId, session)

    const cleanupLock = () => {
      if (session.ttlTimer) {
        clearTimeout(session.ttlTimer)
        session.ttlTimer = undefined
      }
      if (session.releaseLock) {
        const fn = session.releaseLock
        session.releaseLock = undefined
        fn()
      }
    }

    session.ttlTimer = setTimeout(() => {
      if (session.status === 'awaiting_signature' && session.pendingSignReject) {
        session.status = 'cancelled'
        session.pendingSignReject(new Error('Signing session expired'))
        cleanupLock()
      }
    }, this.store.config.poolLockTimeoutMs)

    const emitEvent = (ev: SessionStepResponse) => {
      session.lastResponse = ev
      if (session.onNextEvent) {
        const cb = session.onNextEvent
        session.onNextEvent = undefined
        cb(ev)
      }
    }

    const waitNextEvent = (): Promise<SessionStepResponse> =>
      new Promise((resolve) => {
        session.onNextEvent = resolve
      })

    // Interactive Metalet signer that pauses the engine at each signature step
    const interactiveSigner = {
      getAddress: async () => params.userAddress,
      signTransaction: async (req: { transaction: SignRequestPayload }) => {
        session.stepIndex += 1
        return new Promise((resolve, reject) => {
          session.pendingSignResolve = resolve
          session.pendingSignReject = reject
          emitEvent({
            sessionId,
            poolId: params.poolId,
            operation: params.operation,
            status: 'awaiting_signature',
            stepIndex: session.stepIndex,
            signRequest: {
              ...req.transaction,
              sigtype: req.transaction.sigtype || SwapEngine.SIG_ALL,
            },
            quote: session.quote,
          })
        })
      },
    }

    const firstEventPromise = waitNextEvent()

    // Run engine asynchronously inside pool lock
    ;(async () => {
      try {
        const poolState = await this.store.getPoolState(params.poolId)
        const spaceUtxos = await this.store.getFreshSpaceUtxos(params.userAddress)
        const feeRate = await this.store.chain.getFeeRate()
        const broadcast = params.broadcast !== false
        const slippageBps = params.slippageBps !== undefined ? params.slippageBps : 100

        const poolType = getPoolType(entry)
        const tokenSwapPoolConfig =
          poolType === 'token-token'
            ? {
                swapContractID: entry.contract.swapContractID,
                swapSensibleID: entry.contract.swapSensibleID,
                token1ID: entry.contract.token1ID || entry.token1.tokenID || '',
                token1ScriptCodeHash: entry.contract.token1ScriptCodeHash || entry.token1.codeHash || '',
                token2ID: entry.contract.token2ID || entry.token2.tokenID,
                token2ScriptCodeHash: entry.contract.token2ScriptCodeHash || entry.token2.codeHash,
                lpTokenID: entry.contract.lpTokenID,
                fetchTokenHash: entry.contract.fetchTokenHash,
                swapCodeHash: entry.contract.swapCodeHash,
                contractHashRoot: entry.contract.contractHashRoot,
                contractHashArrayHex: entry.contract.contractHashArrayHex,
              }
            : null

        let execRes: any
        const isSwap1To2 = params.operation === 'swapMvcToToken' || params.operation === 'swapToken1ToToken2'
        const isSwap2To1 = params.operation === 'swapTokenToMvc' || params.operation === 'swapToken2ToToken1'

        if (isSwap1To2) {
          if (!params.amountIn) throw new Error(`amountIn is required for ${params.operation}`)
          const amountIn = BigInt(params.amountIn)
          const q = quoteSwapMvcToToken(poolState, amountIn, slippageBps)
          session.quote = q
          const minOut = params.minAmountOut ? BigInt(params.minAmountOut) : BigInt(q.minAmountOut)
          if (poolType === 'token-token') {
            const tok1Utxos = await this.store.getFreshTokenUtxos(
              params.userAddress,
              tokenSwapPoolConfig!.token1ScriptCodeHash,
              tokenSwapPoolConfig!.token1ID
            )
            const chosen1 = tok1Utxos.find((u: any) => u.amount >= amountIn)
            if (!chosen1) {
              throw new Error(
                `No single unspent ${entry.token1.symbol} UTXO >= ${amountIn}. Merge tokens in Metalet first.`
              )
            }
            execRes = await TokenSwapEngine.executeTokenToTokenSwap({
              contracts: this.store.getTokenSwapContracts(),
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: tokenSwapPoolConfig!,
              poolState,
              direction: 'token1ToToken2',
              userTokenUtxo: chosen1,
              tokenAmountIn: amountIn,
              minAmountOut: minOut,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          } else {
            execRes = await SwapEngine.executeSwapMvcToToken({
              contracts: this.store.contracts,
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: entry.contract,
              poolState,
              mvcAddAmount: amountIn,
              minTokenOut: minOut,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          }
        } else if (isSwap2To1) {
          if (!params.amountIn) throw new Error(`amountIn (raw token units) is required for ${params.operation}`)
          const amountIn = BigInt(params.amountIn)
          const q = quoteSwapTokenToMvc(poolState, amountIn, slippageBps)
          session.quote = q
          const minOut = params.minAmountOut ? BigInt(params.minAmountOut) : BigInt(q.minAmountOut)
          const tokUtxos = await this.store.getFreshTokenUtxos(
            params.userAddress,
            entry.token2.codeHash,
            entry.token2.tokenID
          )
          const chosen = tokUtxos.find((u: any) => u.amount >= amountIn)
          if (!chosen) {
            throw new Error(`No single unspent ${entry.token2.symbol} UTXO >= ${amountIn}. Merge tokens in Metalet first.`)
          }
          if (poolType === 'token-token') {
            execRes = await TokenSwapEngine.executeTokenToTokenSwap({
              contracts: this.store.getTokenSwapContracts(),
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: tokenSwapPoolConfig!,
              poolState,
              direction: 'token2ToToken1',
              userTokenUtxo: chosen,
              tokenAmountIn: amountIn,
              minAmountOut: minOut,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          } else {
            execRes = await SwapEngine.executeSwapTokenToMvc({
              contracts: this.store.contracts,
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: entry.contract,
              poolState,
              userTokenUtxo: chosen,
              tokenInAmount: amountIn,
              minMvcOut: minOut,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          }
        } else if (params.operation === 'addLiquidity') {
          const rawAmt1 = params.token1AddAmount || params.mvcAddAmount
          if (!rawAmt1) {
            throw new Error(
              poolType === 'token-token'
                ? 'token1AddAmount (or mvcAddAmount) is required for addLiquidity'
                : 'mvcAddAmount (sats) is required for addLiquidity'
            )
          }
          const amt1 = BigInt(rawAmt1)
          const rawAmt2 = params.token2AddAmount || params.tokenAddAmount
          const customTok2 = rawAmt2 ? BigInt(rawAmt2) : undefined
          const q = quoteAddLiquidity(poolState, amt1, customTok2)
          session.quote = q
          const reqTok2 = BigInt(q.tokenAddAmount)

          const tok2Utxos = await this.store.getFreshTokenUtxos(
            params.userAddress,
            entry.token2.codeHash,
            entry.token2.tokenID
          )
          const chosen2 = tok2Utxos.find((u: any) => u.amount >= reqTok2)
          if (!chosen2) {
            throw new Error(`No single unspent ${entry.token2.symbol} UTXO >= ${reqTok2}.`)
          }

          if (poolType === 'token-token') {
            const tok1Utxos = await this.store.getFreshTokenUtxos(
              params.userAddress,
              tokenSwapPoolConfig!.token1ScriptCodeHash,
              tokenSwapPoolConfig!.token1ID
            )
            const chosen1 = tok1Utxos.find((u: any) => u.amount >= amt1)
            if (!chosen1) {
              throw new Error(`No single unspent ${entry.token1.symbol} UTXO >= ${amt1}.`)
            }
            execRes = await TokenSwapEngine.executeTokenSwapAddLiquidity({
              contracts: this.store.getTokenSwapContracts(),
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: tokenSwapPoolConfig!,
              poolState,
              userToken1Utxo: chosen1,
              userToken2Utxo: chosen2,
              token1AddAmount: amt1,
              token2AddAmount: reqTok2,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          } else {
            execRes = await SwapEngine.executeAddLiquidity({
              contracts: this.store.contracts,
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: entry.contract,
              poolState,
              userTokenUtxo: chosen2,
              mvcAddAmount: amt1,
              tokenAddAmount: reqTok2,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          }
        } else if (params.operation === 'removeLiquidity') {
          if (!params.lpBurnAmount) throw new Error('lpBurnAmount is required for removeLiquidity')
          const lpBurn = BigInt(params.lpBurnAmount)
          const q = quoteRemoveLiquidity(poolState, lpBurn)
          session.quote = q
          const lpUtxos = await this.store.getFreshTokenUtxos(
            params.userAddress,
            entry.lpToken.codeHash,
            entry.lpToken.tokenID
          )
          const chosen = lpUtxos.find((u: any) => u.amount >= lpBurn)
          if (!chosen) {
            throw new Error(`No single unspent ${entry.lpToken.symbol} UTXO >= ${lpBurn}.`)
          }
          if (poolType === 'token-token') {
            execRes = await TokenSwapEngine.executeTokenSwapRemoveLiquidity({
              contracts: this.store.getTokenSwapContracts(),
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: tokenSwapPoolConfig!,
              poolState,
              userLpUtxo: chosen,
              lpBurnAmount: lpBurn,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          } else {
            execRes = await SwapEngine.executeRemoveLiquidity({
              contracts: this.store.contracts,
              chain: this.store.chain,
              signer: interactiveSigner,
              poolConfig: entry.contract,
              poolState,
              userLpUtxo: chosen,
              lpBurnAmount: lpBurn,
              spaceUtxos,
              feeRateSatPerByte: feeRate,
              broadcast,
            })
          }
        } else {
          throw new Error(`Unsupported operation: ${params.operation}`)
        }

        // Normalize execution result across SwapEngine (`newPoolState`) and TokenSwapEngine (`newPoolHead` + `reservesAfter`)
        const normalizedNewPoolState = execRes.newPoolState
          ? execRes.newPoolState
          : {
              poolTxid: execRes.newPoolHead.poolTxid,
              poolVout: 0,
              poolTokenVout: 1,
              poolToken2Vout: 2,
              lpGenesisTxid: execRes.newPoolHead.lpGenesisTxid,
              lpGenesisVout: execRes.newPoolHead.lpGenesisVout,
              r1: execRes.reservesAfter.r1.toString(),
              r2: execRes.reservesAfter.r2.toString(),
              lp: execRes.reservesAfter.lp.toString(),
            }
        const normalizedBroadcasted =
          execRes.broadcasted !== undefined ? Boolean(execRes.broadcasted) : broadcast
        const normalizedExecSizeBytes =
          execRes.execSizeBytes !== undefined
            ? execRes.execSizeBytes
            : Number(execRes.details?.execBytes || (execRes.execRawHex ? execRes.execRawHex.length / 2 : 0))
        const normalizedExecFee =
          execRes.execFee !== undefined ? execRes.execFee : Number(execRes.details?.execFee || 0)

        // Advance pool head & local 0-conf UTXO cache immediately before releasing the pool lock!
        const rawTxs: Array<{ txid: string; rawHex: string }> = [
          { txid: execRes.prepTxid, rawHex: execRes.prepRawHex },
        ]
        if (execRes.tokenDepositTxid && execRes.tokenDepositRawHex) {
          rawTxs.push({ txid: execRes.tokenDepositTxid, rawHex: execRes.tokenDepositRawHex })
        }
        if (execRes.tokenDeposit2Txid && execRes.tokenDeposit2RawHex) {
          rawTxs.push({ txid: execRes.tokenDeposit2Txid, rawHex: execRes.tokenDeposit2RawHex })
        }
        rawTxs.push({ txid: execRes.execTxid, rawHex: execRes.execRawHex })

        if (normalizedBroadcasted) {
          const updatedState = await this.store.advancePoolHead(
            params.poolId,
            {
              poolTxid: normalizedNewPoolState.poolTxid,
              lpGenesisTxid: normalizedNewPoolState.lpGenesisTxid,
              lpGenesisVout: normalizedNewPoolState.lpGenesisVout,
            },
            rawTxs
          )
          if (updatedState) {
            if (updatedState.poolToken1Vout !== undefined) {
              normalizedNewPoolState.poolTokenVout = updatedState.poolToken1Vout
              normalizedNewPoolState.poolToken2Vout = updatedState.poolToken2Vout
            } else if (updatedState.poolTokenVout !== undefined) {
              normalizedNewPoolState.poolTokenVout = updatedState.poolTokenVout
            }
          }
          this.store.recordTx({
            txid: execRes.execTxid,
            prepTxid: execRes.prepTxid,
            depositTxid: execRes.tokenDepositTxid,
            ...(execRes.tokenDeposit2Txid ? { deposit2Txid: execRes.tokenDeposit2Txid } : {}),
            poolId: params.poolId,
            operation: params.operation,
            userAddress: params.userAddress,
            status: 'broadcasted',
            details: execRes.details,
            reservesAfter: {
              r1: normalizedNewPoolState.r1,
              r2: normalizedNewPoolState.r2,
              lp: normalizedNewPoolState.lp,
            },
            createdAt: new Date().toISOString(),
          })
        }

        session.status = 'completed'
        cleanupLock()
        emitEvent({
          sessionId,
          poolId: params.poolId,
          operation: params.operation,
          status: 'completed',
          stepIndex: session.stepIndex,
          quote: session.quote,
          result: {
            prepTxid: execRes.prepTxid,
            depositTxid: execRes.tokenDepositTxid,
            ...(execRes.tokenDeposit2Txid ? { deposit2Txid: execRes.tokenDeposit2Txid } : {}),
            execTxid: execRes.execTxid,
            execSizeBytes: normalizedExecSizeBytes,
            execFee: normalizedExecFee,
            details: execRes.details,
            newPoolState: normalizedNewPoolState,
            broadcasted: normalizedBroadcasted,
          },
        })
      } catch (err: any) {
        const msg = err && err.message ? err.message : String(err)
        if (session.status !== 'cancelled') session.status = 'failed'
        cleanupLock()
        emitEvent({
          sessionId,
          poolId: params.poolId,
          operation: params.operation,
          status: session.status,
          stepIndex: session.stepIndex,
          quote: session.quote,
          error: msg,
        })
      }
    })()

    return firstEventPromise
  }

  async submitSignature(
    sessionId: string,
    sigPayload: { signature: { publicKey: string; r: string; s: string; sig: string; sigtype: number; txid: string } }
  ): Promise<SessionStepResponse> {
    const session = this.sessions.get(sessionId)
    if (!session) throw new Error(`Session not found: ${sessionId}`)
    if (session.status !== 'awaiting_signature' || !session.pendingSignResolve) {
      throw new Error(`Session ${sessionId} is not awaiting a signature (status=${session.status})`)
    }
    const resolveSign = session.pendingSignResolve
    session.pendingSignResolve = undefined
    session.pendingSignReject = undefined

    const nextEventPromise = new Promise<SessionStepResponse>((resolve) => {
      session.onNextEvent = resolve
    })
    resolveSign(sigPayload)
    return nextEventPromise
  }

  async cancelSession(sessionId: string): Promise<SessionStepResponse> {
    const session = this.sessions.get(sessionId)
    if (!session) throw new Error(`Session not found: ${sessionId}`)
    session.status = 'cancelled'
    if (session.pendingSignReject) {
      const rejectSign = session.pendingSignReject
      session.pendingSignResolve = undefined
      session.pendingSignReject = undefined
      const nextEv = new Promise<SessionStepResponse>((resolve) => {
        session.onNextEvent = resolve
      })
      rejectSign(new Error('Cancelled by user'))
      return nextEv
    }
    return {
      sessionId,
      poolId: session.poolId,
      operation: session.operation,
      status: 'cancelled',
      stepIndex: session.stepIndex,
    }
  }
}
