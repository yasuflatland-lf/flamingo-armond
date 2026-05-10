#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="backend/.env.local"

TOKEN=$(grep '^NOTION_SYNC_TOKEN=' "$ENV_FILE" | cut -d= -f2- || true)
if [ -z "${TOKEN:-}" ]; then
  echo "NOTION_SYNC_TOKEN not found in $ENV_FILE." >&2
  echo "Run 'make notion-local-setup' first." >&2
  exit 1
fi

PORT=$(grep '^PORT=' "$ENV_FILE" | cut -d= -f2- || true)
PORT=${PORT:-1323}
URL="http://localhost:${PORT}/internal/notion-sync"

if ! curl --silent --output /dev/null --connect-timeout 2 "http://localhost:${PORT}/" 2>/dev/null; then
  echo "Backend not reachable on :${PORT}." >&2
  echo "Start it with 'make dev-backend' in another terminal." >&2
  exit 1
fi

exec curl --fail-with-body --show-error -X POST "$URL" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json"
