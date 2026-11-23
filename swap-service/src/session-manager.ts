import * as crypto from 'crypto'
import { PoolStore } from './pool-store'
import { SwapEngine } from './core-bridge'
import {
  quoteAddLiquidity,
  quoteRemoveLiquidity,
  quoteSwapMvcToToken,
  quoteSwapTokenToMvc,
} from './quote'

export type SwapOperation = 'swapMvcToToken' | 'swapTokenToMvc' | 'addLiquidity' | 'removeLiquidity'

export interface StartSessionParams {
  poolId: string
  operation: SwapOperation
  userAddress: string
  amountIn?: string        // sats for swapMvcToToken, raw token units for swapTokenToMvc
  slippageBps?: number
  minAmountOut?: string
  mvcAddAmount?: string    // sats for addLiquidity
  tokenAddAmount?: string  // optional raw token units for addLiquidity
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
    execTxid: string
    execSizeBytes: number
    execFee: number
    details: Record<string, string | number>
    newPoolState: {
      poolTxid: string
      poolVout: number
      poolTokenVout: number
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

        let execRes: any
        if (params.operation === 'swapMvcToToken') {
          if (!params.amountIn) throw new Error('amountIn (sats) is required for swapMvcToToken')
          const amountIn = BigInt(params.amountIn)
          const q = quoteSwapMvcToToken(poolState, amountIn, slippageBps)
          session.quote = q
          const minOut = params.minAmountOut ? BigInt(params.minAmountOut) : BigInt(q.minAmountOut)
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
        } else if (params.operation === 'swapTokenToMvc') {
          if (!params.amountIn) throw new Error('amountIn (raw token units) is required for swapTokenToMvc')
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
        } else if (params.operation === 'addLiquidity') {
          if (!params.mvcAddAmount) throw new Error('mvcAddAmount (sats) is required for addLiquidity')
          const mvcAdd = BigInt(params.mvcAddAmount)
          const customTok = params.tokenAddAmount ? BigInt(params.tokenAddAmount) : undefined
          const q = quoteAddLiquidity(poolState, mvcAdd, customTok)
          session.quote = q
          const reqTok = BigInt(q.tokenAddAmount)
          const tokUtxos = await this.store.getFreshTokenUtxos(
            params.userAddress,
            entry.token2.codeHash,
            entry.token2.tokenID
          )
          const chosen = tokUtxos.find((u: any) => u.amount >= reqTok)
          if (!chosen) {
            throw new Error(`No single unspent ${entry.token2.symbol} UTXO >= ${reqTok}.`)
          }
          execRes = await SwapEngine.executeAddLiquidity({
            contracts: this.store.contracts,
            chain: this.store.chain,
            signer: interactiveSigner,
            poolConfig: entry.contract,
            poolState,
            userTokenUtxo: chosen,
            mvcAddAmount: mvcAdd,
            tokenAddAmount: reqTok,
            spaceUtxos,
            feeRateSatPerByte: feeRate,
            broadcast,
          })
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
        } else {
          throw new Error(`Unsupported operation: ${params.operation}`)
        }

        // Advance pool head & local 0-conf UTXO cache immediately before releasing the pool lock!
        const rawTxs: Array<{ txid: string; rawHex: string }> = [
          { txid: execRes.prepTxid, rawHex: execRes.prepRawHex },
        ]
        if (execRes.tokenDepositTxid && execRes.tokenDepositRawHex) {
          rawTxs.push({ txid: execRes.tokenDepositTxid, rawHex: execRes.tokenDepositRawHex })
        }
        rawTxs.push({ txid: execRes.execTxid, rawHex: execRes.execRawHex })

        if (execRes.broadcasted) {
          await this.store.advancePoolHead(
            params.poolId,
            {
              poolTxid: execRes.newPoolState.poolTxid,
              lpGenesisTxid: execRes.newPoolState.lpGenesisTxid,
              lpGenesisVout: execRes.newPoolState.lpGenesisVout,
            },
            rawTxs
          )
          this.store.recordTx({
            txid: execRes.execTxid,
            prepTxid: execRes.prepTxid,
            depositTxid: execRes.tokenDepositTxid,
            poolId: params.poolId,
            operation: params.operation,
            userAddress: params.userAddress,
            status: 'broadcasted',
            details: execRes.details,
            reservesAfter: {
              r1: execRes.newPoolState.r1,
              r2: execRes.newPoolState.r2,
              lp: execRes.newPoolState.lp,
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
            execTxid: execRes.execTxid,
            execSizeBytes: execRes.execSizeBytes,
            execFee: execRes.execFee,
            details: execRes.details,
            newPoolState: execRes.newPoolState,
            broadcasted: execRes.broadcasted,
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
