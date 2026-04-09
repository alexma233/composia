#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DOC_PLUGIN_DIR=$(go env GOPATH)/bin

if ! command -v buf >/dev/null 2>&1; then
  printf '%s\n' 'buf is required to generate API docs.' >&2
  exit 1
fi

PATH="$DOC_PLUGIN_DIR:$PATH"

if ! command -v protoc-gen-doc >/dev/null 2>&1; then
  printf '%s\n' 'Installing protoc-gen-doc to generate API docs...' >&2
  GOBIN="$DOC_PLUGIN_DIR" go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
fi

mkdir -p \
  "$ROOT_DIR/docs/content/en-us/guide/api" \
  "$ROOT_DIR/docs/content/zh-hans/guide/api"

PATH="$DOC_PLUGIN_DIR:$PATH" buf generate --template "$SCRIPT_DIR/buf.gen.docs.controller.yaml" --path proto/composia/controller/v1
PATH="$DOC_PLUGIN_DIR:$PATH" buf generate --template "$SCRIPT_DIR/buf.gen.docs.agent.yaml" --path proto/composia/agent/v1/agent.proto
