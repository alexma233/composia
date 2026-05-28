---
title: "原始碼建置"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

當您需要自訂修補、開發用二進位檔或打包作業時，從原始碼建置。

## 前置需求

- Go，版本如 `mise.toml` 和 `go.mod` 中所宣告。
- Git，用於版本戳記與存放庫操作。

使用 `mise` 安裝專案工具鏈：

```bash
mise install
```

## 使用腳本建置

建置腳本將輸出寫入 `dist/<os>_<arch>/`，並在 `sha256sum` 可用時產生校驗碼：

```bash
sh scripts/build/binaries.sh
```

在 Linux 上會建置：

- `composia`
- `composia-controller`
- `composia-agent`

在 macOS 和 Windows 上僅建置 CLI。

## 建置變數

| 變數 | 預設值 | 用途 |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | 嵌入二進位檔的版本。 |
| `OUTPUT_DIR` | `dist` | 輸出目錄。 |
| `GOOS` | 主機 OS | 目標 OS。 |
| `GOARCH` | 主機架構 | 目標架構。 |
| `GOARM` | 空白 | ARM 變體，如 `7`。 |
| `CGO_ENABLED` | `0` | 靜態跨平台建置。 |

範例：

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## 手動 Go 建置

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## 建置容器映像檔

根目錄的 `Dockerfile` 為每個執行時期設有建置目標：

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

`dev` 映像檔包含 Go、Air、Docker CLI、Docker Buildx、Docker Compose 與 Git。
