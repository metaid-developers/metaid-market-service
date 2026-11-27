import * as http from 'http'
import { Keeper } from './keeper'

/** Ops/health endpoint bound to 127.0.0.1 ONLY. Returns public data — never
 *  key material (no WIFs/mnemonic exist in the status payload by construction). */
export function startOpsServer(keeper: Keeper, port: number): http.Server {
  const server = http.createServer((req, res) => {
    if (req.url === '/health' || req.url === '/status') {
      const body = JSON.stringify(keeper.status(), null, 2)
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(body)
    } else {
      res.writeHead(404, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'not found' }))
    }
  })
  server.listen(port, '127.0.0.1')
  return server
}
