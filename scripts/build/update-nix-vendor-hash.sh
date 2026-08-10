#!/usr/bin/env bash
set -euo pipefail

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

tar \
  --exclude='.git' \
  --exclude='./vendor' \
  -cf - . |
  tar -xf - -C "$tmp"

(
  cd "$tmp"
  GOTOOLCHAIN=local go mod vendor
)

hash="$(
  nix --extra-experimental-features nix-command \
    hash path \
    --mode nar \
    --algo sha256 \
    --format sri \
    "$tmp/vendor"
)"

echo "vendorHash = $hash"

count="$(grep -Ec 'vendorHash = "sha256-[A-Za-z0-9+/=]+";' flake.nix)"

if [ "$count" -ne 1 ]; then
  echo "Expected exactly one vendorHash in flake.nix, found $count" >&2
  exit 1
fi

sed -i -E \
  's|vendorHash = "sha256-[A-Za-z0-9+/=]+";|vendorHash = "'"$hash"'";|' \
  flake.nix
