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

deno install --frozen
deno task web:format
deno task web:check
deno task web:test
deno task web:build
