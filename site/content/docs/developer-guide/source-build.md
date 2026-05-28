---
title: "Source Build"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Build from source when you need custom patches, development binaries, or packaging work.

## Prerequisites

- Go, as declared in `mise.toml` and `go.mod`.
- Git, for version stamping and repository operations.

Use `mise` to install the project toolchain:

```bash
mise install
```

## Build with the script

The build script writes output to `dist/<os>_<arch>/` and generates checksums when `sha256sum` is available:

```bash
sh scripts/build/binaries.sh
```

On Linux it builds:

- `composia`
- `composia-controller`
- `composia-agent`

On macOS and Windows it builds the CLI only.

## Build variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | Version embedded into binaries. |
| `OUTPUT_DIR` | `dist` | Output directory. |
| `GOOS` | Host OS | Target OS. |
| `GOARCH` | Host architecture | Target architecture. |
| `GOARM` | Empty | ARM variant, such as `7`. |
| `CGO_ENABLED` | `0` | Static cross-builds. |

Examples:

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## Manual Go build

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## Build container images

The root `Dockerfile` has targets for each runtime:

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

The `dev` image includes Go, Air, Docker CLI, Docker Buildx, Docker Compose, and Git.
