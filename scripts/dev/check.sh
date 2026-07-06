#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

unformatted=$(git ls-files '*.go' | while IFS= read -r file; do gofmt -l "$file"; done)
if [ -n "$unformatted" ]; then
  printf 'Unformatted Go files:\n%s\n' "$unformatted" >&2
  exit 1
fi

go build ./...
go vet ./...
go test ./...

bun install --frozen-lockfile
bun run web:format
bun run web:check
bun run web:build
