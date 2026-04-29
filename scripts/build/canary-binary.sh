#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT_DIR"

: "${BINARY_NAME:?BINARY_NAME is required}"
: "${PACKAGE:?PACKAGE is required}"
: "${OUTPUT_DIR:?OUTPUT_DIR is required}"

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  if [ -n "${GITHUB_SHA:-}" ]; then
    VERSION="canary-${GITHUB_SHA}"
  else
    VERSION="canary-$(git rev-parse --short HEAD 2>/dev/null || printf unknown)"
  fi
fi

TARGET_OS="${GOOS:-$(go env GOOS)}"
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"
TARGET_ARM="${GOARM:-}"

EXT=""
if [ "$TARGET_OS" = "windows" ]; then
  EXT=".exe"
fi

mkdir -p "$OUTPUT_DIR"

LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=${VERSION}"

CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" GOARM="$TARGET_ARM" \
  go build -trimpath -ldflags "$LDFLAGS" -o "${OUTPUT_DIR}/${BINARY_NAME}${EXT}" "$PACKAGE"

printf 'built %s for %s/%s in %s\n' "$BINARY_NAME" "$TARGET_OS" "$TARGET_ARCH" "$OUTPUT_DIR"
