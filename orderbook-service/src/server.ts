import * as http from 'http'
import { loadConfig, OrderbookSidecarConfig } from './config'
import { loadOrderbookContracts } from './core-bridge'
import { ChainProvider, HigunChainProvider } from './chain-provider'
import { OrderbookTxBuilder } from './tx-builder'

function readJsonBody(req: http.IncomingMessage, maxBytes = 2 << 20): Promise<any> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = []
    let total = 0
    req.on('data', (chunk: Buffer) => {
      total += chunk.length
      if (total > maxBytes) {
        reject(new Error('request body too large'))
        req.destroy()
        return
      }
      chunks.push(chunk)
    })
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8').trim()
      if (!raw) return resolve({})
      try {
        resolve(JSON.parse(raw))
      } catch (e: any) {
        reject(new Error(`invalid JSON body: ${e.message}`))
      }
    })
    req.on('error', reject)
  })
}

function sendJson(res: http.ServerResponse, status: number, payload: any): void {
  const body = JSON.stringify(payload)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(body),
  })
  res.end(body)
}

export function createOrderbookSidecarServer(builder: OrderbookTxBuilder): http.Server {
  return http.createServer(async (req, res) => {
    const url = (req.url || '/').split('?')[0]
    const method = (req.method || 'GET').toUpperCase()

    try {
      if (method === 'GET' && url === '/internal/ob/health') {
        return sendJson(res, 200, { ok: true, contracts: builder.getContractMeta() })
      }
      if (method === 'GET' && url === '/internal/ob/contracts') {
        return sendJson(res, 200, { code: 0, data: builder.getContractMeta() })
      }
      if (method === 'POST' && (url === '/internal/ob/place/ask/prepare' || url === '/internal/ob/build-place-ask')) {
        const body = await readJsonBody(req)
        const data = await builder.buildPlaceAsk(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/place/bid/prepare' || url === '/internal/ob/build-place-bid')) {
        const body = await readJsonBody(req)
        const data = await builder.buildPlaceBid(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/place/token/prepare' || url === '/internal/ob/build-place-token')) {
        const body = await readJsonBody(req)
        const data = await builder.buildPlaceTokenOrder(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/take/ask/prepare' || url === '/internal/ob/build-take-ask')) {
        const body = await readJsonBody(req)
        const data = await builder.buildTakeAsk(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/take/bid/prepare' || url === '/internal/ob/build-take-bid')) {
        const body = await readJsonBody(req)
        const data = await builder.buildTakeBid(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/take/token/prepare' || url === '/internal/ob/build-take-token')) {
        const body = await readJsonBody(req)
        const data = await builder.buildTakeTokenOrder(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/cancel/prepare' || url === '/internal/ob/build-cancel')) {
        const body = await readJsonBody(req)
        const data = await builder.buildCancel(body)
        return sendJson(res, 200, { code: 0, data })
      }
      if (method === 'POST' && (url === '/internal/ob/finalize' || url === '/internal/ob/finalize-and-broadcast')) {
        const body = await readJsonBody(req)
        const broadcast = body.broadcast !== undefined ? Boolean(body.broadcast) : true
        const data = await builder.finalizeAndBroadcast(body.draftId, body.signatures || [], broadcast)
        data.draftId = body.draftId
        return sendJson(res, 200, { code: 0, data })
      }
      return sendJson(res, 404, { code: 404, message: `unknown route: ${method} ${url}` })
    } catch (err: any) {
      const msg = err?.message || String(err)
      const payload: Record<string, any> = { code: 400, message: msg }
      if (Array.isArray(err?.conflictingOutpoints) && err.conflictingOutpoints.length > 0) {
        payload.conflictingOutpoints = err.conflictingOutpoints
      }
      if (Array.isArray(err?.settledOutpoints) && err.settledOutpoints.length > 0) {
        payload.settledOutpoints = err.settledOutpoints
      }
      return sendJson(res, 400, payload)
    }
  })
}

export function startSidecar(cfgOverride?: Partial<OrderbookSidecarConfig>, chainOverride?: ChainProvider) {
  const cfg = { ...loadConfig(), ...cfgOverride }
  const contracts = loadOrderbookContracts(cfg.artifactsDir)
  const chain =
    chainOverride ||
    new HigunChainProvider({
      higunUrl: cfg.higunUrl,
      metaletApiBase: cfg.metaletApiBase,
      net: cfg.net,
    })
  const builder = new OrderbookTxBuilder({
    contracts,
    chain,
    net: cfg.net,
    feeRateSatPerByte: cfg.feeRateSatPerByte,
    draftTtlMs: cfg.draftTtlMs,
  })
  const server = createOrderbookSidecarServer(builder)
  return { cfg, contracts, chain, builder, server }
}

if (require.main === module) {
  const { cfg, contracts, server } = startSidecar()
  server.listen(cfg.port, cfg.host, () => {
    console.log(
      `[orderbook-service] listening on http://${cfg.host}:${cfg.port} (net=${cfg.net}, askCodeHash=${contracts.askCodeHashHex}, bidCodeHash=${contracts.bidCodeHashHex})`
    )
  })
}
