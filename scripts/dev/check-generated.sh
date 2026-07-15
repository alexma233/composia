#!/usr/bin/env sh

set -eu

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

git diff --exit-code -- gen/go web/src/lib/gen
