import * as fs from 'fs'
import * as bip39 from 'bip39'
import { BIP32Factory, BIP32Interface } from 'bip32'
import * as ecc from '@bitcoin-js/tiny-secp256k1-asmjs'
import * as bitcoin from 'bitcoinjs-lib'
import { ECPairFactory } from 'ecpair'
import { registerMnemonicForRedaction } from './log'

const bip32 = BIP32Factory(ecc)
const ECPair = ECPairFactory(ecc)

const BTC = bitcoin.networks.bitcoin

export function tickToChildIndexes(tick: string): number[] {
  return Array.from(Buffer.from(tick.toUpperCase(), 'utf8'))
}

export const LP_VAULT_ACCOUNT_PATH = "m/84'/0'/1000'"
export const MVC_LP_FUNDING_PATH = "m/44'/10001'/1000'/0/0"

export interface LpKeys {
  accountNode: BIP32Interface          // private node at m/84'/0'/1000'
  mvcFundingNode: BIP32Interface
  mvcFundingAddress: string
}

export function loadLpKeys(mnemonicFile: string): LpKeys {
  const mnemonic = fs.readFileSync(mnemonicFile, 'utf8').trim()
  if (!bip39.validateMnemonic(mnemonic)) {
    throw new Error('invalid BIP39 mnemonic in IDCOIN_LP_MNEMONIC_FILE')
  }
  registerMnemonicForRedaction(mnemonic)
  const seed = bip39.mnemonicToSeedSync(mnemonic)
  const root = bip32.fromSeed(seed, BTC)
  const accountNode = root.derivePath(LP_VAULT_ACCOUNT_PATH)
  const mvcFundingNode = root.derivePath(MVC_LP_FUNDING_PATH)
  const mvcFundingAddress = p2pkhAddress(mvcFundingNode.publicKey)
  return { accountNode, mvcFundingNode, mvcFundingAddress }
}

/** Derive a coin's LP vault P2WPKH address. Works with either a private or
 *  public (neutered) node at m/84'/0'/1000' — child indexes are non-hardened. */
export function deriveLpVault(accountNode: BIP32Interface, tick: string): { address: string; pubkey: Buffer } {
  let node = accountNode
  for (const idx of tickToChildIndexes(tick)) {
    node = node.derive(idx)
  }
  const pubkey = Buffer.from(node.publicKey)
  return { address: p2wpkhAddress(pubkey), pubkey }
}

export function deriveLpVaultFromXpub(xpub: string, tick: string): string {
  const node = bip32.fromBase58(xpub, BTC)
  return deriveLpVault(node, tick).address
}

export function p2wpkhAddress(pubkey: Buffer): string {
  return bitcoin.payments.p2wpkh({ pubkey, network: BTC }).address!
}

/** MVC mainnet shares the 0x00 P2PKH version byte with BTC mainnet. */
export function p2pkhAddress(pubkey: Buffer): string {
  return bitcoin.payments.p2pkh({ pubkey, network: BTC }).address!
}

// ---------------------------------------------------------------------------
// Legacy pool keys file
// ---------------------------------------------------------------------------

export interface LegacyCoinKey {
  tick: string
  address: string
  source: string
  // WIF retained in memory only; never serialised
  wif: string
  keyPair: any
}

export interface LegacyKeysFile {
  version: number | string
  generatedAt?: string
  note?: string
  coins: Record<string, { tick: string; address: string; source: string; wif: string }>
}

export function wifToP2wpkhAddress(wif: string): string {
  const kp = ECPair.fromWIF(wif, BTC)
  return p2wpkhAddress(Buffer.from(kp.publicKey))
}

export function loadLegacyKeys(file: string): Map<string, LegacyCoinKey> {
  const raw = fs.readFileSync(file, 'utf8')
  let parsed: LegacyKeysFile
  try {
    parsed = JSON.parse(raw)
  } catch {
    throw new Error('legacy keys file is not valid JSON')
  }
  if (!parsed || typeof parsed !== 'object' || !parsed.coins || typeof parsed.coins !== 'object') {
    throw new Error('legacy keys file missing coins map')
  }
  const out = new Map<string, LegacyCoinKey>()
  for (const [tickId, entry] of Object.entries(parsed.coins)) {
    if (!entry || typeof entry.wif !== 'string' || typeof entry.address !== 'string') {
      throw new Error(`legacy keys file entry ${tickId} missing wif/address`)
    }
    let kp: any
    try {
      kp = ECPair.fromWIF(entry.wif, BTC)
    } catch {
      throw new Error(`legacy keys file entry ${tickId} (${entry.tick}): invalid WIF`)
    }
    const derived = p2wpkhAddress(Buffer.from(kp.publicKey))
    if (derived !== entry.address) {
      throw new Error(
        `legacy keys file entry ${tickId} (${entry.tick}): WIF does not match address ` +
          `(derived ${derived}, file says ${entry.address})`
      )
    }
    out.set(tickId, { tick: entry.tick, address: entry.address, source: entry.source || '', wif: entry.wif, keyPair: kp })
  }
  return out
}
