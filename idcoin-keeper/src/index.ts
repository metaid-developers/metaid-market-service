import { loadConfig } from './config'
import { KeeperDb } from './db'
import { CompositeChainClient, HigunBtcClient, MempoolClient } from './chain'
import { Keeper } from './keeper'
import { refreshCoinRegistry } from './registry'
import { startOpsServer } from './ops'
import { Logger } from './log'

async function main() {
  const config = loadConfig()
  const log = new Logger()
  log.info('idcoin-keeper starting', {
    dryRun: config.dryRun,
    killSwitch: config.killSwitch,
    enableTicks: [...config.enableTicks],
    pollMs: config.pollMs,
  })

  const db = new KeeperDb(config.dbPath)
  const higun = new HigunBtcClient(config.higunBtcUrl)
  const mempool = new MempoolClient(config.mempoolUrl)
  const chain = new CompositeChainClient(higun, mempool, (address, detail) => {
    db.recordDiscrepancy(address, detail, Date.now())
    log.warn('chain discrepancy', { address, detail })
  })

  const keeper = new Keeper({ config, db, chain, log })

  const server = startOpsServer(keeper, config.opsPort)
  log.info('ops endpoint listening', { bind: '127.0.0.1', port: config.opsPort })

  const tick = async () => {
    try {
      await refreshCoinRegistry({
        coinsUrl: config.coinsUrl,
        accountNode: keeper.lpKeys.accountNode,
        legacyKeys: keeper.legacyKeys,
        db,
        log,
      })
    } catch (e: any) {
      log.error('coin registry refresh failed', { error: e.message })
    }
    try {
      await keeper.sweepTick()
    } catch (e: any) {
      log.error('sweep tick failed', { error: e.message })
    }
    try {
      await keeper.vaultLedgerTick()
    } catch (e: any) {
      log.error('vault ledger tick failed', { error: e.message })
    }
  }

  await tick()
  const timer = setInterval(tick, config.pollMs)

  const shutdown = () => {
    clearInterval(timer)
    server.close()
    db.close()
    log.info('idcoin-keeper stopped')
    process.exit(0)
  }
  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)
}

main().catch((e) => {
  // startup errors may mention file names but never secrets — redacted by Logger
  console.error('fatal startup error:', e.message)
  process.exit(1)
})
