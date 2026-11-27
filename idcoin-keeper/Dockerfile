# idcoin-keeper — protocol-owned ID-Coin liquidity keeper.
# node:20 runtime, production deps only, non-root, no published ports
# (the ops endpoint binds 127.0.0.1 inside the container).

# ---- build stage: full deps (incl. typescript) + compile to plain JS ----
FROM node:20-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json tsconfig.json tsconfig.build.json ./
COPY src ./src
RUN npm ci --no-audit --no-fund && npx tsc -p tsconfig.build.json

# ---- deps stage: production node_modules only (better-sqlite3 prebuilt
#      binaries exist for node20 linux-x64) ----
FROM node:20-bookworm-slim AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-audit --no-fund

# ---- runtime stage ----
FROM node:20-bookworm-slim
ENV NODE_ENV=production
WORKDIR /app

RUN apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=deps /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
COPY package.json ./

RUN mkdir -p /data && chown -R node:node /data /app
USER node

ENV IDCOIN_KEEPER_DB=/data/idcoin-keeper.db \
    IDCOIN_KEEPER_OPS_PORT=18090

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -fsS http://127.0.0.1:${IDCOIN_KEEPER_OPS_PORT}/health >/dev/null || exit 1

CMD ["node", "dist/index.js"]
