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
  GOBIN="$DOC_PLUGIN_DIR" go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v1.5.1
fi

mkdir -p \
  "$ROOT_DIR/site/content/docs/developer-guide/api"

PATH="$DOC_PLUGIN_DIR:$PATH" buf generate --template "$SCRIPT_DIR/buf.gen.docs.controller.yaml" --path proto/composia/controller/v1
PATH="$DOC_PLUGIN_DIR:$PATH" buf generate --template "$SCRIPT_DIR/buf.gen.docs.agent.yaml" --path proto/composia/agent/v1/agent.proto

for f in \
  "$ROOT_DIR/site/content/docs/developer-guide/api/controller-reference.md" \
  "$ROOT_DIR/site/content/docs/developer-guide/api/agent-internal-reference.md"; do
  name=$(basename "$f" .md)
  title=$(echo "$name" | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) substr($i,2)}1')
  frontmatter="---
title: \"$title\"
weight: 10
---"
  tmp=$(mktemp)
  printf '%s\n\n' "$frontmatter" > "$tmp"
  awk '
    NR == 1 && $0 == "---" { skip_frontmatter = 1; next }
    skip_frontmatter && $0 == "---" { skip_frontmatter = 0; next }
    !skip_frontmatter { print }
  ' "$f" >> "$tmp"
  mv "$tmp" "$f"
done
