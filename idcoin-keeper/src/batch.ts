import * as bitcoin from 'bitcoinjs-lib'
import { BIP32Interface } from 'bip32'
import { deriveLpVault } from './keys'

const BTC = bitcoin.networks.bitcoin
export const RBF_SEQUENCE = 0xfffffffd

export interface VaultUtxo {
  outpoint: string   // txid:vout
  txid: string
  vout: number
  sats: number
  confirmed: boolean
}

export interface BatchPlan {
  inputs: VaultUtxo[]
  totalIn: number
  estTxFee: number
  btcFeeSats: number
  amount: number      // principal to bridge vault = totalIn - estTxFee - btcFeeSats
}

export type PlanResult =
  | { kind: 'batch'; plans: BatchPlan[] }
  | { kind: 'wait'; reason: string }

function estVsize(inputs: number, outputs: number): number {
  return Math.ceil(10.5 + inputs * 68 + outputs * 31)
}

/**
 * Batch planner: given confirmed, un-batched LP vault UTXOs of one coin,
 * produce deposit plans that satisfy:
 *   - bridge amount limits [minSats, maxSats] per order (splitting larger sets)
 *   - txFee + btcFeeSats <= haircutBps/10000 * totalIn  (cost budget)
 * The deposit has NO change: amount = total - txFee - btcFeeSats.
 */
export function planBatches(args: {
  utxos: VaultUtxo[]          // confirmed only, not yet batched
  btcFeeSats: number          // from /feeQuote
  feeRateSatPerVb: number
  minSats: number
  maxSats: number
  haircutBps: number
}): PlanResult {
  const eligible = args.utxos.filter((u) => u.confirmed).sort((a, b) => b.sats - a.sats)
  if (eligible.length === 0) return { kind: 'wait', reason: 'no confirmed unbatched vault utxos' }

  // greedy largest-first accumulation; split so amount stays within maxSats
  const plans: BatchPlan[] = []
  let cur: VaultUtxo[] = []
  const budgetNum = args.haircutBps

  const tryEmit = (force: boolean): boolean => {
    if (cur.length === 0) return false
    const totalIn = cur.reduce((a, u) => a + u.sats, 0)
    const txFee = Math.ceil(args.feeRateSatPerVb * estVsize(cur.length, 2))
    const cost = txFee + args.btcFeeSats
    const amount = totalIn - cost
    if (amount < args.minSats) {
      if (force) return false // not enough yet — keep accumulating
      return false
    }
    if (cost * 10000 > budgetNum * totalIn) {
      return false // over budget — keep accumulating (more inputs dilute fixed fee)
    }
    if (amount > args.maxSats) return false // too big — caller handles split
    plans.push({ inputs: cur, totalIn, estTxFee: txFee, btcFeeSats: args.btcFeeSats, amount })
    cur = []
    return true
  }

  for (const u of eligible) {
    cur.push(u)
    const totalIn = cur.reduce((a, x) => a + x.sats, 0)
    const txFee = Math.ceil(args.feeRateSatPerVb * estVsize(cur.length, 2))
    const amount = totalIn - txFee - args.btcFeeSats
    if (amount > args.maxSats) {
      // removing the last utxo may give a valid batch; the oversized remainder
      // waits for the next accumulation round.
      const last = cur.pop()!
      tryEmit(true)
      cur = [last]
    } else {
      tryEmit(false)
    }
  }

  if (plans.length === 0) {
    const totalIn = eligible.reduce((a, u) => a + u.sats, 0)
    const txFee = Math.ceil(args.feeRateSatPerVb * estVsize(eligible.length, 2))
    const amount = totalIn - txFee - args.btcFeeSats
    if (amount < args.minSats) {
      return { kind: 'wait', reason: `amount ${amount} below bridge minimum ${args.minSats}` }
    }
    const cost = txFee + args.btcFeeSats
    if (cost * 10000 > args.haircutBps * totalIn) {
      return { kind: 'wait', reason: `cost ${cost} exceeds haircut budget ${args.haircutBps / 100}% of ${totalIn}` }
    }
    if (amount > args.maxSats) {
      return { kind: 'wait', reason: 'splitting could not bring batch under bridge maximum' }
    }
    // edge: everything together forms exactly one valid batch but the greedy
    // loop never emitted — emit it now.
    plans.push({ inputs: eligible, totalIn, estTxFee: txFee, btcFeeSats: args.btcFeeSats, amount })
  }
  return { kind: 'batch', plans }
}

export interface DepositBuild {
  txid: string
  hex: string
  totalIn: number
  fee: number
  amount: number
  btcFeeSats: number
}

/** Build+sign the deposit tx: inputs ONLY from the coin's LP vault key
 *  (single-key — the bridge verifier requires depositor input ownership),
 *  exactly two outputs: bridge vault (amount) + fee output (btcFeeSats).
 *  No change. RBF-signalling is unnecessary (bridge broadcasts) but harmless;
 *  we use final sequence since replacement is not part of the design. */
export function buildDepositTx(args: {
  vaultNode: BIP32Interface     // private node at the coin's vault path
  inputs: VaultUtxo[]
  bridgeAddress: string
  feeAddress: string
  amount: number
  btcFeeSats: number
}): DepositBuild {
  const pubkey = Buffer.from(args.vaultNode.publicKey)
  const p2wpkh = bitcoin.payments.p2wpkh({ pubkey, network: BTC })
  const totalIn = args.inputs.reduce((a, u) => a + u.sats, 0)
  const fee = totalIn - args.amount - args.btcFeeSats
  if (fee <= 0) throw new Error(`deposit fee ${fee} not positive`)

  const psbt = new bitcoin.Psbt({ network: BTC })
  for (const u of args.inputs) {
    psbt.addInput({
      hash: u.txid,
      index: u.vout,
      witnessUtxo: { script: p2wpkh.output!, value: u.sats },
    })
  }
  psbt.addOutput({ address: args.bridgeAddress, value: args.amount })
  psbt.addOutput({ address: args.feeAddress, value: args.btcFeeSats })
  const signer = {
    publicKey: pubkey,
    sign: (hash: Buffer) => args.vaultNode.sign(hash),
  }
  for (let i = 0; i < args.inputs.length; i++) {
    psbt.signInput(i, signer as any)
  }
  psbt.finalizeAllInputs()
  const tx = psbt.extractTransaction()
  return { txid: tx.getId(), hex: tx.toHex(), totalIn, fee, amount: args.amount, btcFeeSats: args.btcFeeSats }
}

/** Invariants on the decoded signed deposit tx — mirrors the bridge-side
 *  verifyBtcMintTx expectations and the spec §4.1 payout whitelist. */
export function validateDepositInvariants(args: {
  hex: string
  inputs: VaultUtxo[]
  vaultAddress: string
  bridgeAddress: string
  feeAddress: string
  amount: number
  btcFeeSats: number
}): { ok: boolean; reason?: string } {
  let tx: bitcoin.Transaction
  try {
    tx = bitcoin.Transaction.fromHex(args.hex)
  } catch {
    return { ok: false, reason: 'deposit tx does not decode' }
  }
  const expected = new Set(args.inputs.map((u) => u.outpoint))
  if (tx.ins.length !== expected.size) {
    return { ok: false, reason: `input count ${tx.ins.length} != planned ${expected.size}` }
  }
  for (const inp of tx.ins) {
    const op = `${Buffer.from(inp.hash).reverse().toString('hex')}:${inp.index}`
    if (!expected.has(op)) return { ok: false, reason: `input ${op} not a planned vault utxo` }
  }
  if (tx.outs.length !== 2) {
    return { ok: false, reason: `output count ${tx.outs.length} != 2 (bridge + fee, no change)` }
  }
  const bridgeScript = bitcoin.address.toOutputScript(args.bridgeAddress, BTC)
  const feeScript = bitcoin.address.toOutputScript(args.feeAddress, BTC)
  const totalIn = args.inputs.reduce((a, u) => a + u.sats, 0)
  let sawBridge = 0
  let sawFee = 0
  for (const out of tx.outs) {
    if (out.script.equals(bridgeScript) && Number(out.value) === args.amount) sawBridge++
    else if (out.script.equals(feeScript) && Number(out.value) === args.btcFeeSats) sawFee++
    else return { ok: false, reason: 'output is neither the exact bridge principal nor the exact fee output' }
  }
  if (sawBridge !== 1) return { ok: false, reason: 'missing exact bridge principal output' }
  if (sawFee !== 1) return { ok: false, reason: 'missing exact fee output' }
  const fee = totalIn - args.amount - args.btcFeeSats
  if (fee <= 0) return { ok: false, reason: `fee ${fee} not positive` }
  return { ok: true }
}

/** Derive the per-coin vault node (private) from the account node. */
export function vaultNodeForTick(accountNode: BIP32Interface, tick: string): BIP32Interface {
  let node = accountNode
  for (const idx of Array.from(Buffer.from(tick.toUpperCase(), 'utf8'))) {
    node = node.derive(idx)
  }
  return node
}

export { deriveLpVault }
