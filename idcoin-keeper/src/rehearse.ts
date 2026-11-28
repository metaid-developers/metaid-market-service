/**
 * Read-only rehearsal: `npx ts-node --transpile-only src/rehearse.ts [TICK] [amountSats]`
 *
 * Uses production READ APIs only. NEVER broadcasts, NEVER creates bridge
 * orders, NEVER writes to the database. Every section degrades gracefully —
 * an unreachable API prints the error and the rehearsal continues.
 *
 * Prints: legacy payTo + LP vault + UTXOs at both, bridge fee quotes, the
 * batch plan that WOULD be made (real vault UTXOs if any, plus a hypothetical
 * single-UTXO plan of the given amount), the bridge pair status, the exact
 * OrderTokenV2 floor price, a signed-not-broadcast dry-built bid validation
 * (when the pair exists and funding UTXOs allow building), and MVC LP
 * SPACE/WBTC balances.
 */
import axios from 'axios'
import { loadConfig } from './config'
import { loadLpKeys, loadLegacyKeys, deriveLpVault } from './keys'
import { HigunBtcClient, MempoolClient } from './chain'
import { BitcoinRpcClient } from './btcrpc'
import { planBatches, VaultUtxo } from './batch'
import { BridgeClient } from './bridge'
import { computeFloorPrice, floorBidParams, placeFloorBid, batchSaltHex, WBTC_CODEHASH, WBTC_TOKEN_ID } from './bid'
import { loadOrderbookContracts } from '../../orderbook-service/src/core-bridge'
import { HigunChainProvider } from '../../orderbook-service/src/chain-provider'
import { OrderbookTxBuilder } from '../../orderbook-service/src/tx-builder'
import { scrypt } from '../../orderbook-service/src/core-bridge'
import * as os from 'os'
import * as path from 'path'
import * as fs from 'fs'

const { mvc } = scrypt as any

const fast = axios.create({ timeout: 5_000 })

function section(title: string) {
  console.log(`\n=== ${title} ===`)
}

async function attempt<T>(label: string, fn: () => Promise<T>): Promise<T | null> {
  try {
    return await fn()
  } catch (e: any) {
    console.log(`  [unavailable] ${label}: ${e.message?.slice(0, 200)}`)
    return null
  }
}

async function main() {
  const tick = (process.argv[2] || 'SUNNY').toUpperCase()
  const hypothetical = Number(process.argv[3] || 200_000)
  const config = loadConfig()
  console.log(`rehearsal for ${tick} — hypothetical amount ${hypothetical} sats (READ-ONLY, nothing is broadcast or created)`)
  console.log(`config: higunBtc=${config.higunBtcUrl} higunMvc=${config.higunMvcUrl} bridge=${config.bridgeUrl}`)
  console.log(`config: depth=${config.depthUrl} coins=${config.coinsUrl}`)

  // ---- keys ----
  let lpKeys: ReturnType<typeof loadLpKeys> | null = null
  let legacyKeys: Map<string, any> = new Map()
  await attempt('lp keys', async () => {
    lpKeys = loadLpKeys(config.mnemonicFile)
    console.log('  LP keys loaded (mnemonic redacted)')
  })
  await attempt('legacy keys', async () => {
    legacyKeys = loadLegacyKeys(config.legacyKeysFile) as any
    console.log(`  legacy keys loaded: ${legacyKeys.size} entries`)
  })

  // ---- coin registry lookup ----
  section('coin lookup (coins-list API)')
  let coin: any = null
  await attempt('coins-list', async () => {
    const resp = await fast.get(config.coinsUrl)
    const items = resp.data?.data?.list || resp.data?.list || []
    coin = items.find((it: any) => String(it.tick || '').toUpperCase() === tick) || null
    if (!coin) {
      console.log(`  ${tick} NOT FOUND in coins-list (${items.length} coins)`)
      return
    }
    console.log(`  tickId=${coin.tickId} payTo=${coin.payCheck?.payTo || coin.payTo || '(none)'}`)
    console.log(`  amtPerMint=${coin.amtPerMint} liquidityPerMint=${coin.liquidityPerMint} decimals=${coin.decimals}`)
  })

  // ---- addresses ----
  section('addresses')
  let vaultAddress = ''
  if (lpKeys) {
    vaultAddress = deriveLpVault((lpKeys as any).accountNode, tick).address
    console.log(`  LP vault(${tick}): ${vaultAddress}`)
  } else {
    console.log('  LP vault: unavailable (no mnemonic file)')
  }
  let legacyAddr = ''
  if (coin) {
    const entry: any = legacyKeys.get(String(coin.tickId))
    legacyAddr = entry?.address || ''
    console.log(`  legacy pool address: ${legacyAddr || '(not in legacy keys file)'}`)
    const payTo = coin.payCheck?.payTo || coin.payTo || ''
    if (payTo) {
      console.log(`  payTo == legacy pool: ${payTo === legacyAddr} ; payTo == LP vault: ${payTo === vaultAddress}`)
    }
  }

  // ---- BTC UTXOs ----
  const higunBtc = new HigunBtcClient(config.higunBtcUrl, fast)
  const mempool = new MempoolClient(config.mempoolUrl, fast)
  let btcRpc: BitcoinRpcClient | null = null
  if (config.btcRpcCredsFile) {
    try {
      const creds = JSON.parse(fs.readFileSync(config.btcRpcCredsFile, 'utf8'))
      if (creds && typeof creds.user === 'string' && typeof creds.pass === 'string') {
        btcRpc = new BitcoinRpcClient(config.btcRpcUrl, creds.user, creds.pass)
      }
    } catch {
      btcRpc = null
    }
  }
  for (const [label, addr] of [['legacy pool', legacyAddr], ['LP vault', vaultAddress]] as const) {
    if (!addr) continue
    section(`BTC UTXOs at ${label} ${addr}`)
    const utxos = await attempt('higun btc-utxo', () => higunBtc.getUtxos(addr))
    if (utxos) {
      const total = utxos.reduce((a, u) => a + u.sats, 0)
      console.log(`  higun: ${utxos.length} utxos, total ${total} sats`)
      for (const u of utxos.slice(0, 10)) {
        console.log(`    ${u.txid}:${u.vout} ${u.sats} sats confirmed=${u.confirmed} inscription=${u.inscriptionBearing}`)
      }
    }
    if (utxos && btcRpc) {
      await attempt('bitcoin core gettxout verification', async () => {
        let verified = 0
        let dropped = 0
        for (const u of utxos) {
          const out = await btcRpc!.getTxOut(u.txid, u.vout)
          if (out === null) dropped++
          else verified++
        }
        console.log(`  core gettxout: verified=${verified} dropped=${dropped}`)
      })
    }
    const mpUtxos = await attempt('mempool.space utxo', () => mempool.getUtxos(addr))
    if (mpUtxos) {
      console.log(`  mempool.space: ${mpUtxos.length} utxos, total ${mpUtxos.reduce((a, u) => a + u.sats, 0)} sats`)
    }
  }

  // ---- bridge fee quote ----
  const bridge = new BridgeClient(config.bridgeUrl, fast)
  section('bridge fee quote')
  let quote: any = null
  await attempt('feeQuote', async () => {
    quote = await bridge.feeQuote()
    console.log(`  btcFeeSats=${quote.btcFeeSats} btcFeeAddress=${quote.btcFeeAddress}`)
    console.log(`  serviceFee=${quote.btcServiceFeeSats} minerFee=${quote.btcBridgeMinerFeeSats} postage=${quote.btcPostageSats} feeRate=${quote.btcFeeRate}`)
  })

  // ---- batch plan ----
  section('batch plan (hypothetical)')
  if (quote) {
    const feeRate = Math.max(config.feeFloorSatPerVb, await attempt('mempool fastest fee', () => mempool.getFastestFee()) || config.feeFloorSatPerVb)
    const mk = (sats: number): VaultUtxo[] => [{ outpoint: `hypothetical:0`, txid: 'hypothetical', vout: 0, sats, confirmed: true }]
    for (const [label, utxos] of [
      [`single ${hypothetical}-sat utxo`, mk(hypothetical)],
    ] as const) {
      const res = planBatches({
        utxos: utxos as VaultUtxo[],
        btcFeeSats: quote.btcFeeSats,
        feeRateSatPerVb: feeRate,
        minSats: config.bridgeMinSats,
        maxSats: config.bridgeMaxSats,
        haircutBps: config.batchHaircutBps,
      })
      if (res.kind === 'wait') {
        console.log(`  ${label}: WAIT — ${res.reason}`)
      } else {
        for (const p of res.plans) {
          console.log(`  ${label}: batch totalIn=${p.totalIn} txFee=${p.estTxFee} btcFee=${p.btcFeeSats} bridgeAmount=${p.amount} (feeRate ${feeRate} sat/vb)`)
        }
      }
    }
    // real vault utxos if any
    if (vaultAddress) {
      const real = await attempt('higun btc-utxo (vault)', () => higunBtc.getUtxos(vaultAddress))
      if (real && real.length > 0) {
        const usable = real.filter((u) => !u.inscriptionBearing).map((u) => ({ ...u, outpoint: `${u.txid}:${u.vout}` }))
        const res = planBatches({
          utxos: usable, btcFeeSats: quote.btcFeeSats, feeRateSatPerVb: feeRate,
          minSats: config.bridgeMinSats, maxSats: config.bridgeMaxSats, haircutBps: config.batchHaircutBps,
        })
        if (res.kind === 'wait') console.log(`  real vault utxos (${usable.length}): WAIT — ${res.reason}`)
        else for (const p of res.plans) console.log(`  real vault utxos: batch totalIn=${p.totalIn} bridgeAmount=${p.amount} inputs=${p.inputs.length}`)
      } else if (real) {
        console.log('  real vault: no UTXOs — nothing to batch yet')
      }
    }
  } else {
    console.log('  skipped (no fee quote)')
  }

  // ---- bridge pair status ----
  section('bridge pair status')
  let pair: any = null
  if (coin) {
    await attempt('assetList', async () => {
      const list = await bridge.assetList()
      pair = list.find((p: any) => String(p.originTokenId) === String(coin.tickId)) || null
      if (pair) {
        console.log(`  ${tick} REGISTERED: targetSymbol=${pair.targetSymbol} decimals=${pair.decimals} mintEnabled=${pair.isEnableMint}`)
        console.log(`  targetTokenCodeHash=${pair.targetTokenCodeHash}`)
        console.log(`  targetTokenGenesis=${pair.targetTokenGenesis}`)
      } else {
        console.log(`  ${tick} (tickId ${coin.tickId}) NOT registered on the bridge (${list.length} pairs total)`)
      }
    })
  } else {
    console.log('  skipped (coin not found)')
  }

  // ---- floor price ----
  section('floor price (OrderTokenV2 priceNum/priceDen)')
  if (coin && Number(coin.liquidityPerMint) > 0) {
    try {
      const price = computeFloorPrice({
        amtPerMint: BigInt(String(coin.amtPerMint)),
        decimals: Number(coin.decimals),
        liquidityPerMint: BigInt(String(coin.liquidityPerMint)),
        haircutBps: config.floorHaircutBps,
      })
      console.log(`  priceNum=${price.priceNum} priceDen=${price.priceDen} (haircut ${config.floorHaircutBps} bps)`)
      console.log(`  = ${(Number(price.priceNum) / Number(price.priceDen)).toFixed(8)} coin-units(per 1e${coin.decimals}) per sat — see spec for units`)
    } catch (e: any) {
      console.log(`  price computation failed: ${e.message}`)
    }
  } else {
    console.log('  skipped (no liquidityPerMint)')
  }

  // ---- MVC balances ----
  section('MVC LP funding balances')
  let mvcProvider: HigunChainProvider | null = null
  if (lpKeys) {
    mvcProvider = new HigunChainProvider({
      higunUrl: config.higunMvcUrl,
      metaletApiBase: 'https://www.metalet.space/wallet-api',
      net: 'livenet',
    })
    const fundingAddr = (lpKeys as any).mvcFundingAddress
    console.log(`  funding address: ${fundingAddr}`)
    const spaceUtxos = await attempt('mvc SPACE utxos', () => mvcProvider!.getMvcUtxos(fundingAddr))
    if (spaceUtxos) {
      console.log(`  SPACE: ${spaceUtxos.reduce((a: number, u: any) => a + Number(u.satoshis || u.sats || 0), 0)} sats in ${spaceUtxos.length} utxos`)
    }
    const wbtcUtxos = await attempt('mvc WBTC token utxos', () => mvcProvider!.getTokenUtxos(fundingAddr, WBTC_CODEHASH, WBTC_TOKEN_ID))
    if (wbtcUtxos) {
      console.log(`  WBTC: ${wbtcUtxos.reduce((a: bigint, u: any) => a + BigInt(u.tokenAmount), 0n)} sats in ${wbtcUtxos.length} utxos`)
    }
  } else {
    console.log('  skipped (no mnemonic file)')
  }

  // ---- dry-built bid validation ----
  section('dry-built bid validation (signed locally, NEVER broadcast)')
  if (pair && lpKeys && mvcProvider && coin) {
    await attempt('bid draft', async () => {
      const contractsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'idcoin-rehearse-contracts-'))
      const contracts = loadOrderbookContracts(contractsDir)
      const builder = new OrderbookTxBuilder({ contracts, chain: mvcProvider!, net: 'livenet' })
      const fundingAddr = (lpKeys as any).mvcFundingAddress
      const wbtcUtxos = await mvcProvider!.getTokenUtxos(fundingAddr, WBTC_CODEHASH, WBTC_TOKEN_ID)
      const wbtcBalance = wbtcUtxos.reduce((a: bigint, u: any) => a + BigInt(u.tokenAmount), 0n)
      if (wbtcBalance <= 0n) {
        console.log('  no WBTC at the funding address — a draft cannot be funded; skipping build')
        return
      }
      const sellAmount = wbtcBalance < 10_000n ? wbtcBalance : 10_000n
      const price = computeFloorPrice({
        amtPerMint: BigInt(String(coin.amtPerMint)),
        decimals: Number(coin.decimals),
        liquidityPerMint: BigInt(String(coin.liquidityPerMint)),
        haircutBps: config.floorHaircutBps,
      })
      const params = floorBidParams({
        fundingAddress: fundingAddr,
        buyTokenCodeHash: pair.targetTokenCodeHash,
        buyTokenID: pair.targetTokenGenesis,
        sellAmountWbtc: sellAmount,
        priceNum: price.priceNum,
        priceDen: price.priceDen,
        saltHex: batchSaltHex(0),
      })
      const privHex = Buffer.from((lpKeys as any).mvcFundingNode.privateKey!).toString('hex')
      const privKey = new mvc.PrivateKey(privHex, 'livenet')
      const signed = await placeFloorBid({
        builder,
        privKey,
        params,
        expectation: {
          fundingAddress: fundingAddr,
          sellTokenCodeHash: WBTC_CODEHASH,
          sellTokenID: WBTC_TOKEN_ID,
          buyTokenCodeHash: pair.targetTokenCodeHash,
          buyTokenID: pair.targetTokenGenesis,
          sellAmount,
          priceNum: price.priceNum,
          priceDen: price.priceDen,
          minFillAmount: BigInt(params.minFillAmount),
        },
      })
      console.log(`  VALIDATION PASSED — draft built, validated, signed locally; prep txid would be ${signed.prepTxid}`)
      console.log(`  sellAmount=${sellAmount} WBTC sats, order outpoint would be ${signed.orderTxid}:${signed.orderVout}`)
      console.log('  (hexes discarded; nothing broadcast, nothing persisted)')
    })
  } else {
    console.log(`  skipped (pair=${!!pair} keys=${!!lpKeys} coin=${!!coin})`)
  }

  console.log('\nrehearsal complete — no transactions broadcast, no bridge orders created')
}

main().catch((e) => {
  console.error('rehearsal fatal error:', e.message)
  process.exit(1)
})
