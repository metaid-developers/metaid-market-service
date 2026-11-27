# idcoin-keeper

Protocol-owned liquidity keeper for ID-Coins. Sweeps mint payments that still
arrive at the legacy per-coin pool addresses into per-coin LP vaults
(`m/84'/0'/1000'/<UTF-8 bytes of UPPER(tick)>` P2WPKH), tracks the LP vault
ledger, and (from P2) will batch vault funds through the octopus bridge into
WBTC and place uncancellable burn-maker floor bids on the FT/FT orderbook.

P1b scope: config, keys, coin registry, legacy sweeping, vault ledger, ops
endpoint. Bridge batching and floor-bid placement are NOT implemented yet.

## Safety model

- **`IDCOIN_KEEPER_DRY_RUN` (default `true`)** — sweeps are built, signed,
  validated and recorded, but never broadcast. Dry-run records never consume a
  UTXO: flipping to `false` later sweeps everything for real, exactly once.
- **`IDCOIN_KEEPER_KILL` (default `false`)** — when `true`, all sweeping halts.
- **`IDCOIN_KEEPER_ENABLE_TICKS` (default empty)** — comma-separated UPPER
  ticks, e.g. `SUNNY,METAID`. Empty = nothing is ever swept. Only coins that
  are BOTH enabled here AND classified `legacy` (their mint `payTo` still
  equals the legacy pool address) are swept.
- Sweep invariants are re-validated on the decoded signed transaction before
  any broadcast: every input belongs to that coin's legacy payTo, exactly one
  output pays that coin's LP vault, amount = inputs − fee.
- Fee safety: sweep is skipped (with an alert) if the fee would exceed
  `IDCOIN_KEEPER_MAX_FEE_RATIO` (default 0.3) of the swept value, or the output
  would be dust (<546 sats).
- Key-compromise detection: a previously seen pool UTXO that vanishes without a
  live keeper sweep, or a swept input spent by a foreign txid, raises a
  persisted `alert`-level log record.

## Environment variables

| var | default | meaning |
|---|---|---|
| `IDCOIN_LP_MNEMONIC_FILE` | — (required) | BIP39 mnemonic file for LP vault derivation |
| `IDCOIN_LEGACY_KEYS_FILE` | — (required) | legacy pool keys JSON; every WIF is re-derived and startup FAILS on any address mismatch |
| `IDCOIN_KEEPER_DB` | `./idcoin-keeper.db` | SQLite path |
| `IDCOIN_KEEPER_POLL_MS` | `10000` | sweep/ledger poll interval |
| `IDCOIN_HIGUN_BTC_URL` | `http://172.31.165.190:8085` | Higun BTC compat proxy (primary UTXO source) |
| `IDCOIN_MEMPOOL_URL` | `https://mempool.space` | cross-check, fee oracle, broadcast, outspends |
| `IDCOIN_COINS_URL` | api.metaid.market coins-list | coin registry source |
| `IDCOIN_KEEPER_DRY_RUN` | `true` | never broadcast when true |
| `IDCOIN_KEEPER_KILL` | `false` | global kill switch |
| `IDCOIN_KEEPER_ENABLE_TICKS` | `` (empty) | per-tick allowlist, csv |
| `IDCOIN_KEEPER_FEE_FLOOR` | `2` | sat/vB floor |
| `IDCOIN_KEEPER_MAX_FEE_RATIO` | `0.3` | fee/value cap for a sweep tx |
| `IDCOIN_KEEPER_OPS_PORT` | `18090` | ops endpoint port (127.0.0.1 only) |

`IDCOIN_KEEPER_CONFIG` may point at a YAML file with the same keys (camelCase);
environment variables win.

## Fee oracle

mempool.space `fastestFee`, floored at `IDCOIN_KEEPER_FEE_FLOOR`. If the oracle
is unreachable the keeper does NOT skip sweeping: it uses
`max(floor, last-known fastestFee (persisted in the DB), 10 sat/vB)` and
records a `fee-oracle-fallback` alert.

## Known limitation: inscription detection

The Higun compat proxy (`/address/btc-utxo`) currently drops the upstream
inscription-carrier fields, so the keeper **cannot distinguish
inscription-bearing UTXOs** on production data. The parser understands
`inscriptions` / `inscriptionIds` / `isInscriptionCarrier` whenever they are
present, but with the current proxy they never are. In practice the protection
is the ≥1000-sat eligibility floor: mint payments are 10,000+ sats while
inscription carriers are typically dust-level. Do not treat sweep inputs as
guaranteed inscription-free until the proxy passes those fields through.

## Running a tick live (operational procedure)

1. Deploy with `IDCOIN_KEEPER_DRY_RUN=true`, `IDCOIN_KEEPER_ENABLE_TICKS=""`.
   Verify `/status` shows coins classified correctly.
2. Enable one tick: `IDCOIN_KEEPER_ENABLE_TICKS="SUNNY"`, restart. Watch the
   logs for `DRY-RUN sweep built` lines; inspect the recorded txs in SQLite
   (`sweep_txs.hex` — decode and verify by hand if desired).
3. Flip `IDCOIN_KEEPER_DRY_RUN=false`, restart. UTXOs seen during dry-run are
   now swept for real, exactly once each.
4. To halt everything instantly: `IDCOIN_KEEPER_KILL=true` + restart (or
   `docker compose stop`).

## Ops endpoint

`http://127.0.0.1:18090/status` (and `/health`) — JSON: dry-run/kill state,
enabled ticks, last poll time, MVC LP funding address, per-coin
mode/payTo/lpVault, row counts. No secret material is ever included (tested).
Inside a container: `docker exec idcoin-keeper curl -fsS http://127.0.0.1:18090/status`.

## Development

```
npm install     # requires node 20 (better-sqlite3 prebuilt); nvm use 20
npm test        # 29 tests, all HTTP mocked, generated keys only
npm start       # runs src/index.ts via ts-node
```

## Deployment

See `Dockerfile` and `docker-compose.example.yml` — node:20 slim, production
deps only, non-root user, no published ports, secret files bind-mounted
read-only, SQLite on a named volume, json-file log rotation (100m × 5),
`restart: unless-stopped`, healthcheck against the localhost ops endpoint.
