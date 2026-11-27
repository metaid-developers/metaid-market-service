import * as crypto from 'crypto'
import {
  Common,
  LoadedOrderbookContracts,
  OrderProto,
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
  action: 'place_ask' | 'place_bid' | 'take_ask' | 'take_bid' | 'cancel_ask' | 'cancel_bid'
  status: 'awaiting_signature'
  step: number
  totalSteps: number
  signRequests: SignRequestItem[]
  summary: Record<string, any>
  expiresAt: number
}

export interface NewOrderOutputInfo {
  side: 'ask' | 'bid'
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
  side: 'ask' | 'bid'
  orderTxid: string
  orderVout: number
  tokenTxid?: string
  tokenVout?: number
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
      tailLen: OrderProto.DATA_LEN,
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

  private instantiateToken(chainScriptBuf: Buffer): any {
    const rawDataLen = 170 // TokenProto raw payload excluding 2-byte OP_PUSHDATA1 prefix
    const dataHex = chainScriptBuf.subarray(chainScriptBuf.length - rawDataLen).toString('hex')
    const tryMake = (tc: any[], uc: any[]) => {
      const inst = new this.contracts.Token(tc, uc)
      inst.setDataPart(dataHex)
      return inst.lockingScript.toBuffer().equals(chainScriptBuf) ? inst : null
    }
    const direct = tryMake(this.transferCheckHashes, this.unlockCheckHashes)
    if (direct) return direct
    const prod = tryMake(
      PROD_TRANSFER_CHECK_HASHES.map((h) => new Bytes(h)),
      PROD_UNLOCK_CHECK_HASHES.map((h) => new Bytes(h))
    )
    if (prod) return prod
    throw new Error('Unable to instantiate Token contract matching chain lockingScript')
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
    const tokenInst = this.instantiateToken(tokenScriptBuf)
    const backtrace = await this.resolveTokenBacktrace(tokenTx, tokenVout, localTxMap)

    const prevouts = buildPrevouts(tx)
    const preimage = getPreimage(tx, tokenInst.lockingScript, tokenOut.satoshis, tokenInputIdx, SIG_ALL)
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
    const v = call.verify({ tx, inputIndex: tokenInputIdx, inputSatoshis: tokenOut.satoshis })
    if (!v.success) throw new Error(`Token op=1 verify failed at input ${tokenInputIdx}: ${v.error}`)
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
    const tokenInst = this.instantiateToken(tokenScriptBuf)
    const backtrace = await this.resolveTokenBacktrace(tokenTx, tokenVout, localTxMap)

    const prevouts = buildPrevouts(tx)
    const preimage = getPreimage(tx, tokenInst.lockingScript, tokenOut.satoshis, tokenInputIdx, SIG_ALL)
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
    const v = call.verify({ tx, inputIndex: tokenInputIdx, inputSatoshis: tokenOut.satoshis })
    if (!v.success) throw new Error(`Token op=2 verify failed at input ${tokenInputIdx}: ${v.error}`)
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
}
