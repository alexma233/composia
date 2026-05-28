---
title: "源码构建"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

当需要自定义补丁、开发版二进制文件或打包工作时，从源码构建。

## 前置条件

- Go，版本要求见 `mise.toml` 和 `go.mod`。
- Git，用于版本标记和仓库操作。

使用 `mise` 安装项目工具链：

```bash
mise install
```

## 使用构建脚本

构建脚本将输出写入 `dist/<os>_<arch>/`，并在 `sha256sum` 可用时生成校验和：

```bash
sh scripts/build/binaries.sh
```

在 Linux 上构建：

- `composia`
- `composia-controller`
- `composia-agent`

在 macOS 和 Windows 上仅构建 CLI。

## 构建变量

| 变量 | 默认值 | 用途 |
|----------|---------|---------|
| `VERSION` | `git describe --tags --always --dirty` | 嵌入到二进制文件中的版本号。 |
| `OUTPUT_DIR` | `dist` | 输出目录。 |
| `GOOS` | 主机 OS | 目标 OS。 |
| `GOARCH` | 主机架构 | 目标架构。 |
| `GOARM` | 空 | ARM 变体，如 `7`。 |
| `CGO_ENABLED` | `0` | 静态交叉编译。 |

示例：

```bash
GOOS=linux GOARCH=arm64 sh scripts/build/binaries.sh
GOOS=linux GOARCH=arm GOARM=7 sh scripts/build/binaries.sh
```

## 手动 Go 构建

```bash
LDFLAGS="-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=$(git describe --tags --always --dirty)"

CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia ./cmd/composia
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-controller ./cmd/composia-controller
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o composia-agent ./cmd/composia-agent
```

## 构建容器镜像

根目录的 `Dockerfile` 包含每个运行时的构建目标：

```bash
docker build --target cli -t composia-cli:local .
docker build --target controller -t composia-controller:local .
docker build --target agent -t composia-agent:local .
docker build --target dev -t composia-dev:local .
```

`dev` 镜像包含 Go、Air、Docker CLI、Docker Buildx、Docker Compose 和 Git。
