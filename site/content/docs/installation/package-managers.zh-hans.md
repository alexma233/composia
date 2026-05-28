---
title: "包管理器与二进制文件"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

本页适用于非容器安装和手动下载二进制文件。

## 运行时工具

容器镜像已包含这些工具。非容器安装必须在主机上提供它们。

| 主机角色 | 必需工具 |
|-----------|----------------|
| Controller | CA 证书、`git`。 |
| Agent | `git`、Docker CLI、Docker Buildx 插件、Docker Compose 插件、Docker 守护进程访问权限。 |

软件包或归档仅安装 Composia 二进制文件。它不会为您安装 Docker、Docker Compose 或 Git。

{{< tabs >}}

{{< tab name="APT" >}}
## Debian 和 Ubuntu

添加 Forgejo APT 仓库：

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

您也可以从[发布页面](https://forgejo.alexma.top/alexma233/composia/releases)下载 `.deb` 包并使用包管理器安装。

验证：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora、RHEL 及兼容发行版

在 Fedora 上使用 COPR：

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

您也可以从[发布页面](https://forgejo.alexma.top/alexma233/composia/releases)下载 `.rpm` 包并使用 `dnf` 或 `yum` 安装。

验证：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

发布有两个 AUR 包：

| 包 | 来源 |
|---------|--------|
| `composia-bin` | 预构建的发布二进制文件。 |
| `composia` | 从源码构建。 |

使用您的 AUR 助手安装其中之一：

```bash
paru -S composia-bin
```

或：

```bash
paru -S composia
```

两个包互斥。只能安装其中一个。

您也可以从[发布页面](https://forgejo.alexma.top/alexma233/composia/releases)下载 `.pkg.tar.zst` 包并使用 `pacman -U` 安装。

验证：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

从仓库 flake 安装：

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

直接运行二进制文件：

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

支持的 flake 系统：`x86_64-linux`、`aarch64-linux`、`i686-linux`、`armv7l-linux`、`riscv64-linux`。

nixpkgs 包尚未合并。请关注上游 PR [NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061)。在此之前，请使用 flake。
{{< /tab >}}

{{< tab name="手动下载二进制" >}}
## 手动下载二进制文件

从[发布页面](https://forgejo.alexma.top/alexma233/composia/releases)下载归档文件。

### 产物

| 平台 | 产物模式 | 内容 |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`、`composia-controller`、`composia-agent` |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Linux 架构：`amd64`、`arm64`、`armv7`、`386`、`riscv64`。

macOS 架构：`amd64`、`arm64`。

Windows 架构：`amd64`、`arm64`、`386`。

### 下载后安装

Linux 归档：

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

macOS 归档：

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Windows 归档：

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

将 `composia.exe` 放在 `%PATH%` 中的某个位置。

### 验证校验和

从同一发布版本下载 `checksums.txt`，并使用您平台的 SHA-256 工具验证下载的产物。
{{< /tab >}}

{{< /tabs >}}

## 系统服务

在容器外运行控制器或 agent 时，使用您的服务管理器使它们保持运行。

systemd 控制器单元示例：

```ini {filename="/etc/systemd/system/composia-controller.service"}
[Unit]
Description=Composia Controller
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/composia-controller -config /etc/composia/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

systemd agent 单元示例：

```ini {filename="/etc/systemd/system/composia-agent.service"}
[Unit]
Description=Composia Agent
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
ExecStart=/usr/bin/composia-agent -config /etc/composia/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

根据您的环境调整用户、组、配置路径和 Docker 套接字权限。
