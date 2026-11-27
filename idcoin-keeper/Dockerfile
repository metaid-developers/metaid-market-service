# idcoin-keeper — protocol-owned ID-Coin liquidity keeper.
#
# IMPORTANT BUILD CONTEXT: P2 imports ../orderbook-service (in-repo sibling)
# and resolves mvcswap-core at runtime for the compiled sCrypt contracts, so
# the Docker build context must be the metaid-market-service REPO ROOT and a
# mvcswap-core checkout must be present as its sibling directory:
#   docker build -f idcoin-keeper/Dockerfile .
# (see docker-compose.example.yml — the compose file lives in idcoin-keeper/
# but sets the context accordingly)

# ---- build stage ----
FROM node:20-bookworm-slim AS build
WORKDIR /app
COPY idcoin-keeper/package.json idcoin-keeper/package-lock.json idcoin-keeper/tsconfig.json idcoin-keeper/tsconfig.build.json ./idcoin-keeper/
COPY idcoin-keeper/src ./idcoin-keeper/src
COPY orderbook-service/src ./orderbook-service/src
COPY orderbook-service/package.json orderbook-service/tsconfig.json ./orderbook-service/
RUN cd idcoin-keeper && npm ci --no-audit --no-fund && npx tsc -p tsconfig.build.json

# ---- deps stage: production node_modules only ----
FROM node:20-bookworm-slim AS deps
WORKDIR /app/idcoin-keeper
COPY idcoin-keeper/package.json idcoin-keeper/package-lock.json ./
RUN npm ci --omit=dev --no-audit --no-fund

# ---- runtime stage ----
FROM node:20-bookworm-slim
ENV NODE_ENV=production
WORKDIR /app

RUN apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=deps /app/idcoin-keeper/node_modules ./idcoin-keeper/node_modules
COPY --from=build /app/idcoin-keeper/dist ./dist
COPY idcoin-keeper/package.json ./idcoin-keeper/package.json

# mvcswap-core is required at runtime (sCrypt contract compilation +
# mvc-scrypt resolution). Mount or copy it as a sibling of the repo root:
# the resolver accepts /app/mvcswap-core — mount a checkout there (read-only).

RUN mkdir -p /data && chown -R node:node /data /app
USER node

ENV IDCOIN_KEEPER_DB=/data/idcoin-keeper.db \
    IDCOIN_KEEPER_OPS_PORT=18090

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -fsS http://127.0.0.1:${IDCOIN_KEEPER_OPS_PORT}/health >/dev/null || exit 1

CMD ["node", "dist/idcoin-keeper/src/index.js"]
