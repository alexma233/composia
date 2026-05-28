---
title: "ソースビルド"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

カスタムパッチ、開発用バイナリ、パッケージング作業が必要な場合はソースからビルドします。

## 前提条件

- `mise.toml` と `go.mod` で宣言されているバージョンの Go。
- バージョンスタンプとリポジトリ操作用の Git。

プロジェクトツールチェーンのインストールには `mise` を使用します:

```bash
mise install
```

## スクリプトでのビルド

ビルドスクリプトは出力を `dist/<os>_<arch>/` に書き込み、`sha256sum` が利用可能な場合はチェックサムを生成します:

```bash
sh scripts/build/binaries.sh
```

Linux では以下をビルドします:

- `composia`
- `composia-controller`
- `composia-agent`

macOS と Windows では CLI のみをビルドします。

## ビルド変数

| 変数 | デフォルト | 目的 |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | バイナリに埋め込まれるバージョン。 |
| `OUTPUT_DIR` | `dist` | 出力ディレクトリ。 |
| `GOOS` | ホスト OS | ターゲット OS。 |
| `GOARCH` | ホストアーキテクチャ | ターゲットアーキテクチャ。 |
| `GOARM` | 空 | ARM バリアント（例: `7`）。 |
| `CGO_ENABLED` | `0` | 静的クロスビルド。 |

例:

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## 手動 Go ビルド

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## コンテナイメージのビルド

ルートの `Dockerfile` には各ランタイム向けのターゲットがあります:

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

`dev` イメージには Go、Air、Docker CLI、Docker Buildx、Docker Compose、Git が含まれています。
