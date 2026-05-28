---
title: "Build depuis les sources"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Compilez depuis les sources lorsque vous avez besoin de correctifs personnalisés, de binaires de développement ou de travaux de packaging.

## Prérequis

- Go, tel que déclaré dans `mise.toml` et `go.mod`.
- Git, pour le versionnement et les opérations sur le dépôt.

Utilisez `mise` pour installer la chaîne d'outils du projet :

```bash
mise install
```

## Compiler avec le script

Le script de build écrit la sortie dans `dist/<os>_<arch>/` et génère des sommes de contrôle lorsque `sha256sum` est disponible :

```bash
sh scripts/build/binaries.sh
```

Sur Linux, il compile :

- `composia`
- `composia-controller`
- `composia-agent`

Sur macOS et Windows, il compile uniquement la CLI.

## Variables de build

| Variable | Valeur par défaut | Rôle |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | Version intégrée dans les binaires. |
| `OUTPUT_DIR` | `dist` | Répertoire de sortie. |
| `GOOS` | OS hôte | OS cible. |
| `GOARCH` | Architecture hôte | Architecture cible. |
| `GOARM` | Vide | Variante ARM, comme `7`. |
| `CGO_ENABLED` | `0` | Compilations croisées statiques. |

Exemples :

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## Build Go manuel

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## Build des images de conteneur

Le `Dockerfile` racine a des cibles pour chaque runtime :

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

L'image `dev` inclut Go, Air, Docker CLI, Docker Buildx, Docker Compose et Git.
