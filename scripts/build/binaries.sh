#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT_DIR"

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --always --dirty 2>/dev/null || printf 'v0.0.0-dev')"
fi

OUTPUT_DIR="${OUTPUT_DIR:-dist}"
TARGET_OS="${GOOS:-$(go env GOOS)}"
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"
TARGET_ARM="${GOARM:-}"

TARGET="${TARGET_OS}_${TARGET_ARCH}"
if [ -n "$TARGET_ARM" ]; then
  TARGET="${TARGET}_v${TARGET_ARM}"
fi

BIN_DIR="${OUTPUT_DIR}/${TARGET}"
mkdir -p "$BIN_DIR"

EXT=""
if [ "$TARGET_OS" = "windows" ]; then
  EXT=".exe"
fi

LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=${VERSION}"

build_binary() {
  name="$1"
  package="$2"
  CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" GOARM="$TARGET_ARM" \
    go build -trimpath -ldflags "$LDFLAGS" -o "${BIN_DIR}/${name}${EXT}" "$package"
}

build_binary composia ./cmd/composia

RUNTIME_BINARIES=0
if [ "$TARGET_OS" = "linux" ]; then
  RUNTIME_BINARIES=1
  build_binary composia-controller ./cmd/composia-controller
  build_binary composia-agent ./cmd/composia-agent
fi

if command -v sha256sum >/dev/null 2>&1; then
  (
    cd "$BIN_DIR"
    if [ "$RUNTIME_BINARIES" -eq 1 ]; then
      sha256sum "composia${EXT}" "composia-controller${EXT}" "composia-agent${EXT}" > checksums.txt
    else
      sha256sum "composia${EXT}" > checksums.txt
    fi
  )
fi

printf 'built %s binaries in %s\n' "$VERSION" "$BIN_DIR"
