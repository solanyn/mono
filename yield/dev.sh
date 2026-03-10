#!/usr/bin/env bash
set -euo pipefail

docker run -d --name yield-postgres \
  -p 5432:5432 \
  -e POSTGRES_DB=yield \
  -e POSTGRES_PASSWORD=dev \
  postgis/postgis:16-3.4 2>/dev/null || true

docker run -d --name yield-dragonfly \
  -p 6379:6379 \
  docker.dragonflydb.io/dragonflydb/dragonfly 2>/dev/null || true

until pg_isready -h localhost -p 5432 2>/dev/null; do sleep 0.5; done

echo "==> Dependencies ready"

DATABASE_URL="postgres://postgres:dev@localhost:5432/yield" \
REDIS_URL="localhost:6379" \
  bazel run //yield/api:api &

cd yield/web && pnpm dev &

wait
