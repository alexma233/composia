---
title: "Gestionnaires de paquets et binaires"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Utilisez cette page pour les installations sans conteneur et les téléchargements manuels de binaires.

## Outils d'exécution

Les images de conteneur incluent déjà ces outils. Les installations sans conteneur doivent les fournir sur l'hôte.

| Rôle hôte | Outils requis |
|-----------|----------------|
| Contrôleur | Certificats CA, `git`. |
| Agent | `git`, Docker CLI, plugin Docker Buildx, plugin Docker Compose, accès au démon Docker. |

Les paquets et archives Linux installent les binaires Composia et des unités systemd facultatives. Ils n'installent pas Docker, Docker Compose ou Git pour vous.

{{< tabs >}}

{{< tab name="APT" >}}
## Debian et Ubuntu

Ajoutez le dépôt APT Forgejo :

```bash
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl https://forgejo.alexma.top/api/packages/alexma233/debian/repository.key \
  -o /etc/apt/keyrings/composia.asc

echo "deb [signed-by=/etc/apt/keyrings/composia.asc] https://forgejo.alexma.top/api/packages/alexma233/debian stable main" \
  | sudo tee /etc/apt/sources.list.d/composia.list

sudo apt update
sudo apt install composia
```

Vous pouvez également télécharger le paquet `.deb` depuis la [page des Releases](https://forgejo.alexma.top/alexma233/composia/releases) et l'installer avec votre gestionnaire de paquets.

Vérifiez :

```bash
composia --version
```
{{< /tab >}}

{{< tab name="RPM / COPR" >}}
## Fedora, RHEL et distributions compatibles

Utilisez COPR sur Fedora :

```bash
sudo dnf copr enable alexma233/composia
sudo dnf install composia
```

Vous pouvez également télécharger le paquet `.rpm` depuis la [page des Releases](https://forgejo.alexma.top/alexma233/composia/releases) et l'installer avec `dnf` ou `yum`.

Vérifiez :

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Arch / AUR" >}}
## Arch Linux

Deux paquets AUR sont publiés :

| Paquet | Source |
|---------|--------|
| `composia-bin` | Binaires de release précompilés. |
| `composia` | Compilé depuis les sources. |

Installez l'un d'eux avec votre helper AUR :

```bash
paru -S composia-bin
```

ou :

```bash
paru -S composia
```

Les deux paquets sont en conflit l'un avec l'autre. N'en installez qu'un seul.

Vous pouvez également télécharger le paquet `.pkg.tar.zst` depuis la [page des Releases](https://forgejo.alexma.top/alexma233/composia/releases) et l'installer avec `pacman -U`.

Vérifiez :

```bash
composia --version
```
{{< /tab >}}

{{< tab name="Nix" >}}
## Nix Flake

Installez depuis le flake du dépôt :

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

Exécutez un binaire directement :

```bash
nix run git+https://forgejo.alexma.top/alexma233/composia#composia
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-controller
nix run git+https://forgejo.alexma.top/alexma233/composia#composia-agent
```

Systèmes flake pris en charge : `x86_64-linux`, `aarch64-linux`, `i686-linux`, `armv7l-linux`, `riscv64-linux`.

Le paquet nixpkgs n'est pas encore fusionné. Suivez la PR upstream sur [NixOS/nixpkgs#515061](https://github.com/NixOS/nixpkgs/pull/515061). En attendant, utilisez le flake.
{{< /tab >}}

{{< tab name="Binaire manuel" >}}
## Téléchargement manuel de binaires

Téléchargez les archives depuis la [page des Releases](https://forgejo.alexma.top/alexma233/composia/releases).

### Artefacts

| Plateforme | Modèle d'artefact | Contenu |
|----------|------------------|----------|
| Linux | `composia_<version>_linux_<arch>.tar.gz` | `composia`, `composia-controller`, `composia-agent`, unités systemd |
| macOS | `composia_<version>_darwin_<arch>.tar.gz` | `composia` |
| Windows | `composia_<version>_windows_<arch>.zip` | `composia.exe` |

Architectures Linux : `amd64`, `arm64`, `armv7`, `386`, `riscv64`.

Architectures macOS : `amd64`, `arm64`.

Architectures Windows : `amd64`, `arm64`, `386`.

### Installation après téléchargement

Archive Linux :

```bash
tar xzf composia_<version>_linux_<arch>.tar.gz
sudo install -m 755 composia composia-controller composia-agent /usr/local/bin/
```

Archive macOS :

```bash
tar xzf composia_<version>_darwin_<arch>.tar.gz
sudo install -m 755 composia /usr/local/bin/
```

Archive Windows :

```powershell
Expand-Archive composia_<version>_windows_<arch>.zip -DestinationPath .
```

Placez `composia.exe` quelque part dans `%PATH%`.

### Vérifier les sommes de contrôle

Téléchargez `checksums.txt` depuis la même release et vérifiez l'artefact téléchargé avec l'outil SHA-256 de votre plateforme.
{{< /tab >}}

{{< /tabs >}}

## Services système

Les paquets Linux installent des unités systemd inactives pour le contrôleur et l'agent. Les unités ne sont pas activées ni démarrées automatiquement.

Les unités empaquetées utilisent les chemins de configuration par défaut :

| Service | Chemin de configuration |
|---------|-------------|
| `composia-controller.service` | `/etc/composia/controller/config.yaml` |
| `composia-agent.service` | `/etc/composia/agent/config.yaml` |

Après avoir créé les fichiers de configuration, activez les services explicitement :

```bash
sudo systemctl enable --now composia-controller.service
sudo systemctl enable --now composia-agent.service
```

Les archives Linux incluent les mêmes unités sous `packaging/systemd/`. Si vous installez les binaires d'archive sous `/usr/local/bin`, modifiez les chemins `ExecStart` avant de les activer.
