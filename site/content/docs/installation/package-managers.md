---
title: "Package Managers and Binaries"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Use this page for non-container installs and manual binary downloads.

## Runtime tools

Container images already include these tools. Non-container installs must provide them on the host.

| Host role | Required tools |
|-----------|----------------|
| Controller | CA certificates, `git`. |
| Agent | `git`, Docker CLI, Docker Buildx plugin, Docker Compose plugin, access to the Docker daemon. |

Linux packages and archives install Composia binaries and optional systemd unit files. They do not install Docker, Docker Compose, or Git for you.

{{< tabs >}}

{{< tab name="APT" >}}
## Debian and Ubuntu

Add the Forgejo APT repository:

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

You can also download the `.deb` package from the [Releases page](https://forgejo.alexma.top/alexma233/composia/releases) and install it with your package manager.

Verify:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora, RHEL, and compatible distributions

Use COPR on Fedora:

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

You can also download the `.rpm` package from the [Releases page](https://forgejo.alexma.top/alexma233/composia/releases) and install it with `dnf` or `yum`.

Verify:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

Two AUR packages are published:

| Package | Source |
|---------|--------|
| `composia-bin` | Prebuilt release binaries. |
| `composia` | Built from source. |

Install one of them with your AUR helper:

```bash
paru -S composia-bin
```

or:

```bash
paru -S composia
```

The two packages conflict with each other. Install only one.

You can also download the `.pkg.tar.zst` package from the [Releases page](https://forgejo.alexma.top/alexma233/composia/releases) and install it with `pacman -U`.

Verify:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

Install from the repository flake:

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

Run a binary directly:

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

Supported flake systems: `x86_64-linux`, `aarch64-linux`, `i686-linux`, `armv7l-linux`, `riscv64-linux`.

The nixpkgs package is not merged yet. Track the upstream PR at [NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061). Until it lands, use the flake.
{{< /tab >}}

{{< tab name="Manual Binary" >}}
## Manual binary download

Download archives from the [Releases page](https://forgejo.alexma.top/alexma233/composia/releases).

### Artifacts

| Platform | Artifact pattern | Contents |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`, `composia-controller`, `composia-agent`, systemd unit files |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Linux architectures: `amd64`, `arm64`, `armv7`, `386`, `riscv64`.

macOS architectures: `amd64`, `arm64`.

Windows architectures: `amd64`, `arm64`, `386`.

### Install after download

Linux archive:

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

macOS archive:

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Windows archive:

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

Put `composia.exe` somewhere in `%PATH%`.

### Verify checksums

Download `checksums.txt` from the same release and verify the downloaded artifact with your platform's SHA-256 tool.
{{< /tab >}}

{{< /tabs >}}

## System services

Linux packages install inactive systemd units for the controller and agent. The units are not enabled or started automatically.

The packaged units use the default config paths:

| Service | Config path |
|---------|-------------|
| `composia-controller.service` | `/etc/composia/controller/config.yaml` |
| `composia-agent.service` | `/etc/composia/agent/config.yaml` |

After creating the config files, enable the services explicitly:

```bash
sudo systemctl enable --now composia-controller.service
sudo systemctl enable --now composia-agent.service
```

Linux archives include the same unit files under `packaging/systemd/`. If you install archive binaries under `/usr/local/bin`, edit the unit `ExecStart` paths before enabling them.
