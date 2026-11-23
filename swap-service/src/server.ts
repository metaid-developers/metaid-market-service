import * as http from 'http'
import * as crypto from 'crypto'
import { URL } from 'url'
import { loadConfig } from './config'
import { PoolStore } from './pool-store'
import { SwapSessionManager } from './session-manager'
import { inspectOnChainPoolAndBuildEntry } from './pool-admin'
import {
  quoteAddLiquidity,
  quoteRemoveLiquidity,
  quoteSwapMvcToToken,
  quoteSwapTokenToMvc,
} from './quote'
import { TokenSwapProto } from './core-bridge'

function verifyAdminAuth(req: http.IncomingMessage, expectedToken?: string): { ok: boolean; status: number; message: string } {
  if (!expectedToken) {
    return {
      ok: false,
      status: 503,
      message: 'Admin endpoints are disabled (set SWAP_ADMIN_TOKEN env var to enable)',
    }
  }
  const headerToken =
    typeof req.headers['x-swap-admin-token'] === 'string'
      ? req.headers['x-swap-admin-token'].trim()
      : ''
  const authHeader = typeof req.headers['authorization'] === 'string' ? req.headers['authorization'].trim() : ''
  const bearerToken = authHeader.toLowerCase().startsWith('bearer ') ? authHeader.slice(7).trim() : ''
  const provided = headerToken || bearerToken
  if (!provided) {
    return { ok: false, status: 401, message: 'Missing admin token (use X-Swap-Admin-Token or Authorization: Bearer)' }
  }
  const a = Buffer.from(provided, 'utf8')
  const b = Buffer.from(expectedToken, 'utf8')
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
    return { ok: false, status: 403, message: 'Invalid admin token' }
  }
  return { ok: true, status: 200, message: 'ok' }
}

function readBody(req: http.IncomingMessage): Promise<any> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = []
    req.on('data', (c) => chunks.push(Buffer.from(c)))
    req.on('end', () => {
      if (chunks.length === 0) return resolve({})
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')))
      } catch (e) {
        reject(new Error('Invalid JSON request body'))
      }
    })
    req.on('error', reject)
  })
}

function sendJson(res: http.ServerResponse, status: number, payload: any) {
  const body = JSON.stringify(payload)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Swap-Admin-Token',
  })
  res.end(body)
}

export function createSwapServer() {
  const config = loadConfig()
  const store = new PoolStore(config)
  const sessions = new SwapSessionManager(store)

  const server = http.createServer(async (req, res) => {
    if (req.method === 'OPTIONS') {
      res.writeHead(204, {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Swap-Admin-Token',
      })
      return res.end()
    }

    try {
      const url = new URL(req.url || '/', `http://${req.headers.host || '127.0.0.1'}`)
      const p = url.pathname

      // Health
      if (req.method === 'GET' && p === '/internal/health') {
        return sendJson(res, 200, {
          code: 0,
          message: 'ok',
          data: {
            net: config.net,
            poolCount: config.pools.length,
            higunEnabled: Boolean(config.higunUrl),
          },
        })
      }

      // GET /internal/swap/pools
      if (req.method === 'GET' && p === '/internal/swap/pools') {
        const force = url.searchParams.get('refresh') === '1'
        const pools = await store.listPools(force)
        return sendJson(res, 200, { code: 0, message: 'ok', data: { pools } })
      }

      // GET /internal/swap/pools/:poolId
      const poolMatch = p.match(/^\/internal\/swap\/pools\/([^/]+)$/)
      if (req.method === 'GET' && poolMatch) {
        const poolId = decodeURIComponent(poolMatch[1])
        const entry = store.getPoolEntry(poolId)
        const state = await store.getPoolState(poolId, url.searchParams.get('refresh') === '1')
        return sendJson(res, 200, {
          code: 0,
          message: 'ok',
          data: {
            pool: store.formatPoolView(entry, state),
            queueDepth: store.poolQueue.getQueueDepth(poolId),
          },
        })
      }

      // POST /internal/swap/quote
      if (req.method === 'POST' && p === '/internal/swap/quote') {
        const body = await readBody(req)
        const poolId = String(body.poolId || '')
        const operation = String(body.operation || '')
        const state = await store.getPoolState(poolId)

        let quote: any
        if (operation === 'swapMvcToToken') {
          quote = quoteSwapMvcToToken(state, BigInt(body.amountIn), Number(body.slippageBps ?? 100))
        } else if (operation === 'swapTokenToMvc') {
          quote = quoteSwapTokenToMvc(state, BigInt(body.amountIn), Number(body.slippageBps ?? 100))
        } else if (operation === 'addLiquidity') {
          quote = quoteAddLiquidity(
            state,
            BigInt(body.mvcAddAmount),
            body.tokenAddAmount ? BigInt(body.tokenAddAmount) : undefined
          )
        } else if (operation === 'removeLiquidity') {
          quote = quoteRemoveLiquidity(state, BigInt(body.lpBurnAmount))
        } else {
          return sendJson(res, 400, { code: 400, message: `Unsupported quote operation: ${operation}` })
        }
        return sendJson(res, 200, { code: 0, message: 'ok', data: quote })
      }

      // GET /internal/swap/positions?address=...
      if (req.method === 'GET' && p === '/internal/swap/positions') {
        const address = url.searchParams.get('address') || ''
        if (!address) return sendJson(res, 400, { code: 400, message: 'address query param is required' })

        const spaceUtxos = await store.getFreshSpaceUtxos(address)
        const spaceBalance = spaceUtxos.reduce((s, u) => s + BigInt(u.satoshis), BigInt(0))

        const positions: any[] = []
        const tokenBalances: Record<string, { balance: string; utxoCount: number; largestUtxo: string }> = {}

        for (const entry of config.pools) {
          const state = await store.getPoolState(entry.poolId)
          const tokUtxos = await store.getFreshTokenUtxos(address, entry.token2.codeHash, entry.token2.tokenID)
          const tokTotal = tokUtxos.reduce((s: bigint, u: any) => s + u.amount, BigInt(0))
          tokenBalances[entry.token2.tokenID] = {
            balance: tokTotal.toString(),
            utxoCount: tokUtxos.length,
            largestUtxo: tokUtxos.length > 0 ? tokUtxos[0].amount.toString() : '0',
          }

          const lpUtxos = await store.getFreshTokenUtxos(address, entry.lpToken.codeHash, entry.lpToken.tokenID)
          const lpTotal = lpUtxos.reduce((s: bigint, u: any) => s + u.amount, BigInt(0))
          let underlyingMvc = BigInt(0)
          let underlyingToken = BigInt(0)
          let shareBps = 0
          if (lpTotal > BigInt(0) && state.lp > BigInt(0)) {
            const [m, t] = TokenSwapProto.countLpRemoveAmount(lpTotal, state.r1, state.r2, state.lp)
            underlyingMvc = m
            underlyingToken = t
            shareBps = Number((lpTotal * BigInt(10000)) / state.lp)
          }

          positions.push({
            poolId: entry.poolId,
            pairName: entry.pairName,
            lpTokenID: entry.lpToken.tokenID,
            lpBalance: lpTotal.toString(),
            lpUtxoCount: lpUtxos.length,
            largestLpUtxo: lpUtxos.length > 0 ? lpUtxos[0].amount.toString() : '0',
            shareBps,
            underlyingMvc: underlyingMvc.toString(),
            underlyingToken: underlyingToken.toString(),
            poolReserves: {
              r1: state.r1.toString(),
              r2: state.r2.toString(),
              lp: state.lp.toString(),
            },
          })
        }

        return sendJson(res, 200, {
          code: 0,
          message: 'ok',
          data: {
            address,
            spaceBalance: spaceBalance.toString(),
            spaceUtxoCount: spaceUtxos.length,
            tokenBalances,
            positions,
          },
        })
      }

      // POST /internal/swap/session/start
      if (req.method === 'POST' && p === '/internal/swap/session/start') {
        const body = await readBody(req)
        const resp = await sessions.startSession(body)
        return sendJson(res, resp.status === 'failed' ? 400 : 200, {
          code: resp.status === 'failed' ? 400 : 0,
          message: resp.error || 'ok',
          data: resp,
        })
      }

      // POST /internal/swap/session/:id/sign
      const signMatch = p.match(/^\/internal\/swap\/session\/([^/]+)\/sign$/)
      if (req.method === 'POST' && signMatch) {
        const sessionId = decodeURIComponent(signMatch[1])
        const body = await readBody(req)
        const resp = await sessions.submitSignature(sessionId, body)
        return sendJson(res, resp.status === 'failed' ? 400 : 200, {
          code: resp.status === 'failed' ? 400 : 0,
          message: resp.error || 'ok',
          data: resp,
        })
      }

      // POST /internal/swap/session/:id/cancel
      const cancelMatch = p.match(/^\/internal\/swap\/session\/([^/]+)\/cancel$/)
      if (req.method === 'POST' && cancelMatch) {
        const sessionId = decodeURIComponent(cancelMatch[1])
        const resp = await sessions.cancelSession(sessionId)
        return sendJson(res, 200, { code: 0, message: 'cancelled', data: resp })
      }

      // GET /internal/swap/tx/:txid
      const txMatch = p.match(/^\/internal\/swap\/tx\/([^/]+)$/)
      if (req.method === 'GET' && txMatch) {
        const txid = decodeURIComponent(txMatch[1])
        const rec = store.getTx(txid)
        if (!rec) return sendJson(res, 404, { code: 404, message: 'tx not found' })
        return sendJson(res, 200, { code: 0, message: 'ok', data: rec })
      }

      // GET /internal/swap/history
      if (req.method === 'GET' && p === '/internal/swap/history') {
        const address = url.searchParams.get('address') || undefined
        const poolId = url.searchParams.get('poolId') || undefined
        const limit = Number(url.searchParams.get('limit') || '50')
        const list = store.listTxHistory({ address, poolId, limit })
        return sendJson(res, 200, { code: 0, message: 'ok', data: { list } })
      }

      // ================= Admin-only Pool Management Routes =================
      // Protected by SWAP_ADMIN_TOKEN; NOT exposed through public Go /api/v1/swap proxy.
      if (p.startsWith('/internal/swap/admin/')) {
        const auth = verifyAdminAuth(req, config.adminToken)
        if (!auth.ok) {
          return sendJson(res, auth.status, { code: auth.status, message: auth.message })
        }

        // GET /internal/swap/admin/pools
        if (req.method === 'GET' && p === '/internal/swap/admin/pools') {
          return sendJson(res, 200, {
            code: 0,
            message: 'ok',
            data: store.listRegistryEntries(),
          })
        }

        // POST /internal/swap/admin/pools/import
        if (req.method === 'POST' && p === '/internal/swap/admin/pools/import') {
          const body = await readBody(req)
          const inspected = await inspectOnChainPoolAndBuildEntry({
            chain: store.chain,
            contracts: store.contracts,
            whitelistSwapCodeHashes: config.whitelistSwapCodeHashes,
            input: {
              poolId: body.poolId,
              pairName: body.pairName,
              poolTxid: body.poolTxid,
              lpGenesisTxid: body.lpGenesisTxid,
              lpGenesisVout: body.lpGenesisVout,
              tokenSymbol: body.tokenSymbol,
              tokenName: body.tokenName,
              tokenDecimals: body.tokenDecimals,
              lpSymbol: body.lpSymbol,
              contractHashArrayHex: body.contractHashArrayHex,
            },
          })
          const dryRun = Boolean(body.dryRun)
          if (dryRun) {
            return sendJson(res, 200, {
              code: 0,
              message: 'dry-run verified',
              data: { dryRun: true, entry: inspected.entry, onChain: inspected.onChain },
            })
          }
          const liveView = await store.registerPool(inspected.entry, {
            overwrite: Boolean(body.overwrite),
          })
          return sendJson(res, 200, {
            code: 0,
            message: 'pool imported and activated',
            data: { dryRun: false, pool: liveView, entry: inspected.entry },
          })
        }

        // POST /internal/swap/admin/pools/register
        if (req.method === 'POST' && p === '/internal/swap/admin/pools/register') {
          const body = await readBody(req)
          if (!body || !body.entry) {
            return sendJson(res, 400, { code: 400, message: 'entry payload is required' })
          }
          const liveView = await store.registerPool(body.entry, {
            overwrite: Boolean(body.overwrite),
            rawTxsToSeed: Array.isArray(body.rawTxsToSeed) ? body.rawTxsToSeed : undefined,
          })
          return sendJson(res, 200, {
            code: 0,
            message: 'pool registered and activated',
            data: { pool: liveView, entry: body.entry },
          })
        }

        // DELETE /internal/swap/admin/pools/:poolId
        const adminDelMatch = p.match(/^\/internal\/swap\/admin\/pools\/([^/]+)$/)
        if (req.method === 'DELETE' && adminDelMatch) {
          const poolId = decodeURIComponent(adminDelMatch[1])
          const removed = store.removePool(poolId)
          return sendJson(res, 200, {
            code: 0,
            message: 'pool removed',
            data: { removed },
          })
        }
      }

      return sendJson(res, 404, { code: 404, message: `Route not found: ${req.method} ${p}` })
    } catch (err: any) {
      return sendJson(res, 500, {
        code: 500,
        message: err && err.message ? err.message : String(err),
      })
    }
  })

  return { server, config, store, sessions }
}

if (require.main === module) {
  const { server, config } = createSwapServer()
  server.listen(config.port, config.host, () => {
    console.log(`[swap-service] listening on http://${config.host}:${config.port} (net=${config.net}, pools=${config.pools.length})`)
  })
}
