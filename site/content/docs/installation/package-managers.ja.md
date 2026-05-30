---
title: "パッケージマネージャーとバイナリ"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

このページは非コンテナインストールと手動バイナリダウンロード向けです。

## ランタイムツール

コンテナイメージにはこれらのツールがすでに含まれています。非コンテナインストールではホスト上にこれらを用意する必要があります。

| ホストロール | 必要なツール |
|-----------|----------------|
| コントローラー | CA 証明書、`git`。 |
| エージェント | `git`、Docker CLI、Docker Buildx プラグイン、Docker Compose プラグイン、Docker デーモンへのアクセス。 |

Linux パッケージとアーカイブは Composia バイナリと任意の systemd ユニットファイルをインストールします。Docker、Docker Compose、Git はインストールされません。

{{< tabs >}}

{{< tab name="APT" >}}
## Debian および Ubuntu

Forgejo APT リポジトリを追加します:

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

[リリースページ](https://forgejo.alexma.top/alexma233/composia/releases) から `.deb` パッケージをダウンロードし、パッケージマネージャーでインストールすることもできます。

確認:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora、RHEL、および互換ディストリビューション

Fedora では COPR を使用します:

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

[リリースページ](https://forgejo.alexma.top/alexma233/composia/releases) から `.rpm` パッケージをダウンロードし、`dnf` または `yum` でインストールすることもできます。

確認:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

2 つの AUR パッケージが公開されています:

| パッケージ | ソース |
|---------|--------|
| `composia-bin` | ビルド済みリリースバイナリ。 |
| `composia` | ソースからビルド。 |

AUR ヘルパーでいずれかをインストールします:

```bash
paru -S composia-bin
```

または:

```bash
paru -S composia
```

2 つのパッケージは互いに競合します。いずれか 1 つだけをインストールしてください。

[リリースページ](https://forgejo.alexma.top/alexma233/composia/releases) から `.pkg.tar.zst` パッケージをダウンロードし、`pacman -U` でインストールすることもできます。

確認:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

リポジトリフレークからインストールします:

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

バイナリを直接実行:

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

対応フレークシステム: `x86_64-linux`、`aarch64-linux`、`i686-linux`、`armv7l-linux`、`riscv64-linux`。

nixpkgs パッケージはまだマージされていません。上流の PR を [NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061) で追跡してください。マージされるまではフレークを使用してください。
{{< /tab >}}

{{< tab name="手動バイナリ" >}}
## 手動バイナリダウンロード

[リリースページ](https://forgejo.alexma.top/alexma233/composia/releases) からアーカイブをダウンロードします。

### 成果物

| プラットフォーム | 成果物パターン | 内容 |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`、`composia-controller`、`composia-agent`、systemd ユニットファイル |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Linux アーキテクチャ: `amd64`、`arm64`、`armv7`、`386`、`riscv64`。

macOS アーキテクチャ: `amd64`、`arm64`。

Windows アーキテクチャ: `amd64`、`arm64`、`386`。

### ダウンロード後のインストール

Linux アーカイブ:

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

macOS アーカイブ:

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Windows アーカイブ:

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

`composia.exe` を `%PATH%` のどこかに配置します。

### チェックサムの検証

同じリリースから `checksums.txt` をダウンロードし、お使いのプラットフォームの SHA-256 ツールでダウンロードした成果物を検証します。
{{< /tab >}}

{{< /tabs >}}

## システムサービス

Linux パッケージは、コントローラーとエージェント用の無効状態の systemd ユニットをインストールします。ユニットは自動的に有効化または起動されません。

パッケージされたユニットは既定の設定パスを使用します:

| サービス | 設定パス |
|---------|-------------|
| `composia-controller.service` | `/etc/composia/controller/config.yaml` |
| `composia-agent.service` | `/etc/composia/agent/config.yaml` |

設定ファイルを作成した後、明示的にサービスを有効化します:

```bash
sudo systemctl enable --now composia-controller.service
sudo systemctl enable --now composia-agent.service
```

Linux アーカイブにも `packaging/systemd/` 以下に同じユニットファイルが含まれます。アーカイブのバイナリを `/usr/local/bin` にインストールする場合は、有効化する前に `ExecStart` パスを編集してください。
