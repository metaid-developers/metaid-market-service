/**
 * Production wiring for the P2 pipeline: orderbook builder over the real MVC
 * chain, bridge assetList pair resolution, depth confirmation, /status data.
 */
import axios from 'axios'
import * as os from 'os'
import * as path from 'path'
import * as fs from 'fs'
import { loadOrderbookContracts, scrypt } from '../../orderbook-service/src/core-bridge'
import { HigunChainProvider } from '../../orderbook-service/src/chain-provider'
import { OrderbookTxBuilder } from '../../orderbook-service/src/tx-builder'
import { computeContractFingerprints, WBTC_CODEHASH, WBTC_TOKEN_ID } from './bid'
import { BridgeClient, AssetPairInfo } from './bridge'
import { Pipeline } from './pipeline'
import { KeeperConfig } from './config'
import { KeeperDb } from './db'
import { LpKeys } from './keys'
import { ChainClient } from './chain'
import { Logger } from './log'

const { mvc } = scrypt as any

export interface ProductionPipeline {
  pipeline: Pipeline
  refreshPairs: () => Promise<number>
  contractFingerprints: Buffer[]
}

export function createProductionPipeline(args: {
  config: KeeperConfig
  db: KeeperDb
  log: Logger
  chain: ChainClient
  lpKeys: LpKeys
  higunMvcUrl?: string
  metaletApiBase?: string
}): ProductionPipeline {
  const contractsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'idcoin-keeper-contracts-'))
  const contracts = loadOrderbookContracts(contractsDir)
  const fingerprints = computeContractFingerprints(contracts)

  const mvcProvider = new HigunChainProvider({
    higunUrl: args.higunMvcUrl || '',
    metaletApiBase: args.metaletApiBase || 'https://www.metalet.space/wallet-api',
    net: 'livenet',
  })
  const obBuilder = new OrderbookTxBuilder({ contracts, chain: mvcProvider, net: 'livenet' })

  // MVC LP funding private key in mvc-lib form (from the bip32 node)
  const privHex = Buffer.from(args.lpKeys.mvcFundingNode.privateKey!).toString('hex')
  const mvcFundingPrivKey = new mvc.PrivateKey(privHex, 'livenet')

  const bridge = new BridgeClient(args.config.bridgeUrl)
  const pairsRef: { list: AssetPairInfo[] } = { list: [] }

  // live-order lookup: Higun orderbook contract-family index (source of truth)
  const higunBase = (args.higunMvcUrl || '').replace(/\/+$/, '')
  const orderLookup = async (codehash: string) => {
    const out: Array<{ txid: string; vout: number; scriptHex: string }> = []
    let cursor = ''
    for (let page = 0; page < 20; page++) {
      const resp = await axios.get(`${higunBase}/orderbook/utxos`, {
        params: { codehash, spent: 'false', includeMempool: '1', limit: 500, ...(cursor ? { cursor } : {}) },
        timeout: 15_000,
      })
      const items = resp.data?.items || []
      for (const it of items) {
        out.push({ txid: String(it.txid), vout: Number(it.vout), scriptHex: String(it.scriptHex || '') })
      }
      cursor = resp.data?.nextCursor || ''
      if (!cursor || items.length === 0) break
    }
    return out
  }
  const wbtcBalance = async (): Promise<bigint> => {
    const utxos = await mvcProvider.getTokenUtxos(args.lpKeys.mvcFundingAddress, WBTC_CODEHASH, WBTC_TOKEN_ID)
    return utxos.reduce((a: bigint, u: any) => a + BigInt(u.tokenAmount), 0n)
  }

  // hash160 -> base58check livenet address (MVC uses the BTC 0x00 version byte);
  // lets us query the FT index for tokens locked to a contract hash
  const hash160ToAddress = (hash: Buffer): string => {
    const crypto = require('crypto')
    const sha256 = (b: Buffer) => crypto.createHash('sha256').update(b).digest()
    const payload = Buffer.concat([Buffer.from([0x00]), hash])
    const checksum = sha256(sha256(payload)).subarray(0, 4)
    const full = Buffer.concat([payload, checksum])
    const ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz'
    let num = BigInt('0x' + full.toString('hex'))
    let out = ''
    while (num > 0n) {
      out = ALPHABET[Number(num % 58n)] + out
      num /= 58n
    }
    for (const b of full) {
      if (b === 0) out = '1' + out
      else break
    }
    return out
  }

  const pipeline = new Pipeline({
    config: args.config,
    db: args.db,
    log: args.log,
    bridge,
    chain: args.chain,
    lpKeys: args.lpKeys,
    obBuilder,
    mvcFundingPrivKey,
    mvcChain: mvcProvider,
    get pairs() {
      return pairsRef.list
    },
    contractFingerprints: fingerprints,
    orderLookup: higunBase ? orderLookup : undefined,
    wbtcBalance,
    tokenOrderCodeHash: contracts.tokenOrderCodeHashHex,
    mvcBroadcast: (hex: string) => mvcProvider.broadcastTx(hex),
    mvcTxExists: async (txid: string) => {
      try {
        await mvcProvider.getRawTx(txid)
        return true
      } catch {
        return false
      }
    },
    verifyLockedTokens: async (orderScriptHex: string, remainAmount: bigint) => {
      const scriptHash = Buffer.from(mvc.crypto.Hash.sha256ripemd160(Buffer.from(orderScriptHex, 'hex')))
      const lockAddr = hash160ToAddress(scriptHash)
      const utxos = await mvcProvider.getTokenUtxos(lockAddr, WBTC_CODEHASH, WBTC_TOKEN_ID)
      return utxos.some((u: any) => BigInt(u.tokenAmount) === remainAmount)
    },
    depthCheck: async (pairKey: string) => {
      const resp = await axios.get(args.config.depthUrl, { params: { pair: pairKey }, timeout: 10_000 })
      const bids = resp.data?.data?.bids || resp.data?.bids || []
      return { bidSeen: Array.isArray(bids) && bids.length > 0 }
    },
  } as any)

  const refreshPairs = async () => {
    pairsRef.list = await bridge.assetList()
    return pairsRef.list.length
  }

  return { pipeline, refreshPairs, contractFingerprints: fingerprints }
}
