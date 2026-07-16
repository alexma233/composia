---
title: "套件管理器與二進位檔"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

此頁面用於非容器的安裝和手動二進位檔下載。

## 執行時期工具

容器映像檔已包含這些工具。非容器安裝必須在主機上提供它們。

| 主機角色 | 必要工具 |
|-----------|----------------|
| 控制器 | CA 憑證、`git`。 |
| 代理 | `git`、Docker CLI、Docker Buildx 外掛、Docker Compose 外掛、對 Docker 守護程序的存取。 |

Linux 套件和壓縮檔會安裝 Composia 二進位檔與可選的 systemd 單元檔。它們不會為您安裝 Docker、Docker Compose 或 Git。

{{< tabs >}}

{{< tab name="APT" >}}
## Debian 與 Ubuntu

加入 Forgejo APT 存放庫：

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

您也可以從[發行版本頁面](https://forgejo.alexma.top/alexma233/composia/releases)下載 `.deb` 套件並使用您的套件管理器安裝。

驗證：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora、RHEL 及相容的發行版

在 Fedora 上使用 COPR：

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

您也可以從[發行版本頁面](https://forgejo.alexma.top/alexma233/composia/releases)下載 `.rpm` 套件並使用 `dnf` 或 `yum` 安裝。

驗證：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

發布了兩個 AUR 套件：

| 套件 | 來源 |
|---------|--------|
| `composia-bin` | 預先建置的發行版本二進位檔。 |
| `composia` | 從原始碼建置。 |

使用您的 AUR 助手安裝其中之一：

```bash
paru -S composia-bin
```

或：

```bash
paru -S composia
```

兩個套件互相衝突。僅安裝一個。

您也可以從[發行版本頁面](https://forgejo.alexma.top/alexma233/composia/releases)下載 `.pkg.tar.zst` 套件並使用 `pacman -U` 安裝。

驗證：

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

從存放庫 flake 安裝：

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

直接執行二進位檔：

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

支援的 flake 系統：`x86_64-linux`、`aarch64-linux`、`i686-linux`、`armv7l-linux`、`riscv64-linux`。

nixpkgs 套件尚未合併。追蹤上游 PR：[NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061)。在它合併之前，請使用 flake。
{{< /tab >}}

{{< tab name="手動二進位檔" >}}
## 手動二進位檔下載

從[發行版本頁面](https://forgejo.alexma.top/alexma233/composia/releases)下載壓縮檔。

### 成品

| 平台 | 成品模式 | 內容 |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`、`composia-controller`、`composia-agent`、systemd 單元檔 |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Linux 架構：`amd64`、`arm64`、`armv7`、`386`、`riscv64`。

macOS 架構：`amd64`、`arm64`。

Windows 架構：`amd64`、`arm64`、`386`。

### 下載後安裝

Linux 壓縮檔：

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

macOS 壓縮檔：

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Windows 壓縮檔：

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

將 `composia.exe` 放到 `%PATH%` 中的某處。

### 驗證校驗碼

從同一發行版本下載 `checksums.txt`，並使用您平台的 SHA-256 工具驗證下載的成品。
{{< /tab >}}

{{< /tabs >}}

## 系統服務

Linux 套件會安裝未啟用的 controller 與 agent systemd 單元。套件不會自動啟用或啟動這些服務。

打包的單元使用預設設定路徑：

| 服務 | 設定路徑 |
|---------|-------------|
| `composia-controller.service` | `/etc/composia/controller/config.yaml` |
| `composia-agent.service` | `/etc/composia/agent/config.yaml` |

The packaged units use the default config paths, run as root as shipped, and do not create config files, data directories, or Git repositories for you. Bootstrap them before enabling services:

```bash
sudo install -d -m 0755 /etc/composia/controller /etc/composia/agent
sudo install -d -m 0750 \
  /var/lib/composia/controller/repo \
  /var/lib/composia/controller/state \
  /var/lib/composia/controller/logs \
  /var/lib/composia/agent/repo \
  /var/lib/composia/agent/state
sudo chown -R root:root /etc/composia /var/lib/composia
sudo git -C /var/lib/composia/controller/repo init
sudo git -C /var/lib/composia/agent/repo init
```

Use matching paths in the two config files:

```yaml {filename="/etc/composia/controller/config.yaml"}
controller:
  repo_dir: "/var/lib/composia/controller/repo"
  state_dir: "/var/lib/composia/controller/state"
  log_dir: "/var/lib/composia/controller/logs"
```

```yaml {filename="/etc/composia/agent/config.yaml"}
agent:
  repo_dir: "/var/lib/composia/agent/repo"
  state_dir: "/var/lib/composia/agent/state"
```

Verify ownership, write access, and Git initialization:

```bash
stat -c '%U:%G %a %n' \
  /var/lib/composia/controller/repo \
  /var/lib/composia/controller/state \
  /var/lib/composia/controller/logs \
  /var/lib/composia/agent/repo \
  /var/lib/composia/agent/state
sudo test -w /var/lib/composia/controller/repo
sudo test -w /var/lib/composia/agent/repo
sudo git -C /var/lib/composia/controller/repo rev-parse --is-inside-work-tree
sudo git -C /var/lib/composia/agent/repo rev-parse --is-inside-work-tree
```

If you add a systemd drop-in with `User=`, chown these paths to that service user instead.

建立設定檔後，明確啟用服務：

```bash
sudo systemctl enable --now composia-controller.service
sudo systemctl enable --now composia-agent.service
```

Linux 壓縮檔也會在 `packaging/systemd/` 下包含同一組單元檔。如果您將壓縮檔中的二進位檔安裝到 `/usr/local/bin`，請先修改單元檔中的 `ExecStart` 路徑再啟用。
