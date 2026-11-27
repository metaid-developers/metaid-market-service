/**
 * Floor-bid placement: price math + OrderTokenV2 draft validation + signing.
 * Imports the orderbook sidecar's tx-builder in-process (P1a fundingAddress
 * split) — the keeper never exposes key material to any service.
 */
import {
  OrderbookTxBuilder,
  DraftBuildResult,
  FinalizeResult,
  SignRequestItem,
  UserSignatureInput,
} from '../../orderbook-service/src/tx-builder'
import { OrderTokenProto, TokenProto, scrypt } from '../../orderbook-service/src/core-bridge'

const { mvc, signTx, toHex } = scrypt as any

export const WBTC_CODEHASH = 'c9cc7bbd1010b44873959a8b1a2bcedeb62302b7'
export const WBTC_TOKEN_ID = '2ee1af1ae2d3605f08894c17358c99f1fc4c1325'
export const BURN_MVC_ADDRESS = '1111111111111111111114oLvT2'

/** deterministic per-batch salt: first 8 bytes of sha256('idcoin-floor:'+batchId).
 *  Makes the order tail (and therefore the order script + on-chain identity)
 *  unique per batch and recognizable after a crash. */
export function batchSaltHex(batchId: number): string {
  const h = require('crypto').createHash('sha256').update(`idcoin-floor:${batchId}`).digest()
  return h.subarray(0, 8).toString('hex')
}

// ---------------------------------------------------------------------------
// price math
// ---------------------------------------------------------------------------

export function gcdBigInt(a: bigint, b: bigint): bigint {
  while (b) {
    const t = a % b
    a = b
    b = t
  }
  return a
}

const UINT64_MAX = (1n << 64n) - 1n

/**
 * priceNum/priceDen = amtPerMint·10^decimals·10000 / (liquidityPerMint·(10000−haircutBps))
 * reduced by gcd. With the default 150 bps haircut this is the spec's
 * amtPerMint·10^decimals·1000 / (liquidityPerMint·985).
 * Throws when the reduced terms do not fit uint64.
 */
export function computeFloorPrice(args: {
  amtPerMint: bigint
  decimals: number
  liquidityPerMint: bigint
  haircutBps: number
}): { priceNum: bigint; priceDen: bigint } {
  if (args.liquidityPerMint <= 0n) throw new Error('liquidityPerMint must be positive')
  if (args.amtPerMint <= 0n) throw new Error('amtPerMint must be positive')
  if (args.haircutBps <= 0 || args.haircutBps >= 10000) throw new Error('haircutBps out of range')
  let num = args.amtPerMint * 10n ** BigInt(args.decimals) * 10000n
  let den = args.liquidityPerMint * BigInt(10000 - args.haircutBps)
  const g = gcdBigInt(num, den)
  num /= g
  den /= g
  if (num > UINT64_MAX || den > UINT64_MAX) {
    throw new Error(`reduced price ${num}/${den} does not fit uint64`)
  }
  return { priceNum: num, priceDen: den }
}

// ---------------------------------------------------------------------------
// draft validation
// ---------------------------------------------------------------------------

export interface BidExpectation {
  fundingAddress: string
  sellTokenCodeHash: string
  sellTokenID: string
  buyTokenCodeHash: string
  buyTokenID: string
  sellAmount: bigint
  priceNum: bigint
  priceDen: bigint
  minFillAmount: bigint
}

function hash160OfMvcAddress(addr: string): Buffer {
  const a = new mvc.Address(addr)
  // mvc-lib exposes the hash160 through the address object internals
  const obj = a.toObject()
  return Buffer.from(obj.hash, 'hex')
}

/**
 * Decode every transaction referenced by the draft's sign requests and assert:
 *  - every sign request is keyed to the funding address
 *  - every tx output is one of:
 *      (a) plain P2PKH to the funding address (SPACE change / fee-utxo)
 *      (b) an OrderTokenV2 script whose tail exactly matches expectations
 *          (burn maker, correct tokens, price, remain, zero fee, NULL feeAddr)
 *      (c) a contract/token script carrying the funding hash160 (token change
 *          receiver) — these are constructed by the local builder from the
 *          validated parameters
 *  Any foreign P2PKH output or tail mismatch rejects the draft BEFORE signing.
 */
export interface BidValidation {
  ok: boolean
  reason?: string
  /** order contract scripts validated in these txs (step 0 only) */
  orderScripts: Buffer[]
}

export function validateBidDraft(draft: DraftBuildResult, exp: BidExpectation, fingerprints: Buffer[] = []): BidValidation {
  return validateSignRequestsTxs(draft.signRequests, exp, fingerprints, [], true)
}

/** Validate every transaction referenced by a set of sign requests (used for
 *  BOTH the step-0 prep tx and the step-1 lock tx — each is validated BEFORE
 *  its signatures are produced). `orderTokenLocks` whitelists token outputs
 *  locked to hash160(order script) with exactly sellAmount — these appear in
 *  the step-1 lock tx and nowhere else. */
export function validateSignRequestsTxs(
  reqs: SignRequestItem[],
  exp: BidExpectation,
  fingerprints: Buffer[] = [],
  orderTokenLocks: Buffer[] = [],
  requireOrder = false
): BidValidation {
  const fundingHash = hash160OfMvcAddress(exp.fundingAddress)
  const zeroHash = Buffer.alloc(20, 0)
  const fundingP2pkh = Buffer.concat([Buffer.from('76a914', 'hex'), fundingHash, Buffer.from('88ac', 'hex')])
  const orderScripts: Buffer[] = []

  for (const req of reqs) {
    if (req.address !== exp.fundingAddress) {
      return { ok: false, reason: `sign request keyed to ${req.address}, not funding ${exp.fundingAddress}`, orderScripts }
    }
  }

  const seen = new Set<string>()
  let orderValidated = false
  for (const req of reqs) {
    if (seen.has(req.txHex)) continue
    seen.add(req.txHex)
    let tx: any
    try {
      tx = new mvc.Transaction(req.txHex)
    } catch (e: any) {
      return { ok: false, reason: `draft tx does not decode: ${e.message}`, orderScripts }
    }
    for (const out of tx.outputs) {
      const scriptBuf = out.script.toBuffer()
      // (a) plain P2PKH — must be funding exactly
      if (scriptBuf.length === 25 && scriptBuf[0] === 0x76 && scriptBuf[1] === 0xa9) {
        if (!scriptBuf.equals(fundingP2pkh)) {
          return { ok: false, reason: 'foreign P2PKH output in draft tx', orderScripts }
        }
        continue
      }
      // (b) order contract — strict tail validation
      let tail: any = null
      try {
        tail = OrderTokenProto.parseOrderTokenScript(scriptBuf)
      } catch {
        tail = null
      }
      if (tail) {
        if (!Buffer.from(tail.makerAddr).equals(zeroHash)) {
          return { ok: false, reason: 'order tail makerAddr is not the zero/burn hash', orderScripts }
        }
        if (Buffer.from(tail.sellTokenCodeHash).toString('hex') !== exp.sellTokenCodeHash ||
            Buffer.from(tail.sellTokenID).toString('hex') !== exp.sellTokenID) {
          return { ok: false, reason: 'order tail sellToken mismatch', orderScripts }
        }
        if (Buffer.from(tail.buyTokenCodeHash).toString('hex') !== exp.buyTokenCodeHash ||
            Buffer.from(tail.buyTokenID).toString('hex') !== exp.buyTokenID) {
          return { ok: false, reason: 'order tail buyToken mismatch', orderScripts }
        }
        if (BigInt(tail.priceNum) !== exp.priceNum || BigInt(tail.priceDen) !== exp.priceDen) {
          return { ok: false, reason: `order tail price ${tail.priceNum}/${tail.priceDen} != expected ${exp.priceNum}/${exp.priceDen}`, orderScripts }
        }
        if (BigInt(tail.remainAmount) !== exp.sellAmount) {
          return { ok: false, reason: `order tail remainAmount ${tail.remainAmount} != sellAmount ${exp.sellAmount}`, orderScripts }
        }
        if (BigInt(tail.minFillAmount) !== exp.minFillAmount) {
          return { ok: false, reason: 'order tail minFillAmount mismatch', orderScripts }
        }
        if (Number(tail.feeBps) !== 0) {
          return { ok: false, reason: 'order tail feeBps != 0', orderScripts }
        }
        if (!Buffer.from(tail.feeAddr).equals(zeroHash)) {
          return { ok: false, reason: 'order tail feeAddr is not NULL', orderScripts }
        }
        orderValidated = true
        orderScripts.push(scriptBuf)
        continue
      }
      // (c) MCP02 token outputs — receiver hash160 must be funding (change) or
      // a whitelisted order-script hash carrying exactly sellAmount (step-1 lock)
      try {
        const recv = Buffer.from(TokenProto.getTokenAddress(scriptBuf))
        if (recv.length === 20 && recv.equals(fundingHash)) {
          continue
        }
        if (recv.length === 20 && orderTokenLocks.some((h) => h.equals(recv))) {
          if (TokenProto.getTokenAmount(scriptBuf) === exp.sellAmount) {
            continue
          }
          return { ok: false, reason: 'token output locked to order hash carries wrong amount', orderScripts }
        }
      } catch {
        // not parseable as a token script — fall through to fingerprint check
      }
      // (d) transfer-check contract instances — must match a compiled contract
      // template fingerprint (they carry no direct receiver; the contract
      // logic constrains them to the validated order's token movement)
      if (fingerprints.some((fp) => includesBuffer(scriptBuf, fp))) {
        continue
      }
      return { ok: false, reason: 'contract/token output is neither funding-locked nor a known contract — possibly foreign', orderScripts }
    }
  }
  if (requireOrder && !orderValidated) {
    return { ok: false, reason: 'no validatable order output found in draft', orderScripts }
  }
  return { ok: true, orderScripts }
}

function includesBuffer(haystack: Buffer, needle: Buffer): boolean {
  return haystack.includes(needle)
}

// ---------------------------------------------------------------------------
// signing / placement
// ---------------------------------------------------------------------------

export function signRequestsWithMvcKey(reqs: SignRequestItem[], privKey: any): UserSignatureInput[] {
  const pubKeyHex = privKey.publicKey.toBuffer().toString('hex')
  return reqs.map((r) => {
    const tx = new mvc.Transaction(r.txHex)
    const sigObj = signTx(tx, privKey, mvc.Script.fromHex(r.scriptHex), r.satoshis, r.inputIndex, r.sigtype)
    const sigHex = typeof sigObj === 'string' ? sigObj : toHex(sigObj)
    return { txIndex: r.txIndex, inputIndex: r.inputIndex, sig: sigHex, publicKey: pubKeyHex }
  })
}

/** Build the place-token params for a floor bid. */
export function floorBidParams(args: {
  fundingAddress: string
  buyTokenCodeHash: string
  buyTokenID: string
  sellAmountWbtc: bigint
  priceNum: bigint
  priceDen: bigint
  saltHex?: string
}) {
  const minFill = args.sellAmountWbtc < 1000n ? args.sellAmountWbtc : 1000n
  return {
    makerAddress: BURN_MVC_ADDRESS,
    fundingAddress: args.fundingAddress,
    sellTokenCodeHash: WBTC_CODEHASH,
    sellTokenID: WBTC_TOKEN_ID,
    buyTokenCodeHash: args.buyTokenCodeHash,
    buyTokenID: args.buyTokenID,
    sellAmount: args.sellAmountWbtc.toString(),
    priceNum: args.priceNum.toString(),
    priceDen: args.priceDen.toString(),
    minFillAmount: minFill.toString(),
    feeBps: 0,
    feeAddress: '',
    ...(args.saltHex ? { saltHex: args.saltHex } : {}),
  }
}

export interface SignedBidTxs {
  prepHex: string
  prepTxid: string
  lockHex: string
  lockTxid: string
  orderTxid: string     // == prepTxid (the order contract output is prep vout 0)
  orderVout: number
  orderScriptHex: string
  remainAmount: string
  draftId: string
  completed: boolean
}

/**
 * Validate-then-sign-then-finalize loop — NEVER broadcasts: both steps are
 * finalized with broadcast=false so the caller obtains the fully signed prep
 * and lock raw hexes + txids, persists them, and broadcasts itself. Every
 * step's transactions are validated BEFORE that step's signatures are
 * produced; validation failure throws and nothing is signed.
 */
export async function placeFloorBid(args: {
  builder: OrderbookTxBuilder
  privKey: any
  params: ReturnType<typeof floorBidParams>
  expectation: BidExpectation
  contractFingerprints?: Buffer[]
}): Promise<SignedBidTxs> {
  const draft: DraftBuildResult = await buildPlace(args.builder, args.params)
  const fps = args.contractFingerprints || []
  const check = validateBidDraft(draft, args.expectation, fps)
  if (!check.ok) {
    throw new Error(`bid draft invariant violation: ${check.reason}`)
  }
  const orderTokenLocks = check.orderScripts.map((s) =>
    Buffer.from(mvc.crypto.Hash.sha256ripemd160(s))
  )
  let reqs = draft.signRequests
  let lastRes: FinalizeResult | null = null
  for (let guard = 0; guard < 5; guard++) {
    const sigs = signRequestsWithMvcKey(reqs, args.privKey)
    const res = await args.builder.finalizeAndBroadcast(draft.draftId, sigs, false)
    lastRes = res
    if (res.status === 'completed') {
      const order = (res.newOrders || [])[0] || {}
      return {
        prepHex: res.txHexes[0],
        prepTxid: res.txids[0],
        lockHex: res.txHexes[1],
        lockTxid: res.txids[1],
        orderTxid: (order as any).txid || res.txids[0],
        orderVout: (order as any).vout ?? 0,
        orderScriptHex: (order as any).scriptHex || '',
        remainAmount: (order as any).remainAmount || '0',
        draftId: draft.draftId,
        completed: true,
      }
    }
    reqs = res.signRequests || []
    if (reqs.length === 0) break
    // validate the next step's transactions BEFORE signing them
    const stepCheck = validateSignRequestsTxs(reqs, args.expectation, fps, orderTokenLocks, false)
    if (!stepCheck.ok) {
      throw new Error(`bid draft invariant violation (later step): ${stepCheck.reason}`)
    }
  }
  return {
    prepHex: '', prepTxid: '', lockHex: '', lockTxid: '',
    orderTxid: '', orderVout: 0, orderScriptHex: '', remainAmount: '0',
    draftId: draft.draftId, completed: false,
  }
}

async function buildPlace(builder: OrderbookTxBuilder, params: any): Promise<DraftBuildResult> {
  // tx-builder exposes buildPlaceTokenOrder as a public method (P1a)
  return (builder as any).buildPlaceTokenOrder(params)
}

/** Distinctive middle chunks of compiled contract templates, used to recognize
 *  builder-constructed transfer-check outputs in draft validation. */
export function computeContractFingerprints(contracts: any): Buffer[] {
  const out: Buffer[] = []
  for (const key of ['TokenTransferCheck']) {
    try {
      const script = new contracts[key]().lockingScript.toBuffer()
      if (script.length > 64) out.push(Buffer.from(script.subarray(24, 56)))
    } catch {
      // template unavailable — fingerprint list stays shorter, validation
      // then rejects transfer-check outputs outright (fail-safe)
    }
  }
  return out
}
