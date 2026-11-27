import * as bitcoin from 'bitcoinjs-lib'
import { ChainUtxo } from './chain'
import { LegacyCoinKey, deriveLpVault } from './keys'
import { BIP32Interface } from 'bip32'

const BTC = bitcoin.networks.bitcoin
export const DUST_LIMIT = 546
export const RBF_SEQUENCE = 0xfffffffd

export interface SweepBuildParams {
  tickId: string
  tick: string
  legacyKey: LegacyCoinKey
  accountNode: BIP32Interface
  utxos: ChainUtxo[]            // pre-filtered eligible UTXOs (>= minUtxo, no inscriptions)
  feeRateSatPerVb: number       // already max(fastestFee, floor)
  maxFeeRatio: number
}

export interface SweepBuildResult {
  txid: string
  hex: string
  totalIn: number
  fee: number
  outputSats: number
  vsize: number
  inputOutpoints: string[]
  skippedReason?: string
}

export interface SweepSkip {
  skipped: true
  reason: string
}

export type SweepOutcome = ({ built: true } & SweepBuildResult) | SweepSkip

/** Filter eligible UTXOs: value >= minUtxo and not inscription-bearing. */
export function eligibleUtxos(utxos: ChainUtxo[], minUtxoSats: number): ChainUtxo[] {
  return utxos.filter((u) => u.sats >= minUtxoSats && !u.inscriptionBearing)
}

/**
 * Build + sign a sweep tx: all inputs from the legacy payTo, exactly one
 * P2WPKH output to the coin's LP vault, RBF-signalling, fee = rate x vsize.
 */
export function buildSweepTx(params: SweepBuildParams): SweepOutcome {
  const { utxos } = params
  if (utxos.length === 0) return { skipped: true, reason: 'no eligible utxos' }

  const vault = deriveLpVault(params.accountNode, params.tick)
  const totalIn = utxos.reduce((acc, u) => acc + u.sats, 0)

  // Estimate vsize with a 1-output template, compute fee, then build for real.
  // P2WPKH input ~68 vB, output 31 vB, overhead ~10.5 vB.
  const estVsize = Math.ceil(10.5 + utxos.length * 68 + 31)
  const fee = Math.ceil(params.feeRateSatPerVb * estVsize)
  if (totalIn > 0 && fee / totalIn > params.maxFeeRatio) {
    return {
      skipped: true,
      reason: `fee ratio ${(fee / totalIn).toFixed(3)} exceeds cap ${params.maxFeeRatio} (fee ${fee} of ${totalIn} sats)`,
    }
  }
  const outputSats = totalIn - fee
  if (outputSats < DUST_LIMIT) {
    return { skipped: true, reason: `output ${outputSats} below dust limit ${DUST_LIMIT}` }
  }

  const kp = params.legacyKey.keyPair
  const pubkey = Buffer.from(kp.publicKey)
  const p2wpkh = bitcoin.payments.p2wpkh({ pubkey, network: BTC })

  const psbt = new bitcoin.Psbt({ network: BTC })
  for (const u of utxos) {
    psbt.addInput({
      hash: u.txid,
      index: u.vout,
      sequence: RBF_SEQUENCE,
      witnessUtxo: { script: p2wpkh.output!, value: u.sats },
    })
  }
  psbt.addOutput({ address: vault.address, value: outputSats })
  for (let i = 0; i < utxos.length; i++) {
    psbt.signInput(i, kp)
  }
  psbt.finalizeAllInputs()
  const tx = psbt.extractTransaction()
  const hex = tx.toHex()
  const txid = tx.getId()

  // ---- invariant validation on the decoded signed tx ----
  const check = validateSweepInvariants({
    hex,
    inputOutpoints: utxos.map((u) => `${u.txid}:${u.vout}`),
    vaultAddress: vault.address,
    totalIn,
    fee,
    outputSats,
  })
  if (!check.ok) {
    throw new Error(`sweep invariant violation: ${check.reason}`)
  }

  return {
    built: true,
    txid,
    hex,
    totalIn,
    fee,
    outputSats,
    vsize: tx.virtualSize(),
    inputOutpoints: utxos.map((u) => `${u.txid}:${u.vout}`),
  }
}

export function validateSweepInvariants(args: {
  hex: string
  inputOutpoints: string[]
  vaultAddress: string
  totalIn: number
  fee: number
  outputSats: number
}): { ok: boolean; reason?: string } {
  let tx: bitcoin.Transaction
  try {
    tx = bitcoin.Transaction.fromHex(args.hex)
  } catch {
    return { ok: false, reason: 'tx does not decode' }
  }
  // every input is one of the swept legacy outpoints
  const expected = new Set(args.inputOutpoints)
  if (tx.ins.length !== expected.size) {
    return { ok: false, reason: `input count ${tx.ins.length} != swept outpoints ${expected.size}` }
  }
  for (const inp of tx.ins) {
    const op = `${Buffer.from(inp.hash).reverse().toString('hex')}:${inp.index}`
    if (!expected.has(op)) {
      return { ok: false, reason: `input ${op} is not a swept legacy outpoint` }
    }
    if (inp.sequence !== RBF_SEQUENCE) {
      return { ok: false, reason: 'input not signalling RBF' }
    }
  }
  // exactly one output, paying the LP vault, amount = inputs - fee
  if (tx.outs.length !== 1) {
    return { ok: false, reason: `output count ${tx.outs.length} != 1` }
  }
  const vaultScript = bitcoin.address.toOutputScript(args.vaultAddress, BTC)
  if (!tx.outs[0].script.equals(vaultScript)) {
    return { ok: false, reason: 'single output does not pay the LP vault' }
  }
  if (Number(tx.outs[0].value) !== args.totalIn - args.fee) {
    return { ok: false, reason: `output value ${tx.outs[0].value} != inputs-fee ${args.totalIn - args.fee}` }
  }
  if (Number(tx.outs[0].value) !== args.outputSats) {
    return { ok: false, reason: 'output value mismatch vs recorded outputSats' }
  }
  return { ok: true }
}
