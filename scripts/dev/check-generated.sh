#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

GOBIN_DIR=$(mktemp -d)
trap 'rm -rf "$GOBIN_DIR"' EXIT

PROTOBUF_VERSION=$(go list -m -f '{{.Version}}' google.golang.org/protobuf)
CONNECT_VERSION=$(go list -m -f '{{.Version}}' connectrpc.com/connect)

if ! command -v buf >/dev/null 2>&1; then
  printf '%s\n' 'buf is required to generate protobuf code.' >&2
  exit 1
fi

GOBIN="$GOBIN_DIR" go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOBUF_VERSION"
GOBIN="$GOBIN_DIR" go install "connectrpc.com/connect/cmd/protoc-gen-connect-go@$CONNECT_VERSION"

deno install --frozen
PATH="$GOBIN_DIR:$ROOT_DIR/node_modules/.bin:$ROOT_DIR/web/node_modules/.bin:$PATH" buf generate

git diff --exit-code -- gen/go web/src/lib/gen
