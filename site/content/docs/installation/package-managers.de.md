---
title: "Paketmanager und Binärdateien"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Verwende diese Seite für Nicht-Container-Installationen und manuelle Binär-Downloads.

## Laufzeitwerkzeuge

Container-Images enthalten diese Werkzeuge bereits. Nicht-Container-Installationen müssen sie auf dem Host bereitstellen.

| Host-Rolle | Erforderliche Werkzeuge |
|-----------|----------------|
| Controller | CA-Zertifikate, `git`. |
| Agent | `git`, Docker CLI, Docker Buildx-Plugin, Docker Compose-Plugin, Zugriff auf den Docker-Daemon. |

Linux-Pakete und -Archive installieren Composia-Binärdateien und optionale systemd-Unit-Dateien. Sie installieren Docker, Docker Compose oder Git nicht für dich.

{{< tabs >}}

{{< tab name="APT" >}}
## Debian und Ubuntu

Füge das Forgejo-APT-Repository hinzu:

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

Du kannst auch das `.deb`-Paket von der [Releases-Seite](https://forgejo.alexma.top/alexma233/composia/releases) herunterladen und mit deinem Paketmanager installieren.

Überprüfen:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora, RHEL und kompatible Distributionen

Verwende COPR auf Fedora:

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

Du kannst auch das `.rpm`-Paket von der [Releases-Seite](https://forgejo.alexma.top/alexma233/composia/releases) herunterladen und mit `dnf` oder `yum` installieren.

Überprüfen:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

Zwei AUR-Pakete werden veröffentlicht:

| Paket | Quelle |
|---------|--------|
| `composia-bin` | Vorgefertigte Release-Binärdateien. |
| `composia` | Aus dem Quellcode gebaut. |

Installiere eines davon mit deinem AUR-Helper:

```bash
paru -S composia-bin
```

oder:

```bash
paru -S composia
```

Die beiden Pakete stehen miteinander in Konflikt. Installiere nur eines.

Du kannst auch das `.pkg.tar.zst`-Paket von der [Releases-Seite](https://forgejo.alexma.top/alexma233/composia/releases) herunterladen und mit `pacman -U` installieren.

Überprüfen:

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

Installiere aus dem Repository-Flake:

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

Führe eine Binärdatei direkt aus:

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

Unterstützte Flake-Systeme: `x86_64-linux`, `aarch64-linux`, `i686-linux`, `armv7l-linux`, `riscv64-linux`.

Das nixpkgs-Paket ist noch nicht zusammengeführt. Verfolge den Upstream-PR unter [NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061). Bis es verfügbar ist, verwende das Flake.
{{< /tab >}}

{{< tab name="Manuelle Binärdatei" >}}
## Manueller Binär-Download

Lade Archive von der [Releases-Seite](https://forgejo.alexma.top/alexma233/composia/releases) herunter.

### Artefakte

| Plattform | Artefaktmuster | Inhalt |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`, `composia-controller`, `composia-agent`, systemd-Unit-Dateien |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Linux-Architekturen: `amd64`, `arm64`, `armv7`, `386`, `riscv64`.

macOS-Architekturen: `amd64`, `arm64`.

Windows-Architekturen: `amd64`, `arm64`, `386`.

### Nach dem Download installieren

Linux-Archiv:

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

macOS-Archiv:

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Windows-Archiv:

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

Platziere `composia.exe` irgendwo in `%PATH%`.

### Prüfsummen überprüfen

Lade `checksums.txt` vom selben Release herunter und überprüfe das heruntergeladene Artefakt mit dem SHA-256-Werkzeug deiner Plattform.
{{< /tab >}}

{{< /tabs >}}

## Systemdienste

Linux-Pakete installieren inaktive systemd-Units für Controller und Agent. Die Units werden nicht automatisch aktiviert oder gestartet.

Die paketierten Units verwenden die Standard-Konfigurationspfade:

| Dienst | Konfigurationspfad |
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

Nachdem du die Konfigurationsdateien erstellt hast, aktiviere die Dienste explizit:

```bash
sudo systemctl enable --now composia-controller.service
sudo systemctl enable --now composia-agent.service
```

Linux-Archive enthalten dieselben Unit-Dateien unter `packaging/systemd/`. Wenn du Archiv-Binärdateien unter `/usr/local/bin` installierst, passe die `ExecStart`-Pfade vor dem Aktivieren an.
