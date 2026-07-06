#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

COMPOSE_FILE="dev/docker-compose.e2e.yaml"
CONTROLLER_ADDR="http://127.0.0.1:7001"
ACCESS_TOKEN="dev-admin-token"

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v
}
trap cleanup EXIT INT TERM

bun install --frozen-lockfile
bunx playwright install chromium

docker compose -f "$COMPOSE_FILE" up -d --build controller-dev agent-dev

for attempt in $(seq 1 60); do
  if curl -fsS \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H 'Connect-Protocol-Version: 1' \
    -H 'Content-Type: application/json' \
    -H 'X-Composia-Source: local-e2e' \
    --data '{}' \
    "$CONTROLLER_ADDR/api/controller/composia.controller.v1.SystemService/GetSystemStatus" >/dev/null; then
    break
  fi

  if [ "$attempt" = "60" ]; then
    docker compose -f "$COMPOSE_FILE" logs --no-color controller-dev agent-dev
    exit 1
  fi

  sleep 2
done

COMPOSIA_E2E_CONTROLLER_ADDR="$CONTROLLER_ADDR" \
COMPOSIA_E2E_ACCESS_TOKEN="$ACCESS_TOKEN" \
go test -tags=e2e ./test/e2e

COMPOSIA_E2E_CONTROLLER_ADDR="$CONTROLLER_ADDR" \
COMPOSIA_E2E_ACCESS_TOKEN="$ACCESS_TOKEN" \
go test -tags=e2e ./test/controller_e2e

WEB_CONTROLLER_ADDR="$CONTROLLER_ADDR" \
WEB_BROWSER_CONTROLLER_ADDR="$CONTROLLER_ADDR" \
WEB_CONTROLLER_ACCESS_TOKEN="$ACCESS_TOKEN" \
WEB_LOGIN_USERNAME="admin" \
WEB_LOGIN_PASSWORD_HASH='$argon2id$v=19$m=65536,t=3,p=4$/wh05hbH5ipiT42CK+GxVA$2unNmHbsRe5ZkFgIkHNekBGk6KH+79sZAPB9qmRrUlQ' \
WEB_SESSION_SECRET="e2e-session-secret" \
ORIGIN="http://127.0.0.1:4173" \
HOST="127.0.0.1" \
PORT="4173" \
bun run web:e2e
