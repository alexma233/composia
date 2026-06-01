#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT_DIR"

OUTPUT_DIR="${COMPOSIA_COMPLETIONS_DIR:-dist/completions}"
mkdir -p "$OUTPUT_DIR"

go run ./cmd/composia completion bash > "${OUTPUT_DIR}/composia.bash"
go run ./cmd/composia completion zsh > "${OUTPUT_DIR}/_composia"
go run ./cmd/composia completion fish > "${OUTPUT_DIR}/composia.fish"

printf 'generated shell completions in %s\n' "$OUTPUT_DIR"
