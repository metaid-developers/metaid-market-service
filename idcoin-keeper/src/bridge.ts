import axios, { AxiosInstance } from 'axios'
import * as crypto from 'crypto'

/**
 * Octopus bridge client for the btc_mint flow. The keeper is an ordinary
 * bridge client: createPrepayOrderMintBtc -> locally build+sign the deposit ->
 * submitPrepayOrderMintBtc (the BRIDGE broadcasts) -> poll orderStatus.
 */

/** DER-encode a compact 64-byte ECDSA sig (r,s) — needed because the bridge's
 *  easiest accepted signature variant is `verifySignatureMvc`: DER/base64 over
 *  sha256(text). tiny-secp256k1 already enforces low-S. */
export function derEncodeSig(compact: Buffer): Buffer {
  const encInt = (x: Buffer): Buffer => {
    let i = 0
    while (i < x.length - 1 && x[i] === 0) i++
    let v = x.subarray(i)
    if (v[0] & 0x80) v = Buffer.concat([Buffer.from([0]), v])
    return v
  }
  const r = encInt(compact.subarray(0, 32))
  const s = encInt(compact.subarray(32, 64))
  const body = Buffer.concat([
    Buffer.from([0x02, r.length]), r,
    Buffer.from([0x02, s.length]), s,
  ])
  return Buffer.concat([Buffer.from([0x30, body.length]), body])
}

/** sign sha256(text) with a compressed-privkey signer; returns base64 DER. */
export function signMessageMvc(signer: { sign: (hash: Buffer) => Buffer }, text: string): string {
  const hash = crypto.createHash('sha256').update(Buffer.from(text, 'utf8')).digest()
  return derEncodeSig(Buffer.from(signer.sign(hash))).toString('base64')
}

export interface FeeQuote {
  btcFeeSats: number
  btcFeeAddress: string
  btcServiceFeeSats: number
  btcBridgeMinerFeeSats: number
  btcPostageSats: number
  btcFeeRate: number
}

export interface AssetPairInfo {
  network: string
  originTokenId: string
  targetTokenId: string
  targetTokenCodeHash: string
  targetTokenGenesis: string
  targetSymbol: string
  decimals: number
  isEnableMint: boolean
}

export interface CreateOrderResult {
  orderId: string
  bridgeAddress: string
  receiveAmount: string
  confirmNumber: number
  btcFeeSats: number
  btcFeeAddress: string
}

export interface OrderStatus {
  orderId: string
  stage: string
  status: string
  sourceTxid: string | null
  targetTxids: string[]
  failureReason: string | null
}

export class BridgeClient {
  private http: AxiosInstance
  constructor(public baseUrl: string, http?: AxiosInstance) {
    this.http = http || axios.create({ timeout: 20_000 })
  }

  async feeQuote(btcFeeRate?: number): Promise<FeeQuote> {
    const resp = await this.http.get(`${this.baseUrl}/feeQuote`, {
      params: { network: 'BTC', direction: 'mint', ...(btcFeeRate ? { btcFeeRate } : {}) },
    })
    const d = resp.data?.data || resp.data
    return {
      btcFeeSats: Number(d.btcFeeSats),
      btcFeeAddress: String(d.btcFeeAddress || ''),
      btcServiceFeeSats: Number(d.btcServiceFeeSats || 0),
      btcBridgeMinerFeeSats: Number(d.btcBridgeMinerFeeSats || 0),
      btcPostageSats: Number(d.btcPostageSats || 0),
      btcFeeRate: Number(d.btcFeeRate || 0),
    }
  }

  async assetList(): Promise<AssetPairInfo[]> {
    const resp = await this.http.get(`${this.baseUrl}/assetList`)
    const d = resp.data?.data || resp.data
    return (d.assetList || []) as AssetPairInfo[]
  }

  async createMintBtcOrder(args: {
    amountSats: number
    publicKey: string            // depositor LP vault pubkey hex
    publicKeyReceive: string     // MVC LP funding pubkey hex
    sign: (text: string) => string
    receiveSign: (text: string) => string
  }): Promise<CreateOrderResult> {
    const body = {
      amount: String(args.amountSats),
      originTokenId: 'btc',
      addressType: 'P2WPKH',
      publicKey: args.publicKey,
      publicKeyReceive: args.publicKeyReceive,
      publicKeySign: args.sign(args.publicKey),
      publicKeyReceiveSign: args.receiveSign(args.publicKeyReceive),
    }
    const resp = await this.http.post(`${this.baseUrl}/createPrepayOrderMintBtc`, body)
    const d = resp.data?.data
    if (!resp.data?.success || !d) {
      throw new Error(`createPrepayOrderMintBtc rejected: ${JSON.stringify(resp.data).slice(0, 300)}`)
    }
    return {
      orderId: String(d.orderId),
      bridgeAddress: String(d.bridgeAddress),
      receiveAmount: String(d.receiveAmount),
      confirmNumber: Number(d.confirmNumber),
      btcFeeSats: Number(d.btcFeeSats),
      btcFeeAddress: String(d.btcFeeAddress || ''),
    }
  }

  /** Returns { ok, msg } — the bridge itself broadcasts the deposit tx. */
  async submitMintBtcOrder(orderId: string, txHex: string): Promise<{ ok: boolean; msg: string }> {
    const resp = await this.http.post(`${this.baseUrl}/submitPrepayOrderMintBtc`, { orderId, txHex })
    const d = resp.data || {}
    return { ok: d.success === true, msg: String(d.msg || '') }
  }

  async orderStatus(orderId: string): Promise<OrderStatus> {
    const resp = await this.http.get(`${this.baseUrl}/orderStatus`, { params: { orderId } })
    const d = resp.data?.data || resp.data
    return {
      orderId: String(d.orderId || orderId),
      stage: String(d.stage || ''),
      status: String(d.status || ''),
      sourceTxid: d.sourceTxid || null,
      targetTxids: Array.isArray(d.targetTxids) ? d.targetTxids.map(String) : [],
      failureReason: d.failureReason || null,
    }
  }
}

/** Validates a create-order response against the pinned security invariants. */
export function validateCreateOrderResponse(args: {
  res: CreateOrderResult
  expectedAmountSats: number
  pinnedBridgeVault: string
  feeAllowlist: Set<string>
  maxBtcFeeSats: number
}): { ok: boolean; reason?: string } {
  const { res } = args
  if (res.bridgeAddress !== args.pinnedBridgeVault) {
    return { ok: false, reason: `bridgeAddress ${res.bridgeAddress} != pinned vault ${args.pinnedBridgeVault}` }
  }
  if (!args.feeAllowlist.has(res.btcFeeAddress)) {
    return { ok: false, reason: `btcFeeAddress ${res.btcFeeAddress} not in allowlist` }
  }
  if (res.receiveAmount !== String(args.expectedAmountSats)) {
    return { ok: false, reason: `receiveAmount ${res.receiveAmount} != amount ${args.expectedAmountSats} (not 1:1)` }
  }
  if (!Number.isFinite(res.btcFeeSats) || res.btcFeeSats < 0 || res.btcFeeSats > args.maxBtcFeeSats) {
    return { ok: false, reason: `btcFeeSats ${res.btcFeeSats} outside budget ${args.maxBtcFeeSats}` }
  }
  return { ok: true }
}
