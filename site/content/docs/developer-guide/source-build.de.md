---
title: "Source Build"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Baue aus dem Quellcode, wenn du benutzerdefinierte Patches, Entwicklungs-Binärdateien oder Paketierungsarbeiten benötigst.

## Voraussetzungen

- Go, wie in `mise.toml` und `go.mod` deklariert.
- Git, für Versionierung und Repository-Operationen.

Verwende `mise`, um die Projekt-Toolchain zu installieren:

```bash
mise install
```

## Bauen mit dem Skript

Das Build-Skript schreibt die Ausgabe nach `dist/<os>_<arch>/` und generiert Prüfsummen, wenn `sha256sum` verfügbar ist:

```bash
sh scripts/build/binaries.sh
```

Unter Linux baut es:

- `composia`
- `composia-controller`
- `composia-agent`

Unter macOS und Windows baut es nur die CLI.

## Build-Variablen

| Variable | Standard | Zweck |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | In Binärdateien eingebettete Version. |
| `OUTPUT_DIR` | `dist` | Ausgabeverzeichnis. |
| `GOOS` | Host-Betriebssystem | Ziel-Betriebssystem. |
| `GOARCH` | Host-Architektur | Ziel-Architektur. |
| `GOARM` | Leer | ARM-Variante, z.B. `7`. |
| `CGO_ENABLED` | `0` | Statische Cross-Builds. |

Beispiele:

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## Manueller Go-Build

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## Container-Images bauen

Das `Dockerfile` im Wurzelverzeichnis hat Ziele für jede Laufzeitumgebung:

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

Das `dev`-Image enthält Go, Air, Docker CLI, Docker Buildx, Docker Compose und Git.
