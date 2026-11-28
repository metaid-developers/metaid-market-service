#!/bin/sh
# make-build-context.sh [output-dir]
# Produces a minimal Docker build context for Dockerfile.prod:
#   Dockerfile.prod + orderbook-service/src + idcoin-keeper (no node_modules,
#   no tests, no db, no dist) and packs it as keeper-build-context.tar.gz.
set -e

OUT="${1:-/tmp/idcoin-keeper-build-context}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KEEPER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$KEEPER_DIR/.." && pwd)"

rm -rf "$OUT"
mkdir -p "$OUT/orderbook-service" "$OUT/idcoin-keeper"

cp "$KEEPER_DIR/Dockerfile.prod" "$OUT/Dockerfile.prod"
mkdir -p "$OUT/orderbook-service/src" && cp "$REPO_ROOT"/orderbook-service/src/*.ts "$OUT/orderbook-service/src/"

cp -R "$KEEPER_DIR/src" "$OUT/idcoin-keeper/src"
cp "$KEEPER_DIR/package.json" "$KEEPER_DIR/package-lock.json" "$OUT/idcoin-keeper/"
cp "$KEEPER_DIR/tsconfig.json" "$KEEPER_DIR/tsconfig.build.json" "$OUT/idcoin-keeper/"

# sanity: nothing forbidden leaked into the context
if find "$OUT" -name node_modules -o -name '*.db' -o -name '*.js' | grep -q .; then
  echo "ERROR: context contains node_modules or db files" >&2
  exit 1
fi

TAR="$OUT/keeper-build-context.tar.gz"
tar -czf "$TAR" -C "$OUT" Dockerfile.prod orderbook-service idcoin-keeper
echo "build context: $OUT"
echo "tarball:       $TAR ($(du -h "$TAR" | cut -f1))"
echo ""
echo "build with:"
echo "  cd $OUT && docker build -f Dockerfile.prod -t idcoin-keeper:v1.0 ."
