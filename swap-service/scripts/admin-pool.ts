#!/usr/bin/env ts-node
/**
 * Admin Pool Management CLI for MetaID.market Swap Service
 *
 * Subcommands:
 *   list     List registered pools and whitelisted SwapMain codeHashes
 *   import   Verify & import an existing on-chain MVCSwap pool (auto-derives contract fields)
 *   create   Deploy a brand-new SPACE/FT pool + inject initial liquidity on MVC mainnet
 *   remove   Delist a pool from pools.json (and hot-remove from running swap-service if --api is given)
 *
 * Safety:
 *   - `create` defaults to --dry (builds & verifies all 7 txs offline, broadcasts NOTHING).
 *     Pass --live explicitly to broadcast on MVC mainnet.
 *   - Never pass WIF in logs; prefer --wif-env SWAP_ADMIN_WIF or --key-file /path/to/key.json.
 */
import * as fs from 'fs'
import { getPoolType, loadConfig, savePoolsRegistry } from '../src/config'
import { SwapEngine, axios, loadSwapContractsFast, loadTokenSwapContractsFast } from '../src/core-bridge'
import {
  deployNewPoolOnChain,
  deployTokenSwapPoolOnChain,
  inspectOnChainPoolAndBuildEntry,
} from '../src/pool-admin'

function parseArgs(argv: string[]) {
  const command = argv[0] || 'help'
  const flags: Record<string, string | boolean> = {}
  const positional: string[] = []

  for (let i = 1; i < argv.length; i++) {
    const arg = argv[i]
    if (arg.startsWith('--')) {
      const eqIdx = arg.indexOf('=')
      if (eqIdx > 2) {
        flags[arg.slice(2, eqIdx)] = arg.slice(eqIdx + 1)
      } else {
        const key = arg.slice(2)
        const next = argv[i + 1]
        if (next && !next.startsWith('--')) {
          flags[key] = next
          i++
        } else {
          flags[key] = true
        }
      }
    } else {
      positional.push(arg)
    }
  }
  return { command, flags, positional }
}

function strFlag(flags: Record<string, string | boolean>, key: string): string | undefined {
  const v = flags[key]
  return typeof v === 'string' ? v : undefined
}

function boolFlag(flags: Record<string, string | boolean>, key: string): boolean {
  return flags[key] === true || flags[key] === 'true' || flags[key] === '1'
}

function loadAdminWif(flags: Record<string, string | boolean>): string {
  const envName = strFlag(flags, 'wif-env') || 'SWAP_ADMIN_WIF'
  if (process.env[envName]) {
    return process.env[envName]!.trim()
  }
  const keyFile = strFlag(flags, 'key-file')
  if (keyFile) {
    const raw = JSON.parse(fs.readFileSync(keyFile, 'utf8'))
    const wif = raw.wif || raw.privateKey || raw.privKey
    if (!wif || typeof wif !== 'string') {
      throw new Error(`Key file ${keyFile} does not contain a string "wif" field`)
    }
    return wif.trim()
  }
  const directWif = strFlag(flags, 'wif')
  if (directWif) return directWif.trim()
  throw new Error(
    `Missing admin private key. Provide via env ${envName}, --key-file <json>, or --wif <WIF>`
  )
}

function printHelp() {
  console.log(`
MetaID.market Swap 管理员池管理工具 (admin-pool.ts)

用法:
  npx ts-node --transpile-only scripts/admin-pool.ts <command> [options]

子命令:
  1) list [--api <url> --admin-token <token>]
     查看当前已上线的交易对与白名单 SwapMain codeHash。

  2) import --pool-txid <txid> --lp-genesis-txid <txid> [options]
     从链上自动解析并校验已有 MVCSwap 白名单池子，支持写入 pools.json 或热加载到运行中服务。
     选项:
       --lp-genesis-vout <n>        LP Genesis 输出索引（默认 2）
       --pool-id <id>               自定义池子 ID（默认按代币符号生成，如 space-msp）
       --pair <name>                自定义交易对展示名（如 "SPACE / MSP"）
       --symbol / --name / --decimals / --lp-symbol   覆盖链上元数据
       --contract-hash-array <hex>  仅自定义合约池需要（标准池自动推导+密码学校验）
       --overwrite                  覆盖已有同 poolId 配置
       --dry                        仅校验并预览，不写入
       --api <http://127.0.0.1:18081>  热推送到运行中的 swap-service（无需重启）
       --admin-token <token>        配合 --api 使用（或环境变量 SWAP_ADMIN_TOKEN）

  3) create [SPACE/FT 或 Token/Token 建池]
     a) SPACE/FT 池（默认 --type space-token）:
        create --token-id <40hex> --token-codehash <40hex> --init-space <sats> --init-token <raw> [options]
     b) Token/Token 直连池（--type token-token）:
        create --type token-token --token1-id <40hex> --token1-codehash <40hex> --token2-id <40hex> --token2-codehash <40hex> --init-token1 <raw> --init-token2 <raw> [options]
     从 0 部署全新 AMM 交易对并注入首笔流动性，默认处于 --dry 演练模式（离线构造并完整验证所有合约解锁脚本，绝不广播）。
     选项:
       --type <space-token|token-token>  池子类型（默认 space-token）
       --live                       确认真链广播并注册上线（省略则为安全 --dry 模式）
       --wif-env <ENV_NAME>         从环境变量读取管理员 WIF（默认 SWAP_ADMIN_WIF）
       --key-file <path.json>       从本地 JSON 文件读取 {"wif": "..."}
       --pool-id <id>               池子 ID（如 space-msp 或 tmva-tmvb）
       --pair <name>                交易对展示名（如 "SPACE / MSP" 或 "TMVA / TMVB"）
       --token1-symbol / --token1-name / --symbol / --name / --lp-symbol
       --fee-rate <bps10>           LP 手续费（默认 25 = 0.25%）
       --proj-fee-rate <bps10>      项目方抽成（默认 5 = 0.05%）
       --min-proj-fee <raw>         项目方最小抽成阈值（默认 1000）
       --proj-fee-address <addr>    项目方收款地址（默认管理员地址）
       --overwrite                  覆盖已有同 poolId 配置
       --api <http://127.0.0.1:18081>  上链后热推送到运行中 swap-service（无需重启）
       --admin-token <token>        配合 --api 使用（或环境变量 SWAP_ADMIN_TOKEN）

  4) remove --pool-id <id> [--api <url> --admin-token <token>]
     下架指定交易对。
`)
}

async function main() {
  const { command, flags } = parseArgs(process.argv.slice(2))
  if (command === 'help' || command === '--help' || command === '-h') {
    printHelp()
    return
  }

  const config = loadConfig()
  const apiBase = strFlag(flags, 'api')?.replace(/\/$/, '')
  const adminToken = strFlag(flags, 'admin-token') || process.env.SWAP_ADMIN_TOKEN || ''

  const apiHeaders = () => ({
    'Content-Type': 'application/json',
    'X-Swap-Admin-Token': adminToken,
  })

  if (command === 'list') {
    if (apiBase) {
      const r = await axios.get(`${apiBase}/internal/swap/admin/pools`, { headers: apiHeaders(), timeout: 15000 })
      console.log(JSON.stringify(r.data, null, 2))
      return
    }
    console.log(
      JSON.stringify(
        {
          poolsFile: config.poolsFile,
          whitelistSwapCodeHashes: config.whitelistSwapCodeHashes,
          pools: config.pools.map((p) => ({
            poolId: p.poolId,
            pairName: p.pairName,
            poolType: getPoolType(p),
            token1: p.token1.tokenID ? `${p.token1.symbol} (${p.token1.tokenID})` : p.token1.symbol,
            token2: `${p.token2.symbol} (${p.token2.tokenID})`,
            swapContractID: p.contract.swapContractID,
            bootstrapHead: p.bootstrapHead,
          })),
        },
        null,
        2
      )
    )
    return
  }

  if (command === 'import') {
    const poolTxid = strFlag(flags, 'pool-txid')
    const lpGenesisTxid = strFlag(flags, 'lp-genesis-txid')
    if (!poolTxid || !lpGenesisTxid) {
      throw new Error('import requires --pool-txid <txid> and --lp-genesis-txid <txid>')
    }
    const lpGenesisVout = strFlag(flags, 'lp-genesis-vout')
    const token1Decimals = strFlag(flags, 'token1-decimals')
    const tokenDecimals = strFlag(flags, 'decimals')
    const dryRun = boolFlag(flags, 'dry')
    const overwrite = boolFlag(flags, 'overwrite')

    const input = {
      poolId: strFlag(flags, 'pool-id'),
      pairName: strFlag(flags, 'pair'),
      poolTxid,
      lpGenesisTxid,
      lpGenesisVout: lpGenesisVout !== undefined ? Number(lpGenesisVout) : undefined,
      token1Symbol: strFlag(flags, 'token1-symbol'),
      token1Name: strFlag(flags, 'token1-name'),
      token1Decimals: token1Decimals !== undefined ? Number(token1Decimals) : undefined,
      tokenSymbol: strFlag(flags, 'symbol'),
      tokenName: strFlag(flags, 'name'),
      tokenDecimals: tokenDecimals !== undefined ? Number(tokenDecimals) : undefined,
      lpSymbol: strFlag(flags, 'lp-symbol'),
      contractHashArrayHex: strFlag(flags, 'contract-hash-array'),
    }

    if (apiBase) {
      const r = await axios.post(
        `${apiBase}/internal/swap/admin/pools/import`,
        { ...input, dryRun, overwrite },
        { headers: apiHeaders(), timeout: 30000 }
      )
      console.log(JSON.stringify(r.data, null, 2))
      return
    }

    const chain = new SwapEngine.MetaletChainClient({
      apiBase: config.metaletApiBase,
      net: config.net,
      higunBase: config.higunUrl,
    })
    const contracts = loadSwapContractsFast()
    const inspected = await inspectOnChainPoolAndBuildEntry({
      chain,
      contracts,
      whitelistSwapCodeHashes: config.whitelistSwapCodeHashes,
      input,
    })

    if (dryRun) {
      console.log('[DRY-RUN] Verified on-chain pool (not written to pools.json):')
      console.log(JSON.stringify(inspected, null, 2))
      return
    }

    const pools = [...config.pools]
    const existingIdx = pools.findIndex((p) => p.poolId === inspected.entry.poolId)
    if (existingIdx >= 0 && !overwrite) {
      throw new Error(`Pool "${inspected.entry.poolId}" already exists in ${config.poolsFile} (pass --overwrite to replace)`)
    }
    if (existingIdx >= 0) pools[existingIdx] = inspected.entry
    else pools.push(inspected.entry)

    savePoolsRegistry(config.poolsFile, config.whitelistSwapCodeHashes, pools)
    console.log(`[OK] Imported pool "${inspected.entry.poolId}" (${inspected.entry.pairName}) into ${config.poolsFile}`)
    console.log(JSON.stringify(inspected, null, 2))
    return
  }

  if (command === 'create') {
    const poolType = strFlag(flags, 'type') || 'space-token'
    if (poolType !== 'space-token' && poolType !== 'token-token') {
      throw new Error(`Invalid --type "${poolType}". Expected "space-token" or "token-token".`)
    }
    const adminWif = loadAdminWif(flags)
    const live = boolFlag(flags, 'live') && !boolFlag(flags, 'dry')
    const overwrite = boolFlag(flags, 'overwrite')

    const chain = new SwapEngine.MetaletChainClient({
      apiBase: config.metaletApiBase,
      net: config.net,
      higunBase: config.higunUrl,
    })

    let result: any
    if (poolType === 'token-token') {
      const token1ID = strFlag(flags, 'token1-id')
      const token1CodeHash = strFlag(flags, 'token1-codehash')
      const token2ID = strFlag(flags, 'token2-id') || strFlag(flags, 'token-id')
      const token2CodeHash = strFlag(flags, 'token2-codehash') || strFlag(flags, 'token-codehash')
      const initToken1Str = strFlag(flags, 'init-token1')
      const initToken2Str = strFlag(flags, 'init-token2') || strFlag(flags, 'init-token')
      if (!token1ID || !token1CodeHash || !token2ID || !token2CodeHash || !initToken1Str || !initToken2Str) {
        throw new Error(
          'create --type token-token requires --token1-id, --token1-codehash, --token2-id, --token2-codehash, --init-token1, and --init-token2'
        )
      }
      const tokenSwapContracts = loadTokenSwapContractsFast()
      result = await deployTokenSwapPoolOnChain({
        chain,
        tokenSwapContracts,
        whitelistSwapCodeHashes: config.whitelistSwapCodeHashes,
        adminWif,
        input: {
          token1ID,
          token1CodeHash,
          token2ID,
          token2CodeHash,
          initToken1Amount: BigInt(initToken1Str),
          initToken2Amount: BigInt(initToken2Str),
          poolId: strFlag(flags, 'pool-id'),
          pairName: strFlag(flags, 'pair'),
          token1Symbol: strFlag(flags, 'token1-symbol'),
          token1Name: strFlag(flags, 'token1-name'),
          token2Symbol: strFlag(flags, 'token2-symbol') || strFlag(flags, 'symbol'),
          token2Name: strFlag(flags, 'token2-name') || strFlag(flags, 'name'),
          lpSymbol: strFlag(flags, 'lp-symbol'),
          feeRate: strFlag(flags, 'fee-rate') ? Number(strFlag(flags, 'fee-rate')) : undefined,
          projFeeRate: strFlag(flags, 'proj-fee-rate') ? Number(strFlag(flags, 'proj-fee-rate')) : undefined,
          minProjFee: strFlag(flags, 'min-proj-fee') ? BigInt(strFlag(flags, 'min-proj-fee')!) : undefined,
          projFeeAddress: strFlag(flags, 'proj-fee-address'),
          dryRun: !live,
        },
        onProgress: (m) => console.log(`[create-token-pool] ${m}`),
      })
    } else {
      const tokenID = strFlag(flags, 'token-id')
      const tokenCodeHash = strFlag(flags, 'token-codehash')
      const initSpaceStr = strFlag(flags, 'init-space')
      const initTokenStr = strFlag(flags, 'init-token')
      if (!tokenID || !tokenCodeHash || !initSpaceStr || !initTokenStr) {
        throw new Error('create requires --token-id, --token-codehash, --init-space, and --init-token')
      }
      const contracts = loadSwapContractsFast()
      result = await deployNewPoolOnChain({
        chain,
        contracts,
        whitelistSwapCodeHashes: config.whitelistSwapCodeHashes,
        adminWif,
        tokenID,
        tokenCodeHash,
        initSpaceSats: BigInt(initSpaceStr),
        initTokenAmount: BigInt(initTokenStr),
        poolId: strFlag(flags, 'pool-id'),
        pairName: strFlag(flags, 'pair'),
        tokenSymbol: strFlag(flags, 'symbol'),
        tokenName: strFlag(flags, 'name'),
        lpSymbol: strFlag(flags, 'lp-symbol'),
        feeRate: strFlag(flags, 'fee-rate') ? Number(strFlag(flags, 'fee-rate')) : undefined,
        projFeeRate: strFlag(flags, 'proj-fee-rate') ? Number(strFlag(flags, 'proj-fee-rate')) : undefined,
        minProjFee: strFlag(flags, 'min-proj-fee') ? BigInt(strFlag(flags, 'min-proj-fee')!) : undefined,
        projFeeAddress: strFlag(flags, 'proj-fee-address'),
        dryRun: !live,
        onProgress: (m) => console.log(`[create-pool] ${m}`),
      })
    }

    if (!live) {
      console.log(
        JSON.stringify(
          {
            dryRun: true,
            adminAddress: result.adminAddress,
            txids: result.txids,
            fees: result.fees,
            entryPreview: result.entry,
          },
          null,
          2
        )
      )
      return
    }

    if (apiBase) {
      const r = await axios.post(
        `${apiBase}/internal/swap/admin/pools/register`,
        {
          entry: result.entry,
          overwrite,
          rawTxsToSeed: result.rawTxs.map((t) => ({ txid: t.txid, rawHex: t.rawHex })),
        },
        { headers: apiHeaders(), timeout: 30000 }
      )
      console.log('[OK] New pool broadcasted and hot-activated on swap-service:')
      console.log(JSON.stringify({ txids: result.txids, fees: result.fees, response: r.data }, null, 2))
      return
    }

    const pools = [...config.pools]
    const existingIdx = pools.findIndex((p) => p.poolId === result.entry.poolId)
    if (existingIdx >= 0 && !overwrite) {
      console.warn(`[WARN] On-chain deploy succeeded, but poolId "${result.entry.poolId}" already exists in pools.json`)
    } else {
      if (existingIdx >= 0) pools[existingIdx] = result.entry
      else pools.push(result.entry)
      savePoolsRegistry(config.poolsFile, config.whitelistSwapCodeHashes, pools)
      console.log(`[OK] New pool "${result.entry.poolId}" written to ${config.poolsFile}`)
    }
    console.log(JSON.stringify({ txids: result.txids, fees: result.fees, entry: result.entry }, null, 2))
    return
  }

  if (command === 'remove') {
    const poolId = strFlag(flags, 'pool-id')
    if (!poolId) throw new Error('remove requires --pool-id <id>')
    if (apiBase) {
      const r = await axios.delete(`${apiBase}/internal/swap/admin/pools/${encodeURIComponent(poolId)}`, {
        headers: apiHeaders(),
        timeout: 15000,
      })
      console.log(JSON.stringify(r.data, null, 2))
      return
    }
    const pools = [...config.pools]
    const idx = pools.findIndex((p) => p.poolId === poolId)
    if (idx < 0) throw new Error(`Pool "${poolId}" not found in ${config.poolsFile}`)
    const [removed] = pools.splice(idx, 1)
    savePoolsRegistry(config.poolsFile, config.whitelistSwapCodeHashes, pools)
    console.log(`[OK] Removed pool "${poolId}" (${removed.pairName}) from ${config.poolsFile}`)
    return
  }

  throw new Error(`Unknown command "${command}". Run with --help for usage.`)
}

main().catch((err: any) => {
  const detail = err?.response?.data ? JSON.stringify(err.response.data) : err?.message || String(err)
  console.error(`[admin-pool ERROR] ${detail}`)
  process.exit(1)
})
