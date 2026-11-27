import * as crypto from 'crypto'
import {
  Common,
  LoadedOrderbookContracts,
  OrderProto,
  OrderTokenProto,
  PROD_TRANSFER_CHECK_HASHES,
  PROD_UNLOCK_CHECK_HASHES,
  scrypt,
  SwapEngine,
  TokenProto,
} from './core-bridge'
import { ChainProvider, MvcUtxo, TokenUtxo } from './chain-provider'

const { mvc, getPreimage, toHex, SigHashPreimage, Bytes, PubKey, Sig, Ripemd160 } = scrypt

export const SIG_ALL = mvc.crypto.Signature.SIGHASH_ALL | mvc.crypto.Signature.SIGHASH_FORKID
export const DUST = OrderProto.DUST_LIMIT // 546
export const ZERO_PUBKEY_HEX = Buffer.alloc(33, 0).toString('hex')
export const ZERO_SIG_HEX = Buffer.alloc(72, 0).toString('hex')

export interface SignRequestItem {
  txIndex: number
  inputIndex: number
  txHex: string
  scriptHex: string
  satoshis: number
  sigtype: number
  address: string
  kind: 'p2pkh' | 'token_p2pkh' | 'order_cancel'
  message: string
}

export interface UserSignatureInput {
  txIndex: number
  inputIndex: number
  sig: string       // DER + sighash byte hex
  publicKey: string // 33-byte compressed pubkey hex
}

export interface DraftBuildResult {
  draftId: string
  action: 'place_ask' | 'place_bid' | 'place_token' | 'take_ask' | 'take_bid' | 'take_token' | 'cancel_ask' | 'cancel_bid' | 'cancel_token'
  status: 'awaiting_signature'
  step: number
  totalSteps: number
  signRequests: SignRequestItem[]
  summary: Record<string, any>
  expiresAt: number
}

export interface NewOrderOutputInfo {
  side: 'ask' | 'bid' | 'token'
  txid: string
  vout: number
  satoshis: number
  scriptHex: string
  remainAmount: string
  tokenTxid?: string
  tokenVout?: number
  saltHex: string
}

export interface FinalizeResult {
  draftId: string
  action: string
  status: 'awaiting_signature' | 'completed'
  step?: number
  totalSteps?: number
  signRequests?: SignRequestItem[]
  txids: string[]
  txHexes: string[]
  verified: boolean
  broadcasted: boolean
  newOrders: NewOrderOutputInfo[]
}

export interface PlaceAskParams {
  makerAddress: string
  tokenCodeHash: string
  tokenID: string
  tokenAmount: string      // uint64 token raw units
  priceNum: string         // uint64 SPACE sats
  priceDen: string         // uint64 token raw units
  minFillAmount: string    // uint64 token raw units
  feeBps: number
  feeAddress?: string      // optional MVC address; empty -> NULL_ADDRESS
  saltHex?: string         // optional 8-byte hex
  feeRate?: number
}

export interface PlaceBidParams {
  makerAddress: string
  tokenCodeHash: string
  tokenID: string
  spendSats: string        // uint64 SPACE sats locked in OrderBidV2
  priceNum: string
  priceDen: string
  minFillAmount: string
  feeBps: number
  feeAddress?: string
  saltHex?: string
  feeRate?: number
}

export interface TakeAskLeg {
  orderTxid: string
  orderVout: number
  tokenTxid: string
  tokenVout: number
  fillToken: string
}

export interface TakeAskParams {
  takerAddress: string
  legs: TakeAskLeg[]
  feeRate?: number
}

export interface TakeBidLeg {
  orderTxid: string
  orderVout: number
  fillToken: string
}

export interface TakeBidParams {
  takerAddress: string
  tokenCodeHash: string
  tokenID: string
  legs: TakeBidLeg[]
  feeRate?: number
}

export interface CancelOrderParams {
  makerAddress: string
  side: 'ask' | 'bid' | 'token'
  orderTxid: string
  orderVout: number
  tokenTxid?: string
  tokenVout?: number
  feeRate?: number
}

export interface PlaceTokenOrderParams {
  makerAddress: string
  sellTokenCodeHash: string
  sellTokenID: string
  buyTokenCodeHash: string
  buyTokenID: string
  sellAmount: string       // uint64 sellToken raw units locked in OrderTokenV2
  priceNum: string         // uint64 buyToken raw units
  priceDen: string         // uint64 sellToken raw units
  minFillAmount: string    // uint64 sellToken raw units
  feeBps: number
  feeAddress?: string
  saltHex?: string
  feeRate?: number
}

export interface TakeTokenOrderLeg {
  orderTxid: string
  orderVout: number
  tokenTxid: string        // locked sellToken UTXO txid
  tokenVout: number        // locked sellToken UTXO vout
  fillSellAmount: string   // uint64 sellToken raw units taker buys
}

export interface TakeTokenOrderParams {
  takerAddress: string
  buyTokenCodeHash: string
  buyTokenID: string
  legs: TakeTokenOrderLeg[]
  feeRate?: number
}

interface InternalDraft {
  id: string
  action: DraftBuildResult['action']
  createdAt: number
  expiresAt: number
  step: number
  totalSteps: number
  advance: (sigs: UserSignatureInput[], broadcast: boolean) => Promise<FinalizeResult>
}

function addressToHash20(address: string, net: 'livenet' | 'testnet'): Buffer {
  if (!address) return Buffer.alloc(20, 0)
  return Buffer.from(new mvc.Address(address, net).hashBuffer)
}

function p2pkhScript(address: string, net: 'livenet' | 'testnet'): any {
  return mvc.Script.buildPublicKeyHashOut(new mvc.Address(address, net))
}

function p2pkhScriptFromHash20(hash20: Buffer): any {
  return mvc.Script.fromBuffer(
    Buffer.concat([Buffer.from([0x76, 0xa9, 0x14]), hash20, Buffer.from([0x88, 0xac])])
  )
}

/**
 * Returns the canonical MVC v10 txid (double-SHA256 of v10 txHeader, reversed).
 * Matches `TxUtil.verifyTxOutput` inside sCrypt contracts and Higun/MVC node txids.
 */
export function getMvcV10TxId(tx: any): string {
  const info = Common.getTxidInfo(tx)
  return mvc.crypto.Hash.sha256sha256(Buffer.from(info.txHeader, 'hex')).reverse().toString('hex')
}

function buildPrevouts(tx: any): Buffer {
  let buf = Buffer.alloc(0)
  for (const inp of tx.inputs) {
    buf = Buffer.concat([buf, Common.getTxIdBuf(inp.prevTxId.toString('hex')), Common.getUInt32Buf(inp.outputIndex)])
  }
  return buf
}

function buildOutputsRange(tx: any, startIdx: number, endIdx: number): Buffer {
  let buf = Buffer.alloc(0)
  for (let i = startIdx; i < endIdx; i++) {
    buf = Buffer.concat([buf, Common.buildOutput(tx.outputs[i].script.toBuffer(), tx.outputs[i].satoshis)])
  }
  return buf
}

function buildRestOutputs(tx: any, fromIdx: number): Buffer {
  return buildOutputsRange(tx, fromIdx, tx.outputs.length)
}

function addTxInput(tx: any, txid: string, vout: number, lockingScript: any, satoshis: number): void {
  tx.addInput(
    new mvc.Transaction.Input({
      output: new mvc.Transaction.Output({ script: lockingScript, satoshis }),
      prevTxId: txid,
      outputIndex: vout,
      script: mvc.Script.empty(),
    })
  )
}

function selectMvcUtxos(utxos: MvcUtxo[], requiredSats: number): { selected: MvcUtxo[]; total: number } {
  const sorted = [...utxos].sort((a, b) => b.satoshis - a.satoshis)
  const selected: MvcUtxo[] = []
  let total = 0
  for (const u of sorted) {
    selected.push(u)
    total += u.satoshis
    if (total >= requiredSats) break
  }
  if (total < requiredSats) {
    throw new Error(`Insufficient SPACE balance: need ${requiredSats} sats, have ${total} sats`)
  }
  return { selected, total }
}

function selectTokenUtxos(utxos: TokenUtxo[], requiredAmount: bigint, maxInputs = 3): { selected: TokenUtxo[]; total: bigint } {
  const sorted = [...utxos].sort((a, b) => (b.tokenAmount > a.tokenAmount ? 1 : b.tokenAmount < a.tokenAmount ? -1 : 0))
  const selected: TokenUtxo[] = []
  let total = BigInt(0)
  for (const u of sorted) {
    selected.push(u)
    total += u.tokenAmount
    if (total >= requiredAmount) break
    if (selected.length >= maxInputs) break
  }
  if (total < requiredAmount) {
    throw new Error(`Insufficient token balance (in <=${maxInputs} UTXOs): need ${requiredAmount}, have ${total}`)
  }
  return { selected, total }
}

function getSigOrThrow(sigMap: Map<string, UserSignatureInput>, txIndex: number, inputIndex: number): UserSignatureInput {
  const s = sigMap.get(`${txIndex}:${inputIndex}`)
  if (!s || !s.sig || !s.publicKey) {
    throw new Error(`Missing signature for txIndex=${txIndex}, inputIndex=${inputIndex}`)
  }
  return s
}

function applyP2PKHSig(tx: any, inputIndex: number, sig: UserSignatureInput): void {
  const unlock = mvc.Script.empty()
    .add(Buffer.from(sig.sig, 'hex'))
    .add(Buffer.from(sig.publicKey, 'hex'))
  tx.inputs[inputIndex].setScript(unlock)
}

export class OrderbookTxBuilder {
  private readonly contracts: LoadedOrderbookContracts
  private readonly chain: ChainProvider
  private readonly net: 'livenet' | 'testnet'
  private readonly defaultFeeRate: number
  private readonly draftTtlMs: number
  private readonly transferCheckHashes: any[]
  private readonly unlockCheckHashes: any[]
  private readonly drafts = new Map<string, InternalDraft>()

  constructor(opts: {
    contracts: LoadedOrderbookContracts
    chain: ChainProvider
    net: 'livenet' | 'testnet'
    feeRateSatPerByte?: number
    draftTtlMs?: number
    transferCheckHashesHex?: string[]
    unlockCheckHashesHex?: string[]
  }) {
    this.contracts = opts.contracts
    this.chain = opts.chain
    this.net = opts.net
    this.defaultFeeRate = opts.feeRateSatPerByte ?? 1.25
    this.draftTtlMs = opts.draftTtlMs ?? 120000
    const tcHex = opts.transferCheckHashesHex || PROD_TRANSFER_CHECK_HASHES
    const ucHex = opts.unlockCheckHashesHex || PROD_UNLOCK_CHECK_HASHES
    this.transferCheckHashes = tcHex.map((h) => new Bytes(h))
    this.unlockCheckHashes = ucHex.map((h) => new Bytes(h))
  }

  getContractMeta() {
    return {
      askCodeHash: this.contracts.askCodeHashHex,
      bidCodeHash: this.contracts.bidCodeHashHex,
      tokenOrderCodeHash: this.contracts.tokenOrderCodeHashHex,
      tailLen: OrderProto.DATA_LEN,
      tokenTailLen: OrderTokenProto.DATA_LEN,
      net: this.net,
    }
  }

  private pruneDrafts(): void {
    const now = Date.now()
    for (const [id, d] of this.drafts.entries()) {
      if (d.expiresAt <= now) this.drafts.delete(id)
    }
  }

  private saveDraft(
    action: DraftBuildResult['action'],
    totalSteps: number,
    signRequests: SignRequestItem[],
    summary: Record<string, any>,
    advance: InternalDraft['advance']
  ): DraftBuildResult {
    this.pruneDrafts()
    const id = crypto.randomBytes(16).toString('hex')
    const now = Date.now()
    const expiresAt = now + this.draftTtlMs
    this.drafts.set(id, { id, action, createdAt: now, expiresAt, step: 0, totalSteps, advance })
    return {
      draftId: id,
      action,
      status: 'awaiting_signature',
      step: 0,
      totalSteps,
      signRequests,
      summary,
      expiresAt,
    }
  }

  async finalizeAndBroadcast(draftId: string, signatures: UserSignatureInput[], broadcast = true): Promise<FinalizeResult> {
    this.pruneDrafts()
    const draft = this.drafts.get(draftId)
    if (!draft) {
      throw new Error(`Orderbook draft ${draftId} not found or expired`)
    }
    const res = await draft.advance(signatures, broadcast)
    res.draftId = draftId
    if (res.status === 'completed') {
      this.drafts.delete(draftId)
    } else {
      draft.expiresAt = Date.now() + this.draftTtlMs
    }
    return res
  }

  private instantiateToken(chainScriptBuf: Buffer): { inst: any; exactMatch: boolean } {
    const rawDataLen = 170 // TokenProto raw payload excluding 2-byte OP_PUSHDATA1 prefix
    const dataHex = chainScriptBuf.subarray(chainScriptBuf.length - rawDataLen).toString('hex')
    const tryMake = (tc: any[], uc: any[]) => {
      const inst = new this.contracts.Token(tc, uc)
      inst.setDataPart(dataHex)
      return inst.lockingScript.toBuffer().equals(chainScriptBuf) ? inst : null
    }
    const direct = tryMake(this.transferCheckHashes, this.unlockCheckHashes)
    if (direct) return { inst: direct, exactMatch: true }
    const prodTc = PROD_TRANSFER_CHECK_HASHES.map((h) => new Bytes(h))
    const prodUc = PROD_UNLOCK_CHECK_HASHES.map((h) => new Bytes(h))
    const prod = tryMake(prodTc, prodUc)
    if (prod) return { inst: prod, exactMatch: true }

    // Support legacy MVC FT v1 contracts (e.g. MC codehash a2421f1e...) that share
    // the exact same Token.unlock() ABI and PROD TransferCheck/UnlockContractCheck hashes.
    const codeHex = TokenProto.getScriptCode(chainScriptBuf).toString('hex')
    if (codeHex.includes(PROD_TRANSFER_CHECK_HASHES[0]) && codeHex.includes(PROD_UNLOCK_CHECK_HASHES[0])) {
      const abiInst = new this.contracts.Token(prodTc, prodUc)
      abiInst.setDataPart(dataHex)
      return { inst: abiInst, exactMatch: false }
    }
    throw new Error('Unable to instantiate Token contract matching chain lockingScript')
  }

  private verifyScriptAgainstChain(
    tx: any,
    inputIndex: number,
    unlockingScript: any,
    lockingScript: any,
    satoshis: number,
    label: string
  ): void {
    const Interpreter = mvc.Script.Interpreter
    const flags =
      Interpreter.SCRIPT_VERIFY_P2SH |
      Interpreter.SCRIPT_VERIFY_STRICTENC |
      Interpreter.SCRIPT_VERIFY_DERSIG |
      Interpreter.SCRIPT_VERIFY_LOW_S |
      Interpreter.SCRIPT_VERIFY_NULLFAIL |
      Interpreter.SCRIPT_VERIFY_MINIMALDATA |
      Interpreter.SCRIPT_VERIFY_DISCOURAGE_UPGRADABLE_NOPS |
      Interpreter.SCRIPT_VERIFY_CHECKLOCKTIMEVERIFY |
      Interpreter.SCRIPT_VERIFY_CHECKSEQUENCEVERIFY |
      Interpreter.SCRIPT_ENABLE_SIGHASH_FORKID |
      Interpreter.SCRIPT_ENABLE_MAGNETIC_OPCODES |
      Interpreter.SCRIPT_ENABLE_MONOLITH_OPCODES
    const interp = new Interpreter()
    const ok = interp.verify(
      unlockingScript,
      lockingScript,
      tx,
      inputIndex,
      flags,
      new mvc.crypto.BN(satoshis)
    )
    if (!ok) {
      throw new Error(`${label} verify failed at input ${inputIndex}: ${interp.errstr}`)
    }
  }

  private async getTxCached(txid: string, localMap?: Map<string, any>): Promise<any> {
    if (localMap) {
      const hit = localMap.get(txid.toLowerCase())
      if (hit) return hit
    }
    const raw = await this.chain.getRawTx(txid)
    const tx = new mvc.Transaction(raw)
    if (localMap) localMap.set(txid.toLowerCase(), tx)
    return tx
  }

  private async resolveTokenBacktrace(tokenTx: any, tokenVout: number, localMap?: Map<string, any>) {
    const curScriptBuf: Buffer = tokenTx.outputs[tokenVout].script.toBuffer()
    const tokenIDHex = TokenProto.getTokenID(curScriptBuf).toString('hex')
    const tokenCodeHashHex = TokenProto.getScriptCodeHash(curScriptBuf).toString('hex')
    const genesisHashHex = TokenProto.getGenesisHash(curScriptBuf).toString('hex')
    const sensibleIDHex = TokenProto.getGenesisTxid(curScriptBuf).toString('hex')

    for (let i = 0; i < tokenTx.inputs.length; i++) {
      const inp = tokenTx.inputs[i]
      const prevTxid = inp.prevTxId.toString('hex')
      const prevVout = inp.outputIndex
      const prevTx = await this.getTxCached(prevTxid, localMap)
      const prevScriptBuf: Buffer = prevTx.outputs[prevVout].script.toBuffer()
      const prevHash160 = mvc.crypto.Hash.sha256ripemd160(prevScriptBuf).toString('hex')
      const sid = Common.genGenesisTxid(prevTxid, prevVout)

      if (prevHash160 === genesisHashHex) {
        return {
          prevTokenInputIndex: i,
          prevTokTx: prevTx,
          prevTokVout: prevVout,
          prevTokenAddress: prevScriptBuf.length > TokenProto.DATA_LEN ? TokenProto.getTokenAddress(prevScriptBuf) : Buffer.alloc(20, 0),
          prevTokenAmount: prevScriptBuf.length > TokenProto.DATA_LEN ? TokenProto.getTokenAmount(prevScriptBuf) : BigInt(0),
        }
      }
      if (sid === sensibleIDHex) {
        return {
          prevTokenInputIndex: i,
          prevTokTx: prevTx,
          prevTokVout: prevVout,
          prevTokenAddress: Buffer.alloc(20, 0),
          prevTokenAmount: BigInt(0),
        }
      }
      if (prevScriptBuf.length > TokenProto.DATA_LEN) {
        try {
          const prevID = TokenProto.getTokenID(prevScriptBuf).toString('hex')
          const prevCH = TokenProto.getScriptCodeHash(prevScriptBuf).toString('hex')
          if (prevID === tokenIDHex && prevCH === tokenCodeHashHex) {
            return {
              prevTokenInputIndex: i,
              prevTokTx: prevTx,
              prevTokVout: prevVout,
              prevTokenAddress: TokenProto.getTokenAddress(prevScriptBuf),
              prevTokenAmount: TokenProto.getTokenAmount(prevScriptBuf),
            }
          }
        } catch (_) {}
      }
    }
    throw new Error(`Unable to resolve token backtrace for ${getMvcV10TxId(tokenTx)}:${tokenVout}`)
  }

  private verifyAndSetTransferCheck(
    tx: any,
    checkInputIdx: number,
    checkInst: any,
    checkSatoshis: number,
    tokenTxs: any[],
    tokenVouts: number[],
    tokenOutSatoshis: number,
    changeSats: number,
    changeAddrHash20: Buffer
  ): void {
    const prevouts = buildPrevouts(tx)
    let inputTokenAddressArray = Buffer.alloc(0)
    let inputTokenAmountArray = Buffer.alloc(0)
    let tokenTxHeaderArray = Buffer.alloc(0)
    let tokenTxHashProofArray = Buffer.alloc(0)
    let tokenSatoshiBytesArray = Buffer.alloc(0)
    let lastTokenScriptBuf = Buffer.alloc(0)

    for (let i = 0; i < tokenTxs.length; i++) {
      const tokTx = tokenTxs[i]
      const vout = tokenVouts[i]
      lastTokenScriptBuf = tokTx.outputs[vout].script.toBuffer()
      inputTokenAddressArray = Buffer.concat([inputTokenAddressArray, TokenProto.getTokenAddress(lastTokenScriptBuf)])
      inputTokenAmountArray = Buffer.concat([
        inputTokenAmountArray,
        Common.getUInt64Buf(TokenProto.getTokenAmount(lastTokenScriptBuf)),
      ])
      const proof = Common.getTxOutputProof(tokTx, vout)
      tokenTxHeaderArray = Buffer.concat([tokenTxHeaderArray, Buffer.from(proof.txHeader.serialize(), 'hex')])
      const hp = Buffer.from(proof.hashProof.serialize(), 'hex')
      tokenTxHashProofArray = Buffer.concat([tokenTxHashProofArray, Common.getUInt32Buf(hp.length), hp])
      tokenSatoshiBytesArray = Buffer.concat([
        tokenSatoshiBytesArray,
        Buffer.from(proof.satoshiBytes.serialize(), 'hex'),
      ])
    }

    const preimage = getPreimage(tx, checkInst.lockingScript.subScript(0), checkSatoshis, checkInputIdx, SIG_ALL)
    const call = checkInst.unlock(
      new SigHashPreimage(toHex(preimage)),
      new Bytes(prevouts.toString('hex')),
      new Bytes(lastTokenScriptBuf.toString('hex')),
      new Bytes(tokenTxHeaderArray.toString('hex')),
      new Bytes(tokenTxHashProofArray.toString('hex')),
      new Bytes(tokenSatoshiBytesArray.toString('hex')),
      new Bytes(inputTokenAddressArray.toString('hex')),
      new Bytes(inputTokenAmountArray.toString('hex')),
      tokenOutSatoshis,
      changeSats,
      new Ripemd160(changeAddrHash20.toString('hex')),
      new Bytes('')
    )
    const v = call.verify({ tx, inputIndex: checkInputIdx, inputSatoshis: checkSatoshis })
    if (!v.success) throw new Error(`TokenTransferCheck verify failed: ${v.error}`)
    tx.inputs[checkInputIdx].setScript(call.unlockingScript)
  }

  private verifyAndSetUnlockCheck(
    tx: any,
    checkInputIdx: number,
    checkInst: any,
    checkSatoshis: number,
    tokenTxs: any[],
    tokenVouts: number[],
    outputTokenIndexes: number[]
  ): void {
    const prevouts = buildPrevouts(tx)
    let inputTokenAddressArray = Buffer.alloc(0)
    let inputTokenAmountArray = Buffer.alloc(0)
    let tokenTxHeaderArray = Buffer.alloc(0)
    let tokenTxHashProofArray = Buffer.alloc(0)
    let tokenSatoshiBytesArray = Buffer.alloc(0)
    let lastTokenScriptBuf = Buffer.alloc(0)

    for (let i = 0; i < tokenTxs.length; i++) {
      const tokTx = tokenTxs[i]
      const vout = tokenVouts[i]
      lastTokenScriptBuf = tokTx.outputs[vout].script.toBuffer()
      inputTokenAddressArray = Buffer.concat([inputTokenAddressArray, TokenProto.getTokenAddress(lastTokenScriptBuf)])
      inputTokenAmountArray = Buffer.concat([
        inputTokenAmountArray,
        Common.getUInt64Buf(TokenProto.getTokenAmount(lastTokenScriptBuf)),
      ])
      const proof = Common.getTxOutputProof(tokTx, vout)
      tokenTxHeaderArray = Buffer.concat([tokenTxHeaderArray, Buffer.from(proof.txHeader.serialize(), 'hex')])
      const hp = Buffer.from(proof.hashProof.serialize(), 'hex')
      tokenTxHashProofArray = Buffer.concat([tokenTxHashProofArray, Common.getUInt32Buf(hp.length), hp])
      tokenSatoshiBytesArray = Buffer.concat([
        tokenSatoshiBytesArray,
        Buffer.from(proof.satoshiBytes.serialize(), 'hex'),
      ])
    }

    let otherOutputArray = Buffer.alloc(0)
    let tokenOutputIndexArray = Buffer.alloc(0)
    let j = 0
    let tokenOutputSatoshis = DUST
    for (let i = 0; i < tx.outputs.length; i++) {
      if (j < outputTokenIndexes.length && i === outputTokenIndexes[j]) {
        tokenOutputIndexArray = Buffer.concat([tokenOutputIndexArray, Common.getUInt32Buf(i)])
        tokenOutputSatoshis = tx.outputs[i].satoshis
        j++
      } else {
        const out = tx.outputs[i]
        const outBuf = Common.buildOutput(out.script.toBuffer(), out.satoshis)
        otherOutputArray = Buffer.concat([otherOutputArray, Common.getUInt32Buf(outBuf.length), outBuf])
      }
    }

    const preimage = getPreimage(tx, checkInst.lockingScript.subScript(0), checkSatoshis, checkInputIdx, SIG_ALL)
    const call = checkInst.unlock(
      new SigHashPreimage(toHex(preimage)),
      new Bytes(prevouts.toString('hex')),
      new Bytes(lastTokenScriptBuf.toString('hex')),
      new Bytes(tokenTxHeaderArray.toString('hex')),
      new Bytes(tokenTxHashProofArray.toString('hex')),
      new Bytes(tokenSatoshiBytesArray.toString('hex')),
      new Bytes(inputTokenAddressArray.toString('hex')),
      new Bytes(inputTokenAmountArray.toString('hex')),
      tx.outputs.length,
      new Bytes(tokenOutputIndexArray.toString('hex')),
      tokenOutputSatoshis,
      new Bytes(otherOutputArray.toString('hex'))
    )
    const v = call.verify({ tx, inputIndex: checkInputIdx, inputSatoshis: checkSatoshis })
    if (!v.success) throw new Error(`TokenUnlockContractCheck verify failed: ${v.error}`)
    tx.inputs[checkInputIdx].setScript(call.unlockingScript)
  }

  private async verifyAndSetTokenOp1(params: {
    tx: any
    tokenInputIdx: number
    tokenIndexInCheck: number
    checkInputIdx: number
    checkTx: any
    checkVout: number
    tokenTx: any
    tokenVout: number
    userSig: UserSignatureInput
    localTxMap?: Map<string, any>
  }): Promise<void> {
    const { tx, tokenInputIdx, tokenIndexInCheck, checkInputIdx, checkTx, checkVout, tokenTx, tokenVout, userSig, localTxMap } = params
    const tokenOut = tokenTx.outputs[tokenVout]
    const tokenScriptBuf = tokenOut.script.toBuffer()
    const { inst: tokenInst, exactMatch } = this.instantiateToken(tokenScriptBuf)
    const backtrace = await this.resolveTokenBacktrace(tokenTx, tokenVout, localTxMap)

    const prevouts = buildPrevouts(tx)
    const preimage = getPreimage(tx, tokenOut.script, tokenOut.satoshis, tokenInputIdx, SIG_ALL)
    const [inProofRaw, tokenTxHeader] = Common.getTxInputProof(tokenTx, backtrace.prevTokenInputIndex)
    const tokenTxInputProof = new this.contracts.TxInputProof(inProofRaw)
    const prevTokenTxProof = new this.contracts.TxOutputProof(Common.getTxOutputProof(backtrace.prevTokTx, backtrace.prevTokVout))
    const checkProof = new this.contracts.TxOutputProof(Common.getTxOutputProof(checkTx, checkVout))
    const dummyProof = new this.contracts.TxOutputProof(Common.getEmptyTxOutputProof())

    const call = tokenInst.unlock(
      new SigHashPreimage(toHex(preimage)),
      new Bytes(prevouts.toString('hex')),
      tokenIndexInCheck,
      0,
      checkInputIdx,
      checkProof,
      new Bytes(checkTx.outputs[checkVout].script.toBuffer().toString('hex')),
      backtrace.prevTokenInputIndex,
      new Bytes(backtrace.prevTokenAddress.toString('hex')),
      backtrace.prevTokenAmount,
      tokenTxHeader,
      tokenTxInputProof,
      prevTokenTxProof,
      new PubKey(userSig.publicKey),
      new Sig(userSig.sig),
      0,
      dummyProof,
      TokenProto.OP_TRANSFER
    )
    if (exactMatch) {
      const v = call.verify({ tx, inputIndex: tokenInputIdx, inputSatoshis: tokenOut.satoshis })
      if (!v.success) throw new Error(`Token op=1 verify failed at input ${tokenInputIdx}: ${v.error}`)
    } else {
      this.verifyScriptAgainstChain(tx, tokenInputIdx, call.unlockingScript, tokenOut.script, tokenOut.satoshis, 'Token(v1) op=1')
    }
    tx.inputs[tokenInputIdx].setScript(call.unlockingScript)
  }

  private async verifyAndSetTokenOp2(params: {
    tx: any
    tokenInputIdx: number
    tokenIndexInCheck: number
    amountCheckHashIndex: number
    checkInputIdx: number
    checkTx: any
    checkVout: number
    tokenTx: any
    tokenVout: number
    contractInputIdx: number
    contractTx: any
    contractVout: number
    localTxMap?: Map<string, any>
  }): Promise<void> {
    const {
      tx,
      tokenInputIdx,
      tokenIndexInCheck,
      amountCheckHashIndex,
      checkInputIdx,
      checkTx,
      checkVout,
      tokenTx,
      tokenVout,
      contractInputIdx,
      contractTx,
      contractVout,
      localTxMap,
    } = params
    const tokenOut = tokenTx.outputs[tokenVout]
    const tokenScriptBuf = tokenOut.script.toBuffer()
    const { inst: tokenInst, exactMatch } = this.instantiateToken(tokenScriptBuf)
    const backtrace = await this.resolveTokenBacktrace(tokenTx, tokenVout, localTxMap)

    const prevouts = buildPrevouts(tx)
    const preimage = getPreimage(tx, tokenOut.script, tokenOut.satoshis, tokenInputIdx, SIG_ALL)
    const [inProofRaw, tokenTxHeader] = Common.getTxInputProof(tokenTx, backtrace.prevTokenInputIndex)
    const tokenTxInputProof = new this.contracts.TxInputProof(inProofRaw)
    const prevTokenTxProof = new this.contracts.TxOutputProof(Common.getTxOutputProof(backtrace.prevTokTx, backtrace.prevTokVout))
    const checkProof = new this.contracts.TxOutputProof(Common.getTxOutputProof(checkTx, checkVout))
    const contractProof = new this.contracts.TxOutputProof(Common.getTxOutputProof(contractTx, contractVout))

    const call = tokenInst.unlock(
      new SigHashPreimage(toHex(preimage)),
      new Bytes(prevouts.toString('hex')),
      tokenIndexInCheck,
      amountCheckHashIndex,
      checkInputIdx,
      checkProof,
      new Bytes(checkTx.outputs[checkVout].script.toBuffer().toString('hex')),
      backtrace.prevTokenInputIndex,
      new Bytes(backtrace.prevTokenAddress.toString('hex')),
      backtrace.prevTokenAmount,
      tokenTxHeader,
      tokenTxInputProof,
      prevTokenTxProof,
      new PubKey(ZERO_PUBKEY_HEX),
      new Sig(ZERO_SIG_HEX),
      contractInputIdx,
      contractProof,
      TokenProto.OP_UNLOCK_FROM_CONTRACT
    )
    if (exactMatch) {
      const v = call.verify({ tx, inputIndex: tokenInputIdx, inputSatoshis: tokenOut.satoshis })
      if (!v.success) throw new Error(`Token op=2 verify failed at input ${tokenInputIdx}: ${v.error}`)
    } else {
      this.verifyScriptAgainstChain(tx, tokenInputIdx, call.unlockingScript, tokenOut.script, tokenOut.satoshis, 'Token(v1) op=2')
    }
    tx.inputs[tokenInputIdx].setScript(call.unlockingScript)
  }

  // =========================================================================
  // 1. Place Ask (2 steps: Step 0 PrepTx -> Step 1 LockTokenTx)
  // =========================================================================
  async buildPlaceAsk(params: PlaceAskParams): Promise<DraftBuildResult> {
    const tokenAmount = BigInt(params.tokenAmount)
    const priceNum = BigInt(params.priceNum)
    const priceDen = BigInt(params.priceDen)
    const minFillAmount = BigInt(params.minFillAmount)
    if (tokenAmount <= BigInt(0) || priceNum <= BigInt(0) || priceDen <= BigInt(0) || minFillAmount <= BigInt(0)) {
      throw new Error('tokenAmount, priceNum, priceDen, minFillAmount must be > 0')
    }
    if (minFillAmount > tokenAmount) {
      throw new Error('minFillAmount cannot exceed tokenAmount')
    }
    if (params.feeBps < 0 || params.feeBps > OrderProto.MAX_FEE_BPS) {
      throw new Error('feeBps out of range')
    }

    const makerHash20 = addressToHash20(params.makerAddress, this.net)
    const feeAddrHash20 = addressToHash20(params.feeAddress || '', this.net)
    const salt = params.saltHex ? Buffer.from(params.saltHex, 'hex') : crypto.randomBytes(8)
    const tokenCodeHashBuf = Buffer.from(params.tokenCodeHash, 'hex')
    const tokenIDBuf = Buffer.from(params.tokenID, 'hex')
    const feeRate = params.feeRate ?? this.defaultFeeRate

    const orderInst = new this.contracts.OrderAskV2(this.contracts.askUnlockCheckHashes)
    orderInst.setDataPart(
      OrderProto.buildOrderDataPart({
        salt,
        feeBps: params.feeBps,
        feeAddr: feeAddrHash20,
        minFillAmount,
        remainAmount: tokenAmount,
        priceDen,
        priceNum,
        tokenCodeHash: tokenCodeHashBuf,
        tokenID: tokenIDBuf,
        makerAddr: makerHash20,
        orderSide: OrderProto.SIDE_ASK,
      }).toString('hex')
    )
    const orderLockScript = orderInst.lockingScript
    const orderHash20 = mvc.crypto.Hash.sha256ripemd160(orderLockScript.toBuffer())

    const [tokenUtxos, mvcUtxos] = await Promise.all([
      this.chain.getTokenUtxos(params.makerAddress, params.tokenCodeHash, params.tokenID),
      this.chain.getMvcUtxos(params.makerAddress),
    ])
    const { selected: selTokens, total: totalTok } = selectTokenUtxos(tokenUtxos, tokenAmount, 3)
    const tokChange = totalTok - tokenAmount

    const receiverAmounts = tokChange > BigInt(0) ? [tokenAmount, tokChange] : [tokenAmount]
    const receiverAddresses = tokChange > BigInt(0) ? [orderHash20, makerHash20] : [orderHash20]
    const transferCheckInst = SwapEngine.buildTokenTransferCheck(
      this.contracts,
      tokenIDBuf,
      tokenCodeHashBuf,
      selTokens.length,
      receiverAmounts,
      receiverAddresses
    )

    const lockTxFeeBudget = Math.max(8000, Math.ceil((3500 + selTokens.length * 1600) * feeRate))
    const lockTokenDustOut = DUST * receiverAmounts.length
    const lockInputTokenDust = selTokens.reduce((acc, u) => acc + u.satoshis, 0) + DUST
    const lockFeeUtxoSats = Math.max(
      lockTxFeeBudget + DUST,
      lockTxFeeBudget + Math.max(0, lockTokenDustOut - lockInputTokenDust) + DUST
    )
    const prepFeeBudget = Math.max(3000, Math.ceil(2200 * feeRate))
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(
      mvcUtxos,
      DUST + DUST + lockFeeUtxoSats + prepFeeBudget
    )
    const makerP2pkh = p2pkhScript(params.makerAddress, this.net)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    prepTx.addOutput(new mvc.Transaction.Output({ script: orderLockScript, satoshis: DUST })) // vout 0: OrderAskV2
    prepTx.addOutput(new mvc.Transaction.Output({ script: transferCheckInst.lockingScript, satoshis: DUST })) // vout 1: TokenTransferCheck
    prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: lockFeeUtxoSats })) // vout 2: LockTx fee UTXO
    const prepChange = totalMvc - 2 * DUST - lockFeeUtxoSats - prepFeeBudget
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.makerAddress,
      kind: 'p2pkh',
      message: 'Sign Ask order prep transaction',
    }))

    let step = 0
    let lockTx: any = null
    let tokenTxs: any[] = []
    let actualChangeSats = 0
    let signedPrepTxid = ''
    const checkInputIdx = selTokens.length
    const feeInputIdx = selTokens.length + 1

    return this.saveDraft(
      'place_ask',
      2,
      step0Reqs,
      {
        side: 'ask',
        tokenAmount: tokenAmount.toString(),
        priceNum: priceNum.toString(),
        priceDen: priceDen.toString(),
        minFillAmount: minFillAmount.toString(),
        saltHex: salt.toString('hex'),
        estimatedMinerFee: prepFeeBudget + lockTxFeeBudget,
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          lockTx = new mvc.Transaction()
          lockTx.version = Common.TX_VERSION
          tokenTxs = []
          for (const tu of selTokens) {
            const raw = await this.chain.getRawTx(tu.txid)
            tokenTxs.push(new mvc.Transaction(raw))
            addTxInput(lockTx, tu.txid, tu.vout, mvc.Script.fromHex(tu.scriptHex), tu.satoshis)
          }
          addTxInput(lockTx, signedPrepTxid, 1, transferCheckInst.lockingScript, DUST)
          addTxInput(lockTx, signedPrepTxid, 2, makerP2pkh, lockFeeUtxoSats)

          const sampleTokBuf = Buffer.from(selTokens[0].scriptHex, 'hex')
          lockTx.addOutput(
            new mvc.Transaction.Output({
              script: mvc.Script.fromBuffer(TokenProto.getNewTokenScript(sampleTokBuf, orderHash20, tokenAmount)),
              satoshis: DUST,
            })
          )
          if (tokChange > BigInt(0)) {
            lockTx.addOutput(
              new mvc.Transaction.Output({
                script: mvc.Script.fromBuffer(TokenProto.getNewTokenScript(sampleTokBuf, makerHash20, tokChange)),
                satoshis: DUST,
              })
            )
          }
          const lockSpaceChange = lockInputTokenDust + lockFeeUtxoSats - lockTokenDustOut - lockTxFeeBudget
          actualChangeSats = lockSpaceChange >= DUST ? lockSpaceChange : 0
          if (actualChangeSats >= DUST) {
            lockTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: actualChangeSats }))
          }

          const lockHex = lockTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = selTokens.map((tu, i) => ({
            txIndex: 0,
            inputIndex: i,
            txHex: lockHex,
            scriptHex: tu.scriptHex,
            satoshis: tu.satoshis,
            sigtype: SIG_ALL,
            address: params.makerAddress,
            kind: 'token_p2pkh',
            message: 'Sign FT token lock for Ask order',
          }))
          step1Reqs.push({
            txIndex: 0,
            inputIndex: feeInputIdx,
            txHex: lockHex,
            scriptHex: makerP2pkh.toHex(),
            satoshis: lockFeeUtxoSats,
            sigtype: SIG_ALL,
            address: params.makerAddress,
            kind: 'p2pkh',
            message: 'Sign miner fee for Ask order',
          })

          step = 1
          return {
            draftId: '',
            action: 'place_ask',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock TokenTransferCheck + Token(op=1) + P2PKH fee
        const localMap = new Map<string, any>([[signedPrepTxid.toLowerCase(), prepTx]])
        this.verifyAndSetTransferCheck(
          lockTx,
          checkInputIdx,
          transferCheckInst,
          DUST,
          tokenTxs,
          selTokens.map((u) => u.vout),
          DUST,
          actualChangeSats,
          makerHash20
        )
        for (let i = 0; i < selTokens.length; i++) {
          await this.verifyAndSetTokenOp1({
            tx: lockTx,
            tokenInputIdx: i,
            tokenIndexInCheck: i,
            checkInputIdx,
            checkTx: prepTx,
            checkVout: 1,
            tokenTx: tokenTxs[i],
            tokenVout: selTokens[i].vout,
            userSig: getSigOrThrow(sigMap, 0, i),
            localTxMap: localMap,
          })
        }
        applyP2PKHSig(lockTx, feeInputIdx, getSigOrThrow(sigMap, 0, feeInputIdx))

        const txHexes = [prepTx.uncheckedSerialize(), lockTx.uncheckedSerialize()]
        const lockTxid = getMvcV10TxId(lockTx)
        const txids: string[] = []
        if (broadcast) {
          txids.push(await this.chain.broadcastTx(txHexes[0]))
          txids.push(await this.chain.broadcastTx(txHexes[1]))
        } else {
          txids.push(signedPrepTxid, lockTxid)
        }

        return {
          draftId: '',
          action: 'place_ask',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders: [
            {
              side: 'ask',
              txid: signedPrepTxid,
              vout: 0,
              satoshis: DUST,
              scriptHex: orderLockScript.toHex(),
              remainAmount: tokenAmount.toString(),
              tokenTxid: lockTxid,
              tokenVout: 0,
              saltHex: salt.toString('hex'),
            },
          ],
        }
      }
    )
  }

  // =========================================================================
  // 2. Place Bid (1 step: single P2PKH Tx -> OrderBidV2 output)
  // =========================================================================
  async buildPlaceBid(params: PlaceBidParams): Promise<DraftBuildResult> {
    const spendSats = BigInt(params.spendSats)
    const priceNum = BigInt(params.priceNum)
    const priceDen = BigInt(params.priceDen)
    const minFillAmount = BigInt(params.minFillAmount)
    if (spendSats < BigInt(DUST) || priceNum <= BigInt(0) || priceDen <= BigInt(0) || minFillAmount <= BigInt(0)) {
      throw new Error('Invalid Bid parameters (spendSats >= 546, priceNum/priceDen/minFillAmount > 0)')
    }
    if ((spendSats * priceDen) / priceNum < minFillAmount) {
      throw new Error('spendSats too small to purchase minFillAmount tokens at specified price')
    }
    if (params.feeBps < 0 || params.feeBps > OrderProto.MAX_FEE_BPS) {
      throw new Error('feeBps out of range')
    }

    const makerHash20 = addressToHash20(params.makerAddress, this.net)
    const feeAddrHash20 = addressToHash20(params.feeAddress || '', this.net)
    const salt = params.saltHex ? Buffer.from(params.saltHex, 'hex') : crypto.randomBytes(8)
    const feeRate = params.feeRate ?? this.defaultFeeRate

    const orderInst = new this.contracts.OrderBidV2()
    orderInst.setDataPart(
      OrderProto.buildOrderDataPart({
        salt,
        feeBps: params.feeBps,
        feeAddr: feeAddrHash20,
        minFillAmount,
        remainAmount: spendSats,
        priceDen,
        priceNum,
        tokenCodeHash: Buffer.from(params.tokenCodeHash, 'hex'),
        tokenID: Buffer.from(params.tokenID, 'hex'),
        makerAddr: makerHash20,
        orderSide: OrderProto.SIDE_BID,
      }).toString('hex')
    )
    const orderLockScript = orderInst.lockingScript
    const minerFee = Math.max(2000, Math.ceil(1600 * feeRate))
    const mvcUtxos = await this.chain.getMvcUtxos(params.makerAddress)
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, Number(spendSats) + minerFee)
    const makerP2pkh = p2pkhScript(params.makerAddress, this.net)

    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(tx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    tx.addOutput(new mvc.Transaction.Output({ script: orderLockScript, satoshis: Number(spendSats) }))
    const change = totalMvc - Number(spendSats) - minerFee
    if (change >= DUST) {
      tx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: change }))
    }

    const txHex = tx.uncheckedSerialize()
    const signRequests: SignRequestItem[] = selMvc.map((u, idx) => ({
      txIndex: 0,
      inputIndex: idx,
      txHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.makerAddress,
      kind: 'p2pkh',
      message: 'Sign Bid order placement',
    }))

    return this.saveDraft(
      'place_bid',
      1,
      signRequests,
      {
        side: 'bid',
        spendSats: spendSats.toString(),
        priceNum: priceNum.toString(),
        priceDen: priceDen.toString(),
        minFillAmount: minFillAmount.toString(),
        saltHex: salt.toString('hex'),
        estimatedMinerFee: minerFee,
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))
        for (let i = 0; i < selMvc.length; i++) {
          applyP2PKHSig(tx, i, getSigOrThrow(sigMap, 0, i))
        }
        const finalHex = tx.uncheckedSerialize()
        const txid = broadcast ? await this.chain.broadcastTx(finalHex) : getMvcV10TxId(tx)
        return {
          draftId: '',
          action: 'place_bid',
          status: 'completed',
          txids: [txid],
          txHexes: [finalHex],
          verified: true,
          broadcasted: broadcast,
          newOrders: [
            {
              side: 'bid',
              txid,
              vout: 0,
              satoshis: Number(spendSats),
              scriptHex: orderLockScript.toHex(),
              remainAmount: spendSats.toString(),
              saltHex: salt.toString('hex'),
            },
          ],
        }
      }
    )
  }

  // =========================================================================
  // 3. Take Ask (2 steps: Step 0 PrepTx -> Step 1 FillTx[0..N-1])
  // =========================================================================
  async buildTakeAsk(params: TakeAskParams): Promise<DraftBuildResult> {
    if (!params.legs || params.legs.length === 0) {
      throw new Error('At least one Ask leg is required')
    }
    const takerHash20 = addressToHash20(params.takerAddress, this.net)
    const takerP2pkh = p2pkhScript(params.takerAddress, this.net)
    const feeRate = params.feeRate ?? this.defaultFeeRate

    interface PreparedAskLeg {
      leg: TakeAskLeg
      orderTx: any
      tokenTx: any
      orderInst: any
      orderScriptBuf: Buffer
      orderData: any
      fillToken: bigint
      remainNext: bigint
      grossSats: bigint
      feeSats: bigint
      hasFeeOut: boolean
      newOrderScriptBuf?: Buffer
      newOrderHash20?: Buffer
      unlockCheckInst: any
      fillFeeBudget: number
      payUtxoSats: number
      checkVoutInPrep: number
      payVoutInPrep: number
      fillTx?: any
    }

    const prepared: PreparedAskLeg[] = []
    let totalGrossSats = BigInt(0)
    let totalFeeSats = BigInt(0)
    let totalFillToken = BigInt(0)
    let prepOutputsSats = 0

    for (const leg of params.legs) {
      const [orderRaw, tokenRaw] = await Promise.all([
        this.chain.getRawTx(leg.orderTxid),
        this.chain.getRawTx(leg.tokenTxid),
      ])
      const orderTx = new mvc.Transaction(orderRaw)
      const tokenTx = new mvc.Transaction(tokenRaw)
      const orderScriptBuf = orderTx.outputs[leg.orderVout].script.toBuffer()
      const orderData = OrderProto.parseOrderScript(orderScriptBuf)
      if (orderData.orderSide !== OrderProto.SIDE_ASK) {
        throw new Error(`Order ${leg.orderTxid}:${leg.orderVout} is not an Ask order`)
      }

      const orderInst = new this.contracts.OrderAskV2(this.contracts.askUnlockCheckHashes)
      orderInst.setDataPart(orderScriptBuf.subarray(orderScriptBuf.length - OrderProto.RAW_DATA_LEN).toString('hex'))
      if (!orderInst.lockingScript.toBuffer().equals(orderScriptBuf)) {
        throw new Error(`OrderAskV2 script mismatch for ${leg.orderTxid}:${leg.orderVout}`)
      }

      const fillToken = BigInt(leg.fillToken)
      if (fillToken <= BigInt(0) || fillToken > orderData.remainAmount) {
        throw new Error(`Invalid fillToken ${fillToken} for remainAmount ${orderData.remainAmount}`)
      }
      const remainNext = orderData.remainAmount - fillToken
      if (remainNext > BigInt(0) && (fillToken < orderData.minFillAmount || remainNext < orderData.minFillAmount)) {
        throw new Error(`Partial fill violates minFillAmount (${orderData.minFillAmount})`)
      }

      const grossSats = OrderProto.calcAskGrossSats(fillToken, orderData.priceNum, orderData.priceDen)
      if (grossSats < BigInt(DUST)) {
        throw new Error(`grossSats ${grossSats} below dust limit`)
      }
      const feeSats = OrderProto.calcFeeSats(grossSats, orderData.feeBps)
      const hasFeeOut = !orderData.feeAddr.equals(OrderProto.NULL_ADDRESS) && feeSats >= BigInt(DUST)

      let newOrderScriptBuf: Buffer | undefined
      let newOrderHash20: Buffer | undefined
      const recAmounts: bigint[] = []
      const recAddrs: Buffer[] = []
      if (remainNext > BigInt(0)) {
        newOrderScriptBuf = OrderProto.getNewOrderScript(orderScriptBuf, remainNext)
        newOrderHash20 = mvc.crypto.Hash.sha256ripemd160(newOrderScriptBuf)
        recAmounts.push(remainNext, fillToken)
        recAddrs.push(newOrderHash20, takerHash20)
      } else {
        recAmounts.push(fillToken)
        recAddrs.push(takerHash20)
      }

      const unlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
        this.contracts,
        orderData.tokenID,
        orderData.tokenCodeHash,
        [1],
        recAmounts,
        recAddrs,
        false
      )

      const fillFeeBudget = Math.max(12000, Math.ceil(9500 * feeRate))
      const outSatsNeeded =
        (remainNext > BigInt(0) ? 2 * DUST : 0) +
        Number(grossSats) +
        (hasFeeOut ? Number(feeSats) : 0) +
        DUST
      const inContractSats =
        orderTx.outputs[leg.orderVout].satoshis + tokenTx.outputs[leg.tokenVout].satoshis + DUST
      const payUtxoSats = Math.max(DUST, outSatsNeeded + fillFeeBudget - inContractSats)

      prepared.push({
        leg,
        orderTx,
        tokenTx,
        orderInst,
        orderScriptBuf,
        orderData,
        fillToken,
        remainNext,
        grossSats,
        feeSats,
        hasFeeOut,
        newOrderScriptBuf,
        newOrderHash20,
        unlockCheckInst,
        fillFeeBudget,
        payUtxoSats,
        checkVoutInPrep: 0,
        payVoutInPrep: 0,
      })
      totalGrossSats += grossSats
      if (hasFeeOut) totalFeeSats += feeSats
      totalFillToken += fillToken
      prepOutputsSats += DUST + payUtxoSats
    }

    const prepFeeBudget = Math.max(2500, Math.ceil((1200 + prepared.length * 400) * feeRate))
    const mvcUtxos = await this.chain.getMvcUtxos(params.takerAddress)
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, prepOutputsSats + prepFeeBudget)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    for (const p of prepared) {
      p.checkVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.unlockCheckInst.lockingScript, satoshis: DUST }))
      p.payVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: p.payUtxoSats }))
    }
    const prepChange = totalMvc - prepOutputsSats - prepFeeBudget
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.takerAddress,
      kind: 'p2pkh',
      message: 'Sign Take Ask prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''

    return this.saveDraft(
      'take_ask',
      2,
      step0Reqs,
      {
        side: 'ask',
        legs: prepared.length,
        totalFillToken: totalFillToken.toString(),
        totalGrossSats: totalGrossSats.toString(),
        totalFeeSats: totalFeeSats.toString(),
        estimatedMinerFee: prepFeeBudget + prepared.reduce((a, p) => a + p.fillFeeBudget, 0),
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          const step1Reqs: SignRequestItem[] = []
          for (let idx = 0; idx < prepared.length; idx++) {
            const p = prepared[idx]
            const fillTx = new mvc.Transaction()
            fillTx.version = Common.TX_VERSION
            addTxInput(fillTx, p.leg.orderTxid, p.leg.orderVout, mvc.Script.fromBuffer(p.orderScriptBuf), p.orderTx.outputs[p.leg.orderVout].satoshis)
            addTxInput(fillTx, p.leg.tokenTxid, p.leg.tokenVout, p.tokenTx.outputs[p.leg.tokenVout].script, p.tokenTx.outputs[p.leg.tokenVout].satoshis)
            addTxInput(fillTx, signedPrepTxid, p.checkVoutInPrep, p.unlockCheckInst.lockingScript, DUST)
            addTxInput(fillTx, signedPrepTxid, p.payVoutInPrep, takerP2pkh, p.payUtxoSats)

            const lockedTokBuf = p.tokenTx.outputs[p.leg.tokenVout].script.toBuffer()
            if (p.remainNext > BigInt(0)) {
              fillTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(p.newOrderScriptBuf!), satoshis: DUST }))
              const newTokBuf = TokenProto.getNewTokenScript(lockedTokBuf, p.newOrderHash20!, p.remainNext)
              fillTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(newTokBuf), satoshis: DUST }))
            }
            fillTx.addOutput(new mvc.Transaction.Output({ script: p2pkhScriptFromHash20(p.orderData.makerAddr), satoshis: Number(p.grossSats) }))
            if (p.hasFeeOut) {
              fillTx.addOutput(new mvc.Transaction.Output({ script: p2pkhScriptFromHash20(p.orderData.feeAddr), satoshis: Number(p.feeSats) }))
            }
            const takerTokBuf = TokenProto.getNewTokenScript(lockedTokBuf, takerHash20, p.fillToken)
            fillTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(takerTokBuf), satoshis: DUST }))

            p.fillTx = fillTx
            step1Reqs.push({
              txIndex: idx,
              inputIndex: 3,
              txHex: fillTx.uncheckedSerialize(),
              scriptHex: takerP2pkh.toHex(),
              satoshis: p.payUtxoSats,
              sigtype: SIG_ALL,
              address: params.takerAddress,
              kind: 'p2pkh',
              message: `Sign payment for Take Ask leg ${idx + 1}/${prepared.length}`,
            })
          }

          step = 1
          return {
            draftId: '',
            action: 'take_ask',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock & verify every FillTx, then broadcast [PrepTx, FillTx...]
        const localMap = new Map<string, any>([[signedPrepTxid.toLowerCase(), prepTx]])
        const txHexes = [prepTx.uncheckedSerialize()]
        const txids: string[] = []
        const newOrders: NewOrderOutputInfo[] = []

        for (let idx = 0; idx < prepared.length; idx++) {
          const p = prepared[idx]
          const fillTx = p.fillTx
          const prevouts = buildPrevouts(fillTx)
          const outputTokenIndexes = p.remainNext > BigInt(0) ? [1, fillTx.outputs.length - 1] : [fillTx.outputs.length - 1]
          const prefixCount = (p.remainNext > BigInt(0) ? 3 : 1) + (p.hasFeeOut ? 1 : 0)
          const restOutputs = buildRestOutputs(fillTx, prefixCount)

          // input[0]: OrderAskV2 (OP_FILL = 1)
          {
            const preimage = getPreimage(fillTx, p.orderInst.lockingScript, p.orderTx.outputs[p.leg.orderVout].satoshis, 0, SIG_ALL)
            const tokenProof = Common.getTxOutputProof(p.tokenTx, p.leg.tokenVout)
            const checkProof = Common.getTxOutputProof(prepTx, p.checkVoutInPrep)
            const call = p.orderInst.unlock(
              new SigHashPreimage(toHex(preimage)),
              new Bytes(prevouts.toString('hex')),
              new Bytes(p.tokenTx.outputs[p.leg.tokenVout].script.toBuffer().toString('hex')),
              tokenProof.txHeader,
              tokenProof.hashProof,
              tokenProof.satoshiBytes,
              0,
              new Bytes(p.unlockCheckInst.lockingScript.toBuffer().toString('hex')),
              checkProof.txHeader,
              checkProof.hashProof,
              checkProof.satoshiBytes,
              p.fillToken,
              DUST,
              new Bytes(restOutputs.toString('hex')),
              new PubKey(ZERO_PUBKEY_HEX),
              new Sig(ZERO_SIG_HEX),
              OrderProto.OP_FILL
            )
            const v = call.verify({ tx: fillTx, inputIndex: 0, inputSatoshis: p.orderTx.outputs[p.leg.orderVout].satoshis })
            if (!v.success) throw new Error(`OrderAskV2 verify failed on leg ${idx}: ${v.error}`)
            fillTx.inputs[0].setScript(call.unlockingScript)
          }

          // input[1]: LockedToken (OP_UNLOCK_FROM_CONTRACT)
          await this.verifyAndSetTokenOp2({
            tx: fillTx,
            tokenInputIdx: 1,
            tokenIndexInCheck: 0,
            amountCheckHashIndex: 0,
            checkInputIdx: 2,
            checkTx: prepTx,
            checkVout: p.checkVoutInPrep,
            tokenTx: p.tokenTx,
            tokenVout: p.leg.tokenVout,
            contractInputIdx: 0,
            contractTx: p.orderTx,
            contractVout: p.leg.orderVout,
            localTxMap: localMap,
          })

          // input[2]: TokenUnlockContractCheck
          this.verifyAndSetUnlockCheck(fillTx, 2, p.unlockCheckInst, DUST, [p.tokenTx], [p.leg.tokenVout], outputTokenIndexes)

          // input[3]: Taker P2PKH
          applyP2PKHSig(fillTx, 3, getSigOrThrow(sigMap, idx, 3))

          const fillTxid = getMvcV10TxId(fillTx)
          localMap.set(fillTxid.toLowerCase(), fillTx)
          txHexes.push(fillTx.uncheckedSerialize())

          if (p.remainNext > BigInt(0)) {
            newOrders.push({
              side: 'ask',
              txid: fillTxid,
              vout: 0,
              satoshis: DUST,
              scriptHex: p.newOrderScriptBuf!.toString('hex'),
              remainAmount: p.remainNext.toString(),
              tokenTxid: fillTxid,
              tokenVout: 1,
              saltHex: p.orderData.salt.toString('hex'),
            })
          }
        }

        if (broadcast) {
          for (const hex of txHexes) {
            txids.push(await this.chain.broadcastTx(hex))
          }
        } else {
          txids.push(signedPrepTxid, ...prepared.map((p) => getMvcV10TxId(p.fillTx)))
        }

        return {
          draftId: '',
          action: 'take_ask',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders,
        }
      }
    )
  }

  // =========================================================================
  // 4. Take Bid (2 user steps: Step 0 PrepTx -> Step 1 FillerDepositTx -> auto 0-sig SettleTx[0..N-1])
  // =========================================================================
  async buildTakeBid(params: TakeBidParams): Promise<DraftBuildResult> {
    if (!params.legs || params.legs.length === 0) {
      throw new Error('At least one Bid leg is required')
    }
    if (params.legs.length > 2) {
      throw new Error('Single-batch Take Bid supports up to 2 legs per FillerDeposit (3-output TokenTransferCheck)')
    }
    const takerHash20 = addressToHash20(params.takerAddress, this.net)
    const takerP2pkh = p2pkhScript(params.takerAddress, this.net)
    const feeRate = params.feeRate ?? this.defaultFeeRate
    const tokenCodeHashBuf = Buffer.from(params.tokenCodeHash, 'hex')
    const tokenIDBuf = Buffer.from(params.tokenID, 'hex')

    interface PreparedBidLeg {
      leg: TakeBidLeg
      orderTx: any
      orderInst: any
      orderScriptBuf: Buffer
      orderData: any
      inSats: bigint
      fillToken: bigint
      spendSats: bigint
      feeSats: bigint
      hasFeeOut: boolean
      remainSatsNext: bigint
      newOrderScriptBuf?: Buffer
      settleMinerFee: number
      takerReceiveSats: number
      fillerInst: any
      fillerHash20: Buffer
      unlockCheckInst: any
      fillerVoutInPrep: number
      checkVoutInPrep: number
      tokenVoutInDeposit: number
    }

    const prepared: PreparedBidLeg[] = []
    let totalFillToken = BigInt(0)
    let totalSpendSats = BigInt(0)
    let totalFeeSats = BigInt(0)

    for (const leg of params.legs) {
      const orderRaw = await this.chain.getRawTx(leg.orderTxid)
      const orderTx = new mvc.Transaction(orderRaw)
      const orderOut = orderTx.outputs[leg.orderVout]
      const orderScriptBuf = orderOut.script.toBuffer()
      const orderData = OrderProto.parseOrderScript(orderScriptBuf)
      if (orderData.orderSide !== OrderProto.SIDE_BID) {
        throw new Error(`Order ${leg.orderTxid}:${leg.orderVout} is not a Bid order`)
      }
      const inSats = BigInt(orderOut.satoshis)
      if (inSats !== orderData.remainAmount) {
        throw new Error(`OrderBidV2 satoshis (${inSats}) != remainAmount (${orderData.remainAmount})`)
      }

      const orderInst = new this.contracts.OrderBidV2()
      orderInst.setDataPart(orderScriptBuf.subarray(orderScriptBuf.length - OrderProto.RAW_DATA_LEN).toString('hex'))
      if (!orderInst.lockingScript.toBuffer().equals(orderScriptBuf)) {
        throw new Error(`OrderBidV2 script mismatch for ${leg.orderTxid}:${leg.orderVout}`)
      }

      const fillToken = BigInt(leg.fillToken)
      const spendSats = OrderProto.calcBidSpendSats(fillToken, orderData.priceNum, orderData.priceDen)
      if (fillToken <= BigInt(0) || spendSats <= BigInt(0) || spendSats > inSats) {
        throw new Error(`Invalid Bid fillToken=${fillToken} spendSats=${spendSats} inSats=${inSats}`)
      }
      const remainSatsNext = inSats - spendSats
      if (remainSatsNext > BigInt(0)) {
        if (
          fillToken < orderData.minFillAmount ||
          remainSatsNext < BigInt(DUST) ||
          (remainSatsNext * orderData.priceDen) / orderData.priceNum < orderData.minFillAmount
        ) {
          throw new Error('Partial Bid fill violates minFillAmount or zombie-dust guard')
        }
      }

      const feeSats = OrderProto.calcFeeSats(spendSats, orderData.feeBps)
      const hasFeeOut = !orderData.feeAddr.equals(OrderProto.NULL_ADDRESS) && feeSats >= BigInt(DUST)
      const settleMinerFee = Math.max(11000, Math.ceil(9000 * feeRate))
      const takerReceiveSats =
        Number(spendSats) + 2 * DUST - (hasFeeOut ? Number(feeSats) : 0) - settleMinerFee
      if (takerReceiveSats < DUST) {
        throw new Error(
          `Bid leg proceeds (${spendSats} sats) too small to cover fee (${feeSats}) + minerFee (${settleMinerFee})`
        )
      }

      const fillerInst = new this.contracts.OrderBidFiller(
        new Ripemd160(takerHash20.toString('hex')),
        takerReceiveSats
      )
      const fillerHash20 = mvc.crypto.Hash.sha256ripemd160(fillerInst.lockingScript.toBuffer())

      const unlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
        this.contracts,
        orderData.tokenID,
        orderData.tokenCodeHash,
        [2],
        [fillToken],
        [orderData.makerAddr],
        false
      )

      prepared.push({
        leg,
        orderTx,
        orderInst,
        orderScriptBuf,
        orderData,
        inSats,
        fillToken,
        spendSats,
        feeSats,
        hasFeeOut,
        remainSatsNext,
        newOrderScriptBuf: remainSatsNext > BigInt(0) ? OrderProto.getNewOrderScript(orderScriptBuf, remainSatsNext) : undefined,
        settleMinerFee,
        takerReceiveSats,
        fillerInst,
        fillerHash20,
        unlockCheckInst,
        fillerVoutInPrep: 0,
        checkVoutInPrep: 0,
        tokenVoutInDeposit: 0,
      })
      totalFillToken += fillToken
      totalSpendSats += spendSats
      if (hasFeeOut) totalFeeSats += feeSats
    }

    const [tokenUtxos, mvcUtxos] = await Promise.all([
      this.chain.getTokenUtxos(params.takerAddress, params.tokenCodeHash, params.tokenID),
      this.chain.getMvcUtxos(params.takerAddress),
    ])
    const { selected: selTokens, total: totalTok } = selectTokenUtxos(tokenUtxos, totalFillToken, 3)
    const tokChange = totalTok - totalFillToken

    const depositRecAmounts: bigint[] = prepared.map((p) => p.fillToken)
    const depositRecAddrs: Buffer[] = prepared.map((p) => p.fillerHash20)
    if (tokChange > BigInt(0)) {
      depositRecAmounts.push(tokChange)
      depositRecAddrs.push(takerHash20)
    }

    const transferCheckInst = SwapEngine.buildTokenTransferCheck(
      this.contracts,
      tokenIDBuf,
      tokenCodeHashBuf,
      selTokens.length,
      depositRecAmounts,
      depositRecAddrs
    )

    const depositFeeBudget = Math.max(8500, Math.ceil((3800 + selTokens.length * 1600) * feeRate))
    const depositTokenOutDust = DUST * depositRecAmounts.length
    const depositTokenInDust = selTokens.reduce((a, u) => a + u.satoshis, 0) + DUST
    const depositFeeUtxoSats = Math.max(
      depositFeeBudget + DUST,
      depositFeeBudget + Math.max(0, depositTokenOutDust - depositTokenInDust) + DUST
    )
    const prepFeeBudget = Math.max(3000, Math.ceil((1800 + prepared.length * 400) * feeRate))
    const prepFixedDust = DUST + prepared.length * 2 * DUST
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(
      mvcUtxos,
      prepFixedDust + depositFeeUtxoSats + prepFeeBudget
    )

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    const transferCheckVout = 0
    prepTx.addOutput(new mvc.Transaction.Output({ script: transferCheckInst.lockingScript, satoshis: DUST }))
    const depositFeeVout = 1
    prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: depositFeeUtxoSats }))
    for (const p of prepared) {
      p.fillerVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.fillerInst.lockingScript, satoshis: DUST }))
      p.checkVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.unlockCheckInst.lockingScript, satoshis: DUST }))
    }
    const prepChange = totalMvc - prepFixedDust - depositFeeUtxoSats - prepFeeBudget
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.takerAddress,
      kind: 'p2pkh',
      message: 'Sign Take Bid prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''
    let depositTx: any = null
    let tokenTxs: any[] = []
    let actualDepositChange = 0
    const checkInputIdx = selTokens.length
    const feeInputIdx = selTokens.length + 1

    return this.saveDraft(
      'take_bid',
      2,
      step0Reqs,
      {
        side: 'bid',
        legs: prepared.length,
        totalFillToken: totalFillToken.toString(),
        totalSpendSats: totalSpendSats.toString(),
        totalFeeSats: totalFeeSats.toString(),
        totalTakerReceiveSats: prepared.reduce((a, p) => a + p.takerReceiveSats, 0).toString(),
        estimatedMinerFee: prepFeeBudget + depositFeeBudget + prepared.reduce((a, p) => a + p.settleMinerFee, 0),
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          depositTx = new mvc.Transaction()
          depositTx.version = Common.TX_VERSION
          tokenTxs = []
          for (const tu of selTokens) {
            const raw = await this.chain.getRawTx(tu.txid)
            tokenTxs.push(new mvc.Transaction(raw))
            addTxInput(depositTx, tu.txid, tu.vout, mvc.Script.fromHex(tu.scriptHex), tu.satoshis)
          }
          addTxInput(depositTx, signedPrepTxid, transferCheckVout, transferCheckInst.lockingScript, DUST)
          addTxInput(depositTx, signedPrepTxid, depositFeeVout, takerP2pkh, depositFeeUtxoSats)

          const sampleTokBuf = Buffer.from(selTokens[0].scriptHex, 'hex')
          for (const p of prepared) {
            p.tokenVoutInDeposit = depositTx.outputs.length
            depositTx.addOutput(
              new mvc.Transaction.Output({
                script: mvc.Script.fromBuffer(TokenProto.getNewTokenScript(sampleTokBuf, p.fillerHash20, p.fillToken)),
                satoshis: DUST,
              })
            )
          }
          if (tokChange > BigInt(0)) {
            depositTx.addOutput(
              new mvc.Transaction.Output({
                script: mvc.Script.fromBuffer(TokenProto.getNewTokenScript(sampleTokBuf, takerHash20, tokChange)),
                satoshis: DUST,
              })
            )
          }
          const depositChangeRaw = depositTokenInDust + depositFeeUtxoSats - depositTokenOutDust - depositFeeBudget
          actualDepositChange = depositChangeRaw >= DUST ? depositChangeRaw : 0
          if (actualDepositChange >= DUST) {
            depositTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: actualDepositChange }))
          }

          const depositHex = depositTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = selTokens.map((tu, i) => ({
            txIndex: 0,
            inputIndex: i,
            txHex: depositHex,
            scriptHex: tu.scriptHex,
            satoshis: tu.satoshis,
            sigtype: SIG_ALL,
            address: params.takerAddress,
            kind: 'token_p2pkh',
            message: 'Sign FT token deposit into non-custodial OrderBidFiller',
          }))
          step1Reqs.push({
            txIndex: 0,
            inputIndex: feeInputIdx,
            txHex: depositHex,
            scriptHex: takerP2pkh.toHex(),
            satoshis: depositFeeUtxoSats,
            sigtype: SIG_ALL,
            address: params.takerAddress,
            kind: 'p2pkh',
            message: 'Sign miner fee for Take Bid deposit',
          })

          step = 1
          return {
            draftId: '',
            action: 'take_bid',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock FillerDepositTx + build/unlock/verify 0-sig SettleTx[0..N-1]
        const localMap = new Map<string, any>([[signedPrepTxid.toLowerCase(), prepTx]])

        this.verifyAndSetTransferCheck(
          depositTx,
          checkInputIdx,
          transferCheckInst,
          DUST,
          tokenTxs,
          selTokens.map((u) => u.vout),
          DUST,
          actualDepositChange,
          takerHash20
        )
        for (let i = 0; i < selTokens.length; i++) {
          await this.verifyAndSetTokenOp1({
            tx: depositTx,
            tokenInputIdx: i,
            tokenIndexInCheck: i,
            checkInputIdx,
            checkTx: prepTx,
            checkVout: transferCheckVout,
            tokenTx: tokenTxs[i],
            tokenVout: selTokens[i].vout,
            userSig: getSigOrThrow(sigMap, 0, i),
            localTxMap: localMap,
          })
        }
        applyP2PKHSig(depositTx, feeInputIdx, getSigOrThrow(sigMap, 0, feeInputIdx))
        const signedDepositTxid = getMvcV10TxId(depositTx)
        localMap.set(signedDepositTxid.toLowerCase(), depositTx)

        const txHexes = [prepTx.uncheckedSerialize(), depositTx.uncheckedSerialize()]
        const settleTxids: string[] = []
        const newOrders: NewOrderOutputInfo[] = []

        for (let idx = 0; idx < prepared.length; idx++) {
          const p = prepared[idx]
          const settleTx = new mvc.Transaction()
          settleTx.version = Common.TX_VERSION
          addTxInput(settleTx, p.leg.orderTxid, p.leg.orderVout, mvc.Script.fromBuffer(p.orderScriptBuf), Number(p.inSats))
          addTxInput(settleTx, signedPrepTxid, p.fillerVoutInPrep, p.fillerInst.lockingScript, DUST)
          addTxInput(settleTx, signedDepositTxid, p.tokenVoutInDeposit, depositTx.outputs[p.tokenVoutInDeposit].script, DUST)
          addTxInput(settleTx, signedPrepTxid, p.checkVoutInPrep, p.unlockCheckInst.lockingScript, DUST)

          const lockedTokBuf = depositTx.outputs[p.tokenVoutInDeposit].script.toBuffer()
          if (p.remainSatsNext > BigInt(0)) {
            settleTx.addOutput(
              new mvc.Transaction.Output({
                script: mvc.Script.fromBuffer(p.newOrderScriptBuf!),
                satoshis: Number(p.remainSatsNext),
              })
            )
          }
          const makerTokVout = settleTx.outputs.length
          settleTx.addOutput(
            new mvc.Transaction.Output({
              script: mvc.Script.fromBuffer(TokenProto.getNewTokenScript(lockedTokBuf, p.orderData.makerAddr, p.fillToken)),
              satoshis: DUST,
            })
          )
          if (p.hasFeeOut) {
            settleTx.addOutput(
              new mvc.Transaction.Output({
                script: p2pkhScriptFromHash20(p.orderData.feeAddr),
                satoshis: Number(p.feeSats),
              })
            )
          }
          const takerOutIdx = settleTx.outputs.length
          settleTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: p.takerReceiveSats }))

          const prevouts = buildPrevouts(settleTx)
          const prefixCount = (p.remainSatsNext > BigInt(0) ? 2 : 1) + (p.hasFeeOut ? 1 : 0)
          const restOutputs = buildRestOutputs(settleTx, prefixCount)

          // input[0]: OrderBidV2 (OP_FILL = 1)
          {
            const preimage = getPreimage(settleTx, p.orderInst.lockingScript, Number(p.inSats), 0, SIG_ALL)
            const tokenProof = Common.getTxOutputProof(depositTx, p.tokenVoutInDeposit)
            const call = p.orderInst.unlock(
              new SigHashPreimage(toHex(preimage)),
              new Bytes(prevouts.toString('hex')),
              2,
              new Bytes(lockedTokBuf.toString('hex')),
              tokenProof.txHeader,
              tokenProof.hashProof,
              tokenProof.satoshiBytes,
              p.fillToken,
              DUST,
              new Bytes(restOutputs.toString('hex')),
              new PubKey(ZERO_PUBKEY_HEX),
              new Sig(ZERO_SIG_HEX),
              OrderProto.OP_FILL
            )
            const v = call.verify({ tx: settleTx, inputIndex: 0, inputSatoshis: Number(p.inSats) })
            if (!v.success) throw new Error(`OrderBidV2 verify failed on leg ${idx}: ${v.error}`)
            settleTx.inputs[0].setScript(call.unlockingScript)
          }

          // input[1]: OrderBidFiller (OP_SETTLE = 1)
          {
            const preimage = getPreimage(settleTx, p.fillerInst.lockingScript, DUST, 1, SIG_ALL)
            const prevOutputs = buildOutputsRange(settleTx, 0, takerOutIdx)
            const nextOutputs = buildOutputsRange(settleTx, takerOutIdx + 1, settleTx.outputs.length)
            const call = p.fillerInst.unlock(
              new SigHashPreimage(toHex(preimage)),
              p.takerReceiveSats,
              new Bytes(prevOutputs.toString('hex')),
              new Bytes(nextOutputs.toString('hex')),
              new PubKey(ZERO_PUBKEY_HEX),
              new Sig(ZERO_SIG_HEX),
              1
            )
            const v = call.verify({ tx: settleTx, inputIndex: 1, inputSatoshis: DUST })
            if (!v.success) throw new Error(`OrderBidFiller verify failed on leg ${idx}: ${v.error}`)
            settleTx.inputs[1].setScript(call.unlockingScript)
          }

          // input[2]: LockedToken (OP_UNLOCK_FROM_CONTRACT via OrderBidFiller at input 1)
          await this.verifyAndSetTokenOp2({
            tx: settleTx,
            tokenInputIdx: 2,
            tokenIndexInCheck: 0,
            amountCheckHashIndex: 0,
            checkInputIdx: 3,
            checkTx: prepTx,
            checkVout: p.checkVoutInPrep,
            tokenTx: depositTx,
            tokenVout: p.tokenVoutInDeposit,
            contractInputIdx: 1,
            contractTx: prepTx,
            contractVout: p.fillerVoutInPrep,
            localTxMap: localMap,
          })

          // input[3]: TokenUnlockContractCheck
          this.verifyAndSetUnlockCheck(settleTx, 3, p.unlockCheckInst, DUST, [depositTx], [p.tokenVoutInDeposit], [makerTokVout])

          const settleTxid = getMvcV10TxId(settleTx)
          localMap.set(settleTxid.toLowerCase(), settleTx)
          settleTxids.push(settleTxid)
          txHexes.push(settleTx.uncheckedSerialize())

          if (p.remainSatsNext > BigInt(0)) {
            newOrders.push({
              side: 'bid',
              txid: settleTxid,
              vout: 0,
              satoshis: Number(p.remainSatsNext),
              scriptHex: p.newOrderScriptBuf!.toString('hex'),
              remainAmount: p.remainSatsNext.toString(),
              saltHex: p.orderData.salt.toString('hex'),
            })
          }
        }

        const txids: string[] = []
        if (broadcast) {
          for (const hex of txHexes) {
            txids.push(await this.chain.broadcastTx(hex))
          }
        } else {
          txids.push(signedPrepTxid, signedDepositTxid, ...settleTxids)
        }

        return {
          draftId: '',
          action: 'take_bid',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders,
        }
      }
    )
  }

  // =========================================================================
  // 5. Cancel Order (Bid: 1 step; Ask: 2 steps PrepTx -> CancelAskTx)
  // =========================================================================
  async buildCancel(params: CancelOrderParams): Promise<DraftBuildResult> {
    const makerHash20 = addressToHash20(params.makerAddress, this.net)
    const makerP2pkh = p2pkhScript(params.makerAddress, this.net)
    const feeRate = params.feeRate ?? this.defaultFeeRate

    if (params.side === 'token') {
      return this.buildCancelTokenOrder(params)
    }

    const orderRaw = await this.chain.getRawTx(params.orderTxid)
    const orderTx = new mvc.Transaction(orderRaw)
    const orderOut = orderTx.outputs[params.orderVout]
    const orderScriptBuf = orderOut.script.toBuffer()
    const orderData = OrderProto.parseOrderScript(orderScriptBuf)
    if (!orderData.makerAddr.equals(makerHash20)) {
      throw new Error('Only the order maker can cancel this order')
    }

    if (params.side === 'bid') {
      if (orderData.orderSide !== OrderProto.SIDE_BID) {
        throw new Error('Order side mismatch: expected Bid')
      }
      const orderInst = new this.contracts.OrderBidV2()
      orderInst.setDataPart(orderScriptBuf.subarray(orderScriptBuf.length - OrderProto.RAW_DATA_LEN).toString('hex'))
      const minerFee = Math.max(2500, Math.ceil(2000 * feeRate))
      const refundSats = orderOut.satoshis - minerFee
      if (refundSats < DUST) {
        throw new Error(`Bid balance (${orderOut.satoshis}) too small to cover cancel miner fee (${minerFee})`)
      }

      const cancelTx = new mvc.Transaction()
      cancelTx.version = Common.TX_VERSION
      addTxInput(cancelTx, params.orderTxid, params.orderVout, mvc.Script.fromBuffer(orderScriptBuf), orderOut.satoshis)
      cancelTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: refundSats }))

      const signRequests: SignRequestItem[] = [
        {
          txIndex: 0,
          inputIndex: 0,
          txHex: cancelTx.uncheckedSerialize(),
          scriptHex: orderScriptBuf.toString('hex'),
          satoshis: orderOut.satoshis,
          sigtype: SIG_ALL,
          address: params.makerAddress,
          kind: 'order_cancel',
          message: 'Sign Bid order cancellation',
        },
      ]

      return this.saveDraft(
        'cancel_bid',
        1,
        signRequests,
        {
          side: 'bid',
          orderTxid: params.orderTxid,
          orderVout: params.orderVout,
          refundSats: refundSats.toString(),
          estimatedMinerFee: minerFee,
        },
        async (sigs, broadcast) => {
          const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))
          const makerSig = getSigOrThrow(sigMap, 0, 0)
          const preimage = getPreimage(cancelTx, orderInst.lockingScript, orderOut.satoshis, 0, SIG_ALL)
          const call = orderInst.unlock(
            new SigHashPreimage(toHex(preimage)),
            new Bytes(buildPrevouts(cancelTx).toString('hex')),
            0,
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            BigInt(0),
            0,
            new Bytes(''),
            new PubKey(makerSig.publicKey),
            new Sig(makerSig.sig),
            OrderProto.OP_CANCEL
          )
          const v = call.verify({ tx: cancelTx, inputIndex: 0, inputSatoshis: orderOut.satoshis })
          if (!v.success) throw new Error(`OrderBidV2 cancel verify failed: ${v.error}`)
          cancelTx.inputs[0].setScript(call.unlockingScript)

          const finalHex = cancelTx.uncheckedSerialize()
          const txid = broadcast ? await this.chain.broadcastTx(finalHex) : getMvcV10TxId(cancelTx)
          return {
            draftId: '',
            action: 'cancel_bid',
            status: 'completed',
            txids: [txid],
            txHexes: [finalHex],
            verified: true,
            broadcasted: broadcast,
            newOrders: [],
          }
        }
      )
    }

    // Ask Cancel (2 steps: Step 0 PrepTx -> Step 1 CancelAskTx)
    if (orderData.orderSide !== OrderProto.SIDE_ASK) {
      throw new Error('Order side mismatch: expected Ask')
    }
    if (!params.tokenTxid || params.tokenVout === undefined) {
      throw new Error('tokenTxid and tokenVout are required to cancel an Ask order')
    }
    const tokenRaw = await this.chain.getRawTx(params.tokenTxid)
    const tokenTx = new mvc.Transaction(tokenRaw)
    const tokenOut = tokenTx.outputs[params.tokenVout]
    const lockedTokBuf = tokenOut.script.toBuffer()

    const orderInst = new this.contracts.OrderAskV2(this.contracts.askUnlockCheckHashes)
    orderInst.setDataPart(orderScriptBuf.subarray(orderScriptBuf.length - OrderProto.RAW_DATA_LEN).toString('hex'))

    const unlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
      this.contracts,
      orderData.tokenID,
      orderData.tokenCodeHash,
      [1],
      [orderData.remainAmount],
      [makerHash20],
      false
    )

    const cancelFeeBudget = Math.max(10000, Math.ceil(8000 * feeRate))
    const prepFeeBudget = Math.max(2500, Math.ceil(1600 * feeRate))
    const mvcUtxos = await this.chain.getMvcUtxos(params.makerAddress)
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, DUST + cancelFeeBudget + prepFeeBudget)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    prepTx.addOutput(new mvc.Transaction.Output({ script: unlockCheckInst.lockingScript, satoshis: DUST }))
    prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: cancelFeeBudget }))
    const prepChange = totalMvc - DUST - cancelFeeBudget - prepFeeBudget
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.makerAddress,
      kind: 'p2pkh',
      message: 'Sign Ask cancel prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''
    let cancelTx: any = null

    return this.saveDraft(
      'cancel_ask',
      2,
      step0Reqs,
      {
        side: 'ask',
        orderTxid: params.orderTxid,
        orderVout: params.orderVout,
        returnTokenAmount: orderData.remainAmount.toString(),
        estimatedMinerFee: prepFeeBudget + cancelFeeBudget,
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          cancelTx = new mvc.Transaction()
          cancelTx.version = Common.TX_VERSION
          addTxInput(cancelTx, params.orderTxid, params.orderVout, mvc.Script.fromBuffer(orderScriptBuf), orderOut.satoshis)
          addTxInput(cancelTx, params.tokenTxid!, params.tokenVout!, tokenOut.script, tokenOut.satoshis)
          addTxInput(cancelTx, signedPrepTxid, 0, unlockCheckInst.lockingScript, DUST)
          addTxInput(cancelTx, signedPrepTxid, 1, makerP2pkh, cancelFeeBudget)

          const makerTokBuf = TokenProto.getNewTokenScript(lockedTokBuf, makerHash20, orderData.remainAmount)
          cancelTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(makerTokBuf), satoshis: DUST }))

          const cancelHex = cancelTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = [
            {
              txIndex: 0,
              inputIndex: 0,
              txHex: cancelHex,
              scriptHex: orderScriptBuf.toString('hex'),
              satoshis: orderOut.satoshis,
              sigtype: SIG_ALL,
              address: params.makerAddress,
              kind: 'order_cancel',
              message: 'Sign Ask order cancellation',
            },
            {
              txIndex: 0,
              inputIndex: 3,
              txHex: cancelHex,
              scriptHex: makerP2pkh.toHex(),
              satoshis: cancelFeeBudget,
              sigtype: SIG_ALL,
              address: params.makerAddress,
              kind: 'p2pkh',
              message: 'Sign miner fee for Ask cancellation',
            },
          ]

          step = 1
          return {
            draftId: '',
            action: 'cancel_ask',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock OrderAskV2 (OP_CANCEL) + LockedToken (op=2) + UnlockCheck + P2PKH fee
        const localMap = new Map<string, any>([[signedPrepTxid.toLowerCase(), prepTx]])
        {
          const makerSig = getSigOrThrow(sigMap, 0, 0)
          const preimage = getPreimage(cancelTx, orderInst.lockingScript, orderOut.satoshis, 0, SIG_ALL)
          const call = orderInst.unlock(
            new SigHashPreimage(toHex(preimage)),
            new Bytes(buildPrevouts(cancelTx).toString('hex')),
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            0,
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            new Bytes(''),
            BigInt(0),
            0,
            new Bytes(''),
            new PubKey(makerSig.publicKey),
            new Sig(makerSig.sig),
            OrderProto.OP_CANCEL
          )
          const v = call.verify({ tx: cancelTx, inputIndex: 0, inputSatoshis: orderOut.satoshis })
          if (!v.success) throw new Error(`OrderAskV2 cancel verify failed: ${v.error}`)
          cancelTx.inputs[0].setScript(call.unlockingScript)
        }

        await this.verifyAndSetTokenOp2({
          tx: cancelTx,
          tokenInputIdx: 1,
          tokenIndexInCheck: 0,
          amountCheckHashIndex: 0,
          checkInputIdx: 2,
          checkTx: prepTx,
          checkVout: 0,
          tokenTx,
          tokenVout: params.tokenVout!,
          contractInputIdx: 0,
          contractTx: orderTx,
          contractVout: params.orderVout,
          localTxMap: localMap,
        })

        this.verifyAndSetUnlockCheck(cancelTx, 2, unlockCheckInst, DUST, [tokenTx], [params.tokenVout!], [0])
        applyP2PKHSig(cancelTx, 3, getSigOrThrow(sigMap, 0, 3))

        const txHexes = [prepTx.uncheckedSerialize(), cancelTx.uncheckedSerialize()]
        const txids: string[] = []
        if (broadcast) {
          txids.push(await this.chain.broadcastTx(txHexes[0]))
          txids.push(await this.chain.broadcastTx(txHexes[1]))
        } else {
          txids.push(signedPrepTxid, getMvcV10TxId(cancelTx))
        }
        return {
          draftId: '',
          action: 'cancel_ask',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders: [],
        }
      }
    )
  }

  // =========================================================================
  // 6. PLACE FT/FT TOKEN ORDER (OrderTokenV2)
  // Step 0: PrepTx (P2PKH SPACE) -> OrderTokenV2(546) + TokenTransferCheck(546) + lockFeeUtxo
  // Step 1: LockTokenTx (sellToken op=1 + TransferCheck + lockFeeUtxo) -> sellToken locked to hash160(OrderTokenV2)
  // =========================================================================
  async buildPlaceTokenOrder(params: PlaceTokenOrderParams): Promise<DraftBuildResult> {
    const feeRate = params.feeRate ?? this.defaultFeeRate
    const makerHash20 = addressToHash20(params.makerAddress, this.net)
    const makerP2pkh = p2pkhScript(params.makerAddress, this.net)
    const sellCodeHashBuf = Buffer.from(params.sellTokenCodeHash, 'hex')
    const sellTokenIDBuf = Buffer.from(params.sellTokenID, 'hex')
    const buyCodeHashBuf = Buffer.from(params.buyTokenCodeHash, 'hex')
    const buyTokenIDBuf = Buffer.from(params.buyTokenID, 'hex')
    if (sellCodeHashBuf.equals(buyCodeHashBuf) && sellTokenIDBuf.equals(buyTokenIDBuf)) {
      throw new Error('sellToken and buyToken must be distinct')
    }

    const sellAmount = BigInt(params.sellAmount)
    const priceNum = BigInt(params.priceNum)
    const priceDen = BigInt(params.priceDen)
    const minFillAmount = BigInt(params.minFillAmount)
    if (sellAmount <= BigInt(0) || priceNum <= BigInt(0) || priceDen <= BigInt(0)) {
      throw new Error('sellAmount, priceNum, and priceDen must be positive')
    }
    if (minFillAmount <= BigInt(0) || minFillAmount > sellAmount) {
      throw new Error('minFillAmount must be in (0, sellAmount]')
    }
    if (params.feeBps < 0 || params.feeBps > 500) {
      throw new Error('feeBps must be between 0 and 500')
    }
    const feeAddrBuf = params.feeAddress ? addressToHash20(params.feeAddress, this.net) : OrderTokenProto.NULL_ADDRESS
    const saltBuf = params.saltHex ? Buffer.from(params.saltHex, 'hex') : crypto.randomBytes(8)
    if (saltBuf.length !== 8) throw new Error('saltHex must be 8 bytes')

    const orderDataPart = OrderTokenProto.buildOrderTokenDataPart({
      salt: saltBuf,
      feeBps: params.feeBps,
      feeAddr: feeAddrBuf,
      minFillAmount,
      remainAmount: sellAmount,
      priceDen,
      priceNum,
      buyTokenCodeHash: buyCodeHashBuf,
      buyTokenID: buyTokenIDBuf,
      sellTokenCodeHash: sellCodeHashBuf,
      sellTokenID: sellTokenIDBuf,
      makerAddr: makerHash20,
    })
    const orderInst = new this.contracts.OrderTokenV2(this.contracts.askUnlockCheckHashes)
    orderInst.setDataPart(orderDataPart.toString('hex'))
    const orderLockScript = orderInst.lockingScript
    const orderHash20 = mvc.crypto.Hash.sha256ripemd160(orderLockScript.toBuffer())

    const [tokenUtxos, mvcUtxos] = await Promise.all([
      this.chain.getTokenUtxos(params.makerAddress, params.sellTokenCodeHash, params.sellTokenID),
      this.chain.getMvcUtxos(params.makerAddress),
    ])
    const { selected: selTokens, total: totalTok } = selectTokenUtxos(tokenUtxos, sellAmount, 3)
    const tokChange = totalTok - sellAmount

    const recAmounts = tokChange > BigInt(0) ? [sellAmount, tokChange] : [sellAmount]
    const recAddrs = tokChange > BigInt(0) ? [orderHash20, makerHash20] : [orderHash20]
    const transferCheckInst = SwapEngine.buildTokenTransferCheck(
      this.contracts,
      sellTokenIDBuf,
      sellCodeHashBuf,
      selTokens.length,
      recAmounts,
      recAddrs
    )

    const lockFeeBudget = Math.max(8000, Math.ceil((3500 + selTokens.length * 1600) * feeRate))
    const prepFeeBudget = Math.max(2500, Math.ceil(1600 * feeRate))
    const lockUtxoSats = lockFeeBudget + DUST
    const needPrepSats = DUST + DUST + lockUtxoSats + prepFeeBudget
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, needPrepSats)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    prepTx.addOutput(new mvc.Transaction.Output({ script: orderLockScript, satoshis: DUST }))
    prepTx.addOutput(new mvc.Transaction.Output({ script: transferCheckInst.lockingScript, satoshis: DUST }))
    prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: lockUtxoSats }))
    const prepChange = totalMvc - needPrepSats
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.makerAddress,
      kind: 'p2pkh',
      message: 'Sign Place Token Order prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''
    let lockTx: any = null
    let tokenTxs: any[] = []
    let actualLockChange = 0
    const checkInputIdx = selTokens.length
    const feeInputIdx = selTokens.length + 1

    return this.saveDraft(
      'place_token',
      2,
      step0Reqs,
      {
        side: 'token',
        sellTokenCodeHash: params.sellTokenCodeHash,
        sellTokenID: params.sellTokenID,
        buyTokenCodeHash: params.buyTokenCodeHash,
        buyTokenID: params.buyTokenID,
        sellAmount: sellAmount.toString(),
        priceNum: priceNum.toString(),
        priceDen: priceDen.toString(),
        minFillAmount: minFillAmount.toString(),
        feeBps: params.feeBps,
        saltHex: saltBuf.toString('hex'),
        estimatedMinerFee: prepFeeBudget + lockFeeBudget,
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          tokenTxs = await Promise.all(selTokens.map((u) => this.getTxCached(u.txid)))
          const sampleTokBuf = tokenTxs[0].outputs[selTokens[0].vout].script.toBuffer()

          lockTx = new mvc.Transaction()
          lockTx.version = Common.TX_VERSION
          for (let i = 0; i < selTokens.length; i++) {
            const u = selTokens[i]
            addTxInput(lockTx, u.txid, u.vout, tokenTxs[i].outputs[u.vout].script, u.satoshis)
          }
          addTxInput(lockTx, signedPrepTxid, 1, transferCheckInst.lockingScript, DUST)
          addTxInput(lockTx, signedPrepTxid, 2, makerP2pkh, lockUtxoSats)

          const lockedTokBuf = TokenProto.getNewTokenScript(sampleTokBuf, orderHash20, sellAmount)
          lockTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(lockedTokBuf), satoshis: DUST }))
          if (tokChange > BigInt(0)) {
            const chgTokBuf = TokenProto.getNewTokenScript(sampleTokBuf, makerHash20, tokChange)
            lockTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(chgTokBuf), satoshis: DUST }))
          }
          const lockInSats = selTokens.reduce((a, u) => a + u.satoshis, 0) + DUST + lockUtxoSats
          const lockOutTokSats = DUST * recAmounts.length
          actualLockChange = lockInSats - lockOutTokSats - lockFeeBudget
          if (actualLockChange >= DUST) {
            lockTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: actualLockChange }))
          } else {
            actualLockChange = 0
          }

          const lockHex = lockTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = []
          for (let i = 0; i < selTokens.length; i++) {
            step1Reqs.push({
              txIndex: 0,
              inputIndex: i,
              txHex: lockHex,
              scriptHex: tokenTxs[i].outputs[selTokens[i].vout].script.toHex(),
              satoshis: selTokens[i].satoshis,
              sigtype: SIG_ALL,
              address: params.makerAddress,
              kind: 'token_p2pkh',
              message: `Sign sellToken lock input #${i}`,
            })
          }
          step1Reqs.push({
            txIndex: 0,
            inputIndex: feeInputIdx,
            txHex: lockHex,
            scriptHex: makerP2pkh.toHex(),
            satoshis: lockUtxoSats,
            sigtype: SIG_ALL,
            address: params.makerAddress,
            kind: 'p2pkh',
            message: 'Sign Place Token Order lock fee input',
          })

          step = 1
          return {
            draftId: '',
            action: 'place_token',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock sellToken inputs + TransferCheck + fee P2PKH
        const localMap = new Map<string, any>()
        localMap.set(signedPrepTxid.toLowerCase(), prepTx)
        for (let i = 0; i < selTokens.length; i++) {
          await this.verifyAndSetTokenOp1({
            tx: lockTx,
            tokenInputIdx: i,
            tokenIndexInCheck: i,
            tokenTx: tokenTxs[i],
            tokenVout: selTokens[i].vout,
            checkInputIdx,
            checkTx: prepTx,
            checkVout: 1,
            userSig: getSigOrThrow(sigMap, 0, i),
            localTxMap: localMap,
          })
        }
        this.verifyAndSetTransferCheck(
          lockTx,
          checkInputIdx,
          transferCheckInst,
          DUST,
          tokenTxs,
          selTokens.map((u) => u.vout),
          DUST,
          actualLockChange,
          makerHash20
        )
        applyP2PKHSig(lockTx, feeInputIdx, getSigOrThrow(sigMap, 0, feeInputIdx))

        const txHexes = [prepTx.uncheckedSerialize(), lockTx.uncheckedSerialize()]
        const txids: string[] = []
        if (broadcast) {
          txids.push(await this.chain.broadcastTx(txHexes[0]))
          txids.push(await this.chain.broadcastTx(txHexes[1]))
        } else {
          txids.push(signedPrepTxid, getMvcV10TxId(lockTx))
        }

        return {
          draftId: '',
          action: 'place_token',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders: [
            {
              side: 'token',
              txid: txids[0],
              vout: 0,
              satoshis: DUST,
              scriptHex: orderLockScript.toHex(),
              remainAmount: sellAmount.toString(),
              tokenTxid: txids[1],
              tokenVout: 0,
              saltHex: saltBuf.toString('hex'),
            },
          ],
        }
      }
    )
  }

  // =========================================================================
  // 7. TAKE FT/FT TOKEN ORDER(S) (OrderTokenV2 + OrderTokenFiller, 0-sig SettleTx!)
  // Step 0: PrepTx (P2PKH SPACE) -> buyTransferCheck + depositFeeUtxo + per-leg [OrderTokenFiller(fillerSats), sellUnlockCheck(546), buyUnlockCheck(546)]
  // Step 1: DepositTx (buyToken op=1 + buyTransferCheck + depositFeeUtxo) -> per-leg buyToken locked to OrderTokenFiller + buyToken change
  // Auto 0-sig SettleTx[0..N-1]: 6 inputs [OrderTokenV2, sellToken, sellUnlockCheck, OrderTokenFiller, buyToken, buyUnlockCheck]
  // =========================================================================
  async buildTakeTokenOrder(params: TakeTokenOrderParams): Promise<DraftBuildResult> {
    if (!params.legs || params.legs.length === 0) {
      throw new Error('At least one token order leg is required')
    }
    const feeRate = params.feeRate ?? this.defaultFeeRate
    const takerHash20 = addressToHash20(params.takerAddress, this.net)
    const takerP2pkh = p2pkhScript(params.takerAddress, this.net)
    const buyCodeHashBuf = Buffer.from(params.buyTokenCodeHash, 'hex')
    const buyTokenIDBuf = Buffer.from(params.buyTokenID, 'hex')

    interface PreparedTokenLeg {
      leg: TakeTokenOrderLeg
      orderTx: any
      sellTokenTx: any
      orderInst: any
      orderScriptBuf: Buffer
      orderData: any
      fillSellAmount: bigint
      remainNext: bigint
      buyAmount: bigint
      feeAmount: bigint
      hasFeeOut: boolean
      totalLegBuyAmount: bigint
      newOrderScriptBuf?: Buffer
      newOrderHash20?: Buffer
      fillerInst: any
      fillerHash20: Buffer
      fillerSats: number
      settleFeeBudget: number
      sellUnlockCheckInst: any
      buyUnlockCheckInst: any
      fillerVoutInPrep: number
      sellCheckVoutInPrep: number
      buyCheckVoutInPrep: number
      buyTokenVoutInDeposit: number
    }

    const prepared: PreparedTokenLeg[] = []
    let totalBuyAmountNeeded = BigInt(0)
    let totalFillSellAmount = BigInt(0)
    let totalFeeAmount = BigInt(0)
    let prepFillerAndCheckSats = 0

    for (const leg of params.legs) {
      const [orderRaw, sellTokenRaw] = await Promise.all([
        this.chain.getRawTx(leg.orderTxid),
        this.chain.getRawTx(leg.tokenTxid),
      ])
      const orderTx = new mvc.Transaction(orderRaw)
      const sellTokenTx = new mvc.Transaction(sellTokenRaw)
      const orderScriptBuf = orderTx.outputs[leg.orderVout].script.toBuffer()
      const orderData = OrderTokenProto.parseOrderTokenScript(orderScriptBuf)

      if (
        !orderData.buyTokenCodeHash.equals(buyCodeHashBuf) ||
        !orderData.buyTokenID.equals(buyTokenIDBuf)
      ) {
        throw new Error(`Order ${leg.orderTxid}:${leg.orderVout} buyToken mismatch`)
      }

      const orderInst = new this.contracts.OrderTokenV2(this.contracts.askUnlockCheckHashes)
      orderInst.setDataPart(
        orderScriptBuf.subarray(orderScriptBuf.length - OrderTokenProto.RAW_DATA_LEN).toString('hex')
      )
      if (!orderInst.lockingScript.toBuffer().equals(orderScriptBuf)) {
        throw new Error(`OrderTokenV2 script mismatch for ${leg.orderTxid}:${leg.orderVout}`)
      }

      const fillSellAmount = BigInt(leg.fillSellAmount)
      if (fillSellAmount <= BigInt(0) || fillSellAmount > orderData.remainAmount) {
        throw new Error(`Invalid fillSellAmount ${fillSellAmount} for remainAmount ${orderData.remainAmount}`)
      }
      const remainNext = orderData.remainAmount - fillSellAmount
      if (
        remainNext > BigInt(0) &&
        (fillSellAmount < orderData.minFillAmount || remainNext < orderData.minFillAmount)
      ) {
        throw new Error(`Partial fill violates minFillAmount (${orderData.minFillAmount})`)
      }

      const buyAmount = OrderTokenProto.calcBuyTokenAmount(fillSellAmount, orderData.priceNum, orderData.priceDen)
      if (buyAmount <= BigInt(0)) {
        throw new Error(`Calculated buyAmount must be positive`)
      }
      const feeAmount = OrderTokenProto.calcFeeAmount(buyAmount, orderData.feeBps)
      const hasFeeOut = !orderData.feeAddr.equals(OrderTokenProto.NULL_ADDRESS) && feeAmount > BigInt(0)
      const totalLegBuyAmount = buyAmount + (hasFeeOut ? feeAmount : BigInt(0))

      let newOrderScriptBuf: Buffer | undefined
      let newOrderHash20: Buffer | undefined
      const sellRecAmounts: bigint[] = []
      const sellRecAddrs: Buffer[] = []
      if (remainNext > BigInt(0)) {
        newOrderScriptBuf = OrderTokenProto.getNewOrderScript(orderScriptBuf, remainNext)
        newOrderHash20 = mvc.crypto.Hash.sha256ripemd160(newOrderScriptBuf)
        sellRecAmounts.push(remainNext, fillSellAmount)
        sellRecAddrs.push(newOrderHash20, takerHash20)
      } else {
        sellRecAmounts.push(fillSellAmount)
        sellRecAddrs.push(takerHash20)
      }

      const buyRecAmounts: bigint[] = hasFeeOut ? [buyAmount, feeAmount] : [buyAmount]
      const buyRecAddrs: Buffer[] = hasFeeOut ? [orderData.makerAddr, orderData.feeAddr] : [orderData.makerAddr]

      const fillerInst = new this.contracts.OrderTokenFiller(
        new Ripemd160(takerHash20.toString('hex')),
        new Bytes(orderData.sellTokenID.toString('hex')),
        new Bytes(orderData.sellTokenCodeHash.toString('hex')),
        fillSellAmount
      )
      const fillerHash20 = mvc.crypto.Hash.sha256ripemd160(fillerInst.lockingScript.toBuffer())

      const sellUnlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
        this.contracts,
        orderData.sellTokenID,
        orderData.sellTokenCodeHash,
        [1],
        sellRecAmounts,
        sellRecAddrs,
        false
      )
      const buyUnlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
        this.contracts,
        orderData.buyTokenID,
        orderData.buyTokenCodeHash,
        [4],
        buyRecAmounts,
        buyRecAddrs,
        false
      )

      const outCount = (remainNext > BigInt(0) ? 2 : 0) + 1 + (hasFeeOut ? 1 : 0) + 1
      const settleFeeBudget = Math.max(24000, Math.ceil(20000 * feeRate))
      const inOtherSats =
        orderTx.outputs[leg.orderVout].satoshis +
        sellTokenTx.outputs[leg.tokenVout].satoshis +
        DUST * 3 // sellCheck + buyToken + buyCheck
      const fillerSats = Math.max(DUST, outCount * DUST + settleFeeBudget - inOtherSats)

      prepared.push({
        leg,
        orderTx,
        sellTokenTx,
        orderInst,
        orderScriptBuf,
        orderData,
        fillSellAmount,
        remainNext,
        buyAmount,
        feeAmount,
        hasFeeOut,
        totalLegBuyAmount,
        newOrderScriptBuf,
        newOrderHash20,
        fillerInst,
        fillerHash20,
        fillerSats,
        settleFeeBudget,
        sellUnlockCheckInst,
        buyUnlockCheckInst,
        fillerVoutInPrep: 0,
        sellCheckVoutInPrep: 0,
        buyCheckVoutInPrep: 0,
        buyTokenVoutInDeposit: 0,
      })

      totalBuyAmountNeeded += totalLegBuyAmount
      totalFillSellAmount += fillSellAmount
      if (hasFeeOut) totalFeeAmount += feeAmount
      prepFillerAndCheckSats += fillerSats + DUST * 2
    }

    const [buyTokenUtxos, mvcUtxos] = await Promise.all([
      this.chain.getTokenUtxos(params.takerAddress, params.buyTokenCodeHash, params.buyTokenID),
      this.chain.getMvcUtxos(params.takerAddress),
    ])
    const { selected: selBuyTokens, total: totalBuyTok } = selectTokenUtxos(buyTokenUtxos, totalBuyAmountNeeded, 3)
    const buyTokChange = totalBuyTok - totalBuyAmountNeeded

    const depositRecAmounts: bigint[] = prepared.map((p) => p.totalLegBuyAmount)
    const depositRecAddrs: Buffer[] = prepared.map((p) => p.fillerHash20)
    if (buyTokChange > BigInt(0)) {
      depositRecAmounts.push(buyTokChange)
      depositRecAddrs.push(takerHash20)
    }

    const buyTransferCheckInst = SwapEngine.buildTokenTransferCheck(
      this.contracts,
      buyTokenIDBuf,
      buyCodeHashBuf,
      selBuyTokens.length,
      depositRecAmounts,
      depositRecAddrs
    )

    const depositFeeBudget = Math.max(8500, Math.ceil((3800 + selBuyTokens.length * 1600) * feeRate))
    const depositTokenOutDust = DUST * depositRecAmounts.length
    const depositTokenInDust = selBuyTokens.reduce((a, u) => a + u.satoshis, 0) + DUST
    const depositFeeUtxoSats = Math.max(
      depositFeeBudget + DUST,
      depositFeeBudget + Math.max(0, depositTokenOutDust - depositTokenInDust) + DUST
    )
    const prepFeeBudget = Math.max(3000, Math.ceil((2000 + prepared.length * 500) * feeRate))
    const prepNeedSats = DUST + depositFeeUtxoSats + prepFillerAndCheckSats + prepFeeBudget
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, prepNeedSats)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    const buyTransferCheckVout = 0
    prepTx.addOutput(new mvc.Transaction.Output({ script: buyTransferCheckInst.lockingScript, satoshis: DUST }))
    const depositFeeVout = 1
    prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: depositFeeUtxoSats }))
    for (const p of prepared) {
      p.fillerVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.fillerInst.lockingScript, satoshis: p.fillerSats }))
      p.sellCheckVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.sellUnlockCheckInst.lockingScript, satoshis: DUST }))
      p.buyCheckVoutInPrep = prepTx.outputs.length
      prepTx.addOutput(new mvc.Transaction.Output({ script: p.buyUnlockCheckInst.lockingScript, satoshis: DUST }))
    }
    const prepChange = totalMvc - prepNeedSats
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: prepChange }))
    }

    const prepHex = prepTx.uncheckedSerialize()
    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepHex,
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.takerAddress,
      kind: 'p2pkh',
      message: 'Sign Take Token Order prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''
    let depositTx: any = null
    let buyTokenTxs: any[] = []
    let actualDepositChange = 0
    const checkInputIdx = selBuyTokens.length
    const feeInputIdx = selBuyTokens.length + 1

    return this.saveDraft(
      'take_token',
      2,
      step0Reqs,
      {
        side: 'token',
        legs: prepared.length,
        totalFillSellAmount: totalFillSellAmount.toString(),
        totalBuyAmount: (totalBuyAmountNeeded - totalFeeAmount).toString(),
        totalFeeAmount: totalFeeAmount.toString(),
        totalPayBuyAmount: totalBuyAmountNeeded.toString(),
        estimatedMinerFee:
          prepFeeBudget + depositFeeBudget + prepared.reduce((a, p) => a + p.settleFeeBudget, 0),
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          buyTokenTxs = await Promise.all(selBuyTokens.map((u) => this.getTxCached(u.txid)))
          const sampleBuyTokBuf = buyTokenTxs[0].outputs[selBuyTokens[0].vout].script.toBuffer()

          depositTx = new mvc.Transaction()
          depositTx.version = Common.TX_VERSION
          for (let i = 0; i < selBuyTokens.length; i++) {
            const u = selBuyTokens[i]
            addTxInput(depositTx, u.txid, u.vout, buyTokenTxs[i].outputs[u.vout].script, u.satoshis)
          }
          addTxInput(depositTx, signedPrepTxid, buyTransferCheckVout, buyTransferCheckInst.lockingScript, DUST)
          addTxInput(depositTx, signedPrepTxid, depositFeeVout, takerP2pkh, depositFeeUtxoSats)

          for (const p of prepared) {
            p.buyTokenVoutInDeposit = depositTx.outputs.length
            const legBuyBuf = TokenProto.getNewTokenScript(sampleBuyTokBuf, p.fillerHash20, p.totalLegBuyAmount)
            depositTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(legBuyBuf), satoshis: DUST }))
          }
          if (buyTokChange > BigInt(0)) {
            const chgBuyBuf = TokenProto.getNewTokenScript(sampleBuyTokBuf, takerHash20, buyTokChange)
            depositTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(chgBuyBuf), satoshis: DUST }))
          }
          const depInSats = selBuyTokens.reduce((a, u) => a + u.satoshis, 0) + DUST + depositFeeUtxoSats
          const depOutTokSats = DUST * depositRecAmounts.length
          actualDepositChange = depInSats - depOutTokSats - depositFeeBudget
          if (actualDepositChange >= DUST) {
            depositTx.addOutput(new mvc.Transaction.Output({ script: takerP2pkh, satoshis: actualDepositChange }))
          } else {
            actualDepositChange = 0
          }

          const depHex = depositTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = []
          for (let i = 0; i < selBuyTokens.length; i++) {
            step1Reqs.push({
              txIndex: 0,
              inputIndex: i,
              txHex: depHex,
              scriptHex: buyTokenTxs[i].outputs[selBuyTokens[i].vout].script.toHex(),
              satoshis: selBuyTokens[i].satoshis,
              sigtype: SIG_ALL,
              address: params.takerAddress,
              kind: 'token_p2pkh',
              message: `Sign Take Token Order buyToken input #${i}`,
            })
          }
          step1Reqs.push({
            txIndex: 0,
            inputIndex: feeInputIdx,
            txHex: depHex,
            scriptHex: takerP2pkh.toHex(),
            satoshis: depositFeeUtxoSats,
            sigtype: SIG_ALL,
            address: params.takerAddress,
            kind: 'p2pkh',
            message: 'Sign Take Token Order deposit fee input',
          })

          step = 1
          return {
            draftId: '',
            action: 'take_token',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        // Step 1 finalize: unlock DepositTx + build/verify 0-sig SettleTx[0..N-1]
        const localMap = new Map<string, any>()
        localMap.set(signedPrepTxid.toLowerCase(), prepTx)
        for (let i = 0; i < buyTokenTxs.length; i++) {
          localMap.set(selBuyTokens[i].txid.toLowerCase(), buyTokenTxs[i])
        }

        for (let i = 0; i < selBuyTokens.length; i++) {
          await this.verifyAndSetTokenOp1({
            tx: depositTx,
            tokenInputIdx: i,
            tokenIndexInCheck: i,
            tokenTx: buyTokenTxs[i],
            tokenVout: selBuyTokens[i].vout,
            checkInputIdx,
            checkTx: prepTx,
            checkVout: buyTransferCheckVout,
            userSig: getSigOrThrow(sigMap, 0, i),
            localTxMap: localMap,
          })
        }
        this.verifyAndSetTransferCheck(
          depositTx,
          checkInputIdx,
          buyTransferCheckInst,
          DUST,
          buyTokenTxs,
          selBuyTokens.map((u) => u.vout),
          DUST,
          actualDepositChange,
          takerHash20
        )
        applyP2PKHSig(depositTx, feeInputIdx, getSigOrThrow(sigMap, 0, feeInputIdx))

        const signedDepositTxid = getMvcV10TxId(depositTx)
        localMap.set(signedDepositTxid.toLowerCase(), depositTx)

        const txHexes: string[] = [prepTx.uncheckedSerialize(), depositTx.uncheckedSerialize()]
        const settleTxs: any[] = []
        const newOrders: NewOrderOutputInfo[] = []

        for (const p of prepared) {
          localMap.set(p.leg.orderTxid.toLowerCase(), p.orderTx)
          localMap.set(p.leg.tokenTxid.toLowerCase(), p.sellTokenTx)

          const sellTokOut = p.sellTokenTx.outputs[p.leg.tokenVout]
          const buyTokOut = depositTx.outputs[p.buyTokenVoutInDeposit]
          const sellTokBuf = sellTokOut.script.toBuffer()
          const buyTokBuf = buyTokOut.script.toBuffer()

          const settleTx = new mvc.Transaction()
          settleTx.version = Common.TX_VERSION
          // 0: OrderTokenV2, 1: sellToken, 2: sellUnlockCheck, 3: OrderTokenFiller, 4: buyToken, 5: buyUnlockCheck
          addTxInput(settleTx, p.leg.orderTxid, p.leg.orderVout, mvc.Script.fromBuffer(p.orderScriptBuf), p.orderTx.outputs[p.leg.orderVout].satoshis)
          addTxInput(settleTx, p.leg.tokenTxid, p.leg.tokenVout, sellTokOut.script, sellTokOut.satoshis)
          addTxInput(settleTx, signedPrepTxid, p.sellCheckVoutInPrep, p.sellUnlockCheckInst.lockingScript, DUST)
          addTxInput(settleTx, signedPrepTxid, p.fillerVoutInPrep, p.fillerInst.lockingScript, p.fillerSats)
          addTxInput(settleTx, signedDepositTxid, p.buyTokenVoutInDeposit, buyTokOut.script, buyTokOut.satoshis)
          addTxInput(settleTx, signedPrepTxid, p.buyCheckVoutInPrep, p.buyUnlockCheckInst.lockingScript, DUST)

          let remainSellTokBuf = Buffer.alloc(0)
          if (p.remainNext > BigInt(0)) {
            settleTx.addOutput(
              new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(p.newOrderScriptBuf!), satoshis: DUST })
            )
            remainSellTokBuf = TokenProto.getNewTokenScript(sellTokBuf, p.newOrderHash20!, p.remainNext)
            settleTx.addOutput(
              new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(remainSellTokBuf), satoshis: DUST })
            )
          }

          const makerBuyTokBuf = TokenProto.getNewTokenScript(buyTokBuf, p.orderData.makerAddr, p.buyAmount)
          settleTx.addOutput(
            new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(makerBuyTokBuf), satoshis: DUST })
          )

          let feeBuyTokBuf = Buffer.alloc(0)
          if (p.hasFeeOut) {
            feeBuyTokBuf = TokenProto.getNewTokenScript(buyTokBuf, p.orderData.feeAddr, p.feeAmount)
            settleTx.addOutput(
              new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(feeBuyTokBuf), satoshis: DUST })
            )
          }

          const takerSellTokIndex = settleTx.outputs.length
          const takerSellTokBuf = TokenProto.getNewTokenScript(sellTokBuf, takerHash20, p.fillSellAmount)
          settleTx.addOutput(
            new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(takerSellTokBuf), satoshis: DUST })
          )

          const prevoutsBuf = buildPrevouts(settleTx)
          const prevOutputsBeforeTaker = Buffer.concat(
            settleTx.outputs.slice(0, takerSellTokIndex).map((o: any) => o.toBufferWriter().toBuffer())
          )
          const otherOutputsForOrder = settleTx.outputs[takerSellTokIndex].toBufferWriter().toBuffer()

          const sellTokProof = Common.getTxOutputProof(p.sellTokenTx, p.leg.tokenVout)
          const sellCheckProof = Common.getTxOutputProof(prepTx, p.sellCheckVoutInPrep)
          const buyTokProof = Common.getTxOutputProof(depositTx, p.buyTokenVoutInDeposit)

          // Input 0: OrderTokenV2 (OP_FILL)
          const orderPreimage = getPreimage(
            settleTx,
            p.orderInst.lockingScript,
            p.orderTx.outputs[p.leg.orderVout].satoshis,
            0,
            SIG_ALL
          )
          const orderCall = p.orderInst.unlock(
            new SigHashPreimage(toHex(orderPreimage)),
            new Bytes(prevoutsBuf.toString('hex')),
            new Bytes(sellTokBuf.toString('hex')),
            sellTokProof.txHeader,
            sellTokProof.hashProof,
            sellTokProof.satoshiBytes,
            0,
            new Bytes(p.sellUnlockCheckInst.lockingScript.toBuffer().toString('hex')),
            sellCheckProof.txHeader,
            sellCheckProof.hashProof,
            sellCheckProof.satoshiBytes,
            4,
            new Bytes(buyTokBuf.toString('hex')),
            buyTokProof.txHeader,
            buyTokProof.hashProof,
            buyTokProof.satoshiBytes,
            p.fillSellAmount,
            DUST,
            new Bytes(otherOutputsForOrder.toString('hex')),
            new PubKey(ZERO_PUBKEY_HEX),
            new Sig(ZERO_SIG_HEX),
            OrderTokenProto.OP_FILL
          )
          const v0 = orderCall.verify({
            tx: settleTx,
            inputIndex: 0,
            inputSatoshis: p.orderTx.outputs[p.leg.orderVout].satoshis,
          })
          if (!v0.success) throw new Error(`OrderTokenV2 unlock verify failed: ${v0.error}`)
          settleTx.inputs[0].setScript(orderCall.unlockingScript)

          // Input 1: sellToken (op=2 contract unlock by OrderTokenV2)
          await this.verifyAndSetTokenOp2({
            tx: settleTx,
            tokenInputIdx: 1,
            tokenIndexInCheck: 0,
            amountCheckHashIndex: 0,
            checkInputIdx: 2,
            checkTx: prepTx,
            checkVout: p.sellCheckVoutInPrep,
            tokenTx: p.sellTokenTx,
            tokenVout: p.leg.tokenVout,
            contractInputIdx: 0,
            contractTx: p.orderTx,
            contractVout: p.leg.orderVout,
            localTxMap: localMap,
          })

          // Input 2: sellUnlockCheck
          const sellOutputIndexes = p.remainNext > BigInt(0) ? [1, takerSellTokIndex] : [takerSellTokIndex]
          this.verifyAndSetUnlockCheck(
            settleTx,
            2,
            p.sellUnlockCheckInst,
            DUST,
            [p.sellTokenTx],
            [p.leg.tokenVout],
            sellOutputIndexes
          )

          // Input 3: OrderTokenFiller (0-sig OP_SETTLE)
          const fillerPreimage = getPreimage(settleTx, p.fillerInst.lockingScript, p.fillerSats, 3, SIG_ALL)
          const fillerCall = p.fillerInst.unlock(
            new SigHashPreimage(toHex(fillerPreimage)),
            new Bytes(prevoutsBuf.toString('hex')),
            1,
            new Bytes(sellTokBuf.toString('hex')),
            sellTokProof.txHeader,
            sellTokProof.hashProof,
            sellTokProof.satoshiBytes,
            p.fillSellAmount,
            DUST,
            new Bytes(prevOutputsBeforeTaker.toString('hex')),
            new Bytes(''),
            new PubKey(ZERO_PUBKEY_HEX),
            new Sig(ZERO_SIG_HEX),
            1 // OP_SETTLE
          )
          const v3 = fillerCall.verify({ tx: settleTx, inputIndex: 3, inputSatoshis: p.fillerSats })
          if (!v3.success) throw new Error(`OrderTokenFiller OP_SETTLE verify failed: ${v3.error}`)
          settleTx.inputs[3].setScript(fillerCall.unlockingScript)

          // Input 4: buyToken (op=2 contract unlock by OrderTokenFiller)
          await this.verifyAndSetTokenOp2({
            tx: settleTx,
            tokenInputIdx: 4,
            tokenIndexInCheck: 0,
            amountCheckHashIndex: 0,
            checkInputIdx: 5,
            checkTx: prepTx,
            checkVout: p.buyCheckVoutInPrep,
            tokenTx: depositTx,
            tokenVout: p.buyTokenVoutInDeposit,
            contractInputIdx: 3,
            contractTx: prepTx,
            contractVout: p.fillerVoutInPrep,
            localTxMap: localMap,
          })

          // Input 5: buyUnlockCheck
          const makerBuyTokIndex = p.remainNext > BigInt(0) ? 2 : 0
          const buyOutputIndexes = p.hasFeeOut ? [makerBuyTokIndex, makerBuyTokIndex + 1] : [makerBuyTokIndex]
          this.verifyAndSetUnlockCheck(
            settleTx,
            5,
            p.buyUnlockCheckInst,
            DUST,
            [depositTx],
            [p.buyTokenVoutInDeposit],
            buyOutputIndexes
          )

          const settleTxid = getMvcV10TxId(settleTx)
          settleTxs.push(settleTx)
          txHexes.push(settleTx.uncheckedSerialize())
          if (p.remainNext > BigInt(0)) {
            newOrders.push({
              side: 'token',
              txid: settleTxid,
              vout: 0,
              satoshis: DUST,
              scriptHex: p.newOrderScriptBuf!.toString('hex'),
              remainAmount: p.remainNext.toString(),
              tokenTxid: settleTxid,
              tokenVout: 1,
              saltHex: p.orderData.salt.toString('hex'),
            })
          }
        }

        const txids: string[] = []
        if (broadcast) {
          for (const hex of txHexes) {
            txids.push(await this.chain.broadcastTx(hex))
          }
        } else {
          txids.push(signedPrepTxid, signedDepositTxid, ...settleTxs.map((t) => getMvcV10TxId(t)))
        }

        return {
          draftId: '',
          action: 'take_token',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders,
        }
      }
    )
  }

  // =========================================================================
  // 8. CANCEL FT/FT TOKEN ORDER (OrderTokenV2 OP_CANCEL)
  // Step 0: PrepTx (P2PKH SPACE) -> sellUnlockCheck(546) + cancelFeeUtxo
  // Step 1: CancelTx -> OrderTokenV2(OP_CANCEL) + sellToken(op=2) + sellUnlockCheck + makerFeeP2pkh
  // =========================================================================
  private async buildCancelTokenOrder(params: CancelOrderParams): Promise<DraftBuildResult> {
    const feeRate = params.feeRate ?? this.defaultFeeRate
    const makerHash20 = addressToHash20(params.makerAddress, this.net)
    const makerP2pkh = p2pkhScript(params.makerAddress, this.net)

    if (!params.tokenTxid || params.tokenVout === undefined) {
      throw new Error('tokenTxid and tokenVout are required to cancel a Token order')
    }

    const [orderRaw, tokenRaw] = await Promise.all([
      this.chain.getRawTx(params.orderTxid),
      this.chain.getRawTx(params.tokenTxid),
    ])
    const orderTx = new mvc.Transaction(orderRaw)
    const tokenTx = new mvc.Transaction(tokenRaw)
    const orderOut = orderTx.outputs[params.orderVout]
    const orderScriptBuf = orderOut.script.toBuffer()
    const orderData = OrderTokenProto.parseOrderTokenScript(orderScriptBuf)

    if (!orderData.makerAddr.equals(makerHash20)) {
      throw new Error('Only the order maker can cancel this token order')
    }

    const tokenOut = tokenTx.outputs[params.tokenVout]
    const lockedTokBuf = tokenOut.script.toBuffer()

    const orderInst = new this.contracts.OrderTokenV2(this.contracts.askUnlockCheckHashes)
    orderInst.setDataPart(
      orderScriptBuf.subarray(orderScriptBuf.length - OrderTokenProto.RAW_DATA_LEN).toString('hex')
    )
    if (!orderInst.lockingScript.toBuffer().equals(orderScriptBuf)) {
      throw new Error(`OrderTokenV2 script mismatch for ${params.orderTxid}:${params.orderVout}`)
    }

    const unlockCheckInst = SwapEngine.buildTokenUnlockContractCheck(
      this.contracts,
      orderData.sellTokenID,
      orderData.sellTokenCodeHash,
      [1],
      [orderData.remainAmount],
      [makerHash20],
      false
    )

    const cancelFeeBudget = Math.max(10000, Math.ceil(8000 * feeRate))
    const prepFeeBudget = Math.max(2500, Math.ceil(1600 * feeRate))
    const mvcUtxos = await this.chain.getMvcUtxos(params.makerAddress)
    const { selected: selMvc, total: totalMvc } = selectMvcUtxos(mvcUtxos, DUST + cancelFeeBudget + prepFeeBudget)

    const prepTx = new mvc.Transaction()
    prepTx.version = Common.TX_VERSION
    for (const u of selMvc) {
      addTxInput(prepTx, u.txid, u.vout, mvc.Script.fromHex(u.scriptHex), u.satoshis)
    }
    prepTx.addOutput(new mvc.Transaction.Output({ script: unlockCheckInst.lockingScript, satoshis: DUST }))
    prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: cancelFeeBudget }))
    const prepChange = totalMvc - DUST - cancelFeeBudget - prepFeeBudget
    if (prepChange >= DUST) {
      prepTx.addOutput(new mvc.Transaction.Output({ script: makerP2pkh, satoshis: prepChange }))
    }

    const step0Reqs: SignRequestItem[] = selMvc.map((u, i) => ({
      txIndex: 0,
      inputIndex: i,
      txHex: prepTx.uncheckedSerialize(),
      scriptHex: u.scriptHex,
      satoshis: u.satoshis,
      sigtype: SIG_ALL,
      address: params.makerAddress,
      kind: 'p2pkh',
      message: 'Sign Cancel Token Order prep transaction',
    }))

    let step = 0
    let signedPrepTxid = ''
    let cancelTx: any = null

    return this.saveDraft(
      'cancel_token',
      2,
      step0Reqs,
      {
        side: 'token',
        orderTxid: params.orderTxid,
        orderVout: params.orderVout,
        returnSellTokenAmount: orderData.remainAmount.toString(),
        estimatedMinerFee: prepFeeBudget + cancelFeeBudget,
      },
      async (sigs, broadcast) => {
        const sigMap = new Map(sigs.map((s) => [`${s.txIndex}:${s.inputIndex}`, s]))

        if (step === 0) {
          for (let i = 0; i < selMvc.length; i++) {
            applyP2PKHSig(prepTx, i, getSigOrThrow(sigMap, 0, i))
          }
          signedPrepTxid = getMvcV10TxId(prepTx)

          cancelTx = new mvc.Transaction()
          cancelTx.version = Common.TX_VERSION
          addTxInput(cancelTx, params.orderTxid, params.orderVout, mvc.Script.fromBuffer(orderScriptBuf), orderOut.satoshis)
          addTxInput(cancelTx, params.tokenTxid!, params.tokenVout!, tokenOut.script, tokenOut.satoshis)
          addTxInput(cancelTx, signedPrepTxid, 0, unlockCheckInst.lockingScript, DUST)
          addTxInput(cancelTx, signedPrepTxid, 1, makerP2pkh, cancelFeeBudget)

          const returnTokBuf = TokenProto.getNewTokenScript(lockedTokBuf, makerHash20, orderData.remainAmount)
          cancelTx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromBuffer(returnTokBuf), satoshis: DUST }))

          const cancelHex = cancelTx.uncheckedSerialize()
          const step1Reqs: SignRequestItem[] = [
            {
              txIndex: 0,
              inputIndex: 0,
              txHex: cancelHex,
              scriptHex: orderScriptBuf.toString('hex'),
              satoshis: orderOut.satoshis,
              sigtype: SIG_ALL,
              address: params.makerAddress,
              kind: 'order_cancel',
              message: 'Sign Token Order cancellation',
            },
            {
              txIndex: 0,
              inputIndex: 3,
              txHex: cancelHex,
              scriptHex: makerP2pkh.toHex(),
              satoshis: cancelFeeBudget,
              sigtype: SIG_ALL,
              address: params.makerAddress,
              kind: 'p2pkh',
              message: 'Sign Token Order cancel fee input',
            },
          ]

          step = 1
          return {
            draftId: '',
            action: 'cancel_token',
            status: 'awaiting_signature',
            step: 1,
            totalSteps: 2,
            signRequests: step1Reqs,
            txids: [],
            txHexes: [],
            verified: false,
            broadcasted: false,
            newOrders: [],
          }
        }

        const localMap = new Map<string, any>()
        localMap.set(signedPrepTxid.toLowerCase(), prepTx)
        localMap.set(params.orderTxid.toLowerCase(), orderTx)
        localMap.set(params.tokenTxid!.toLowerCase(), tokenTx)

        const makerSig = getSigOrThrow(sigMap, 0, 0)
        const orderPreimage = getPreimage(cancelTx, orderInst.lockingScript, orderOut.satoshis, 0, SIG_ALL)
        const orderCall = orderInst.unlock(
          new SigHashPreimage(toHex(orderPreimage)),
          new Bytes(buildPrevouts(cancelTx).toString('hex')),
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          0,
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          0,
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          new Bytes(''),
          BigInt(0),
          0,
          new Bytes(''),
          new PubKey(makerSig.publicKey),
          new Sig(makerSig.sig),
          OrderTokenProto.OP_CANCEL
        )
        const v0 = orderCall.verify({ tx: cancelTx, inputIndex: 0, inputSatoshis: orderOut.satoshis })
        if (!v0.success) throw new Error(`OrderTokenV2 cancel verify failed: ${v0.error}`)
        cancelTx.inputs[0].setScript(orderCall.unlockingScript)

        await this.verifyAndSetTokenOp2({
          tx: cancelTx,
          tokenInputIdx: 1,
          tokenIndexInCheck: 0,
          amountCheckHashIndex: 0,
          checkInputIdx: 2,
          checkTx: prepTx,
          checkVout: 0,
          tokenTx,
          tokenVout: params.tokenVout!,
          contractInputIdx: 0,
          contractTx: orderTx,
          contractVout: params.orderVout,
          localTxMap: localMap,
        })

        this.verifyAndSetUnlockCheck(cancelTx, 2, unlockCheckInst, DUST, [tokenTx], [params.tokenVout!], [0])
        applyP2PKHSig(cancelTx, 3, getSigOrThrow(sigMap, 0, 3))

        const txHexes = [prepTx.uncheckedSerialize(), cancelTx.uncheckedSerialize()]
        const txids: string[] = []
        if (broadcast) {
          txids.push(await this.chain.broadcastTx(txHexes[0]))
          txids.push(await this.chain.broadcastTx(txHexes[1]))
        } else {
          txids.push(signedPrepTxid, getMvcV10TxId(cancelTx))
        }
        return {
          draftId: '',
          action: 'cancel_token',
          status: 'completed',
          txids,
          txHexes,
          verified: true,
          broadcasted: broadcast,
          newOrders: [],
        }
      }
    )
  }
}
