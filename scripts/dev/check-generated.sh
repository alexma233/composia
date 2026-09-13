#!/usr/bin/env sh
#
# Regenerates the protobuf output with the tool versions CI pins and fails when
# the committed output differs. Renovate's postUpgradeTasks passes --write to
# regenerate without that check, so generator bumps ship their own output.
#
# Usage: check-generated.sh [--write]

set -eu

write=0
case "${1:-}" in
  "") ;;
  --write) write=1 ;;
  *)
    printf 'usage: %s [--write]\n' "$0" >&2
    exit 2
    ;;
esac

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

GOBIN_DIR=$(mktemp -d)
trap 'rm -rf "$GOBIN_DIR"' EXIT

BUF_VERSION=$(sed -n 's/^buf = "\([^"]*\)"$/\1/p' mise.toml)
PROTOBUF_VERSION=$(go list -m -f '{{.Version}}' google.golang.org/protobuf)
CONNECT_VERSION=$(go list -m -f '{{.Version}}' connectrpc.com/connect)

GOBIN="$GOBIN_DIR" go install "github.com/bufbuild/buf/cmd/buf@v$BUF_VERSION"
GOBIN="$GOBIN_DIR" go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOBUF_VERSION"
GOBIN="$GOBIN_DIR" go install "connectrpc.com/connect/cmd/protoc-gen-connect-go@$CONNECT_VERSION"

deno install --frozen
PATH="$GOBIN_DIR:$ROOT_DIR/node_modules/.bin:$ROOT_DIR/web/node_modules/.bin:$PATH" buf generate

if [ "$write" = "1" ]; then
  exit 0
fi

git diff --exit-code -- gen/go web/src/lib/gen
