# Composia

<div align="center">
  <p><strong>Main Repository</strong></p>
  <p>
    <a href="https://forgejo.alexma.top/alexma233/composia">
      <img src="https://img.shields.io/gitea/stars/alexma233/composia?gitea_url=https://forgejo.alexma.top&style=for-the-badge&label=AlexMa's%20Forgejo%20Stars" alt="Forgejo Stars" />
    </a>
  </p>

  <p>Mirrors</p>
  <p>
    <a href="https://codeberg.org/alexma233/composia">
      <img src="https://img.shields.io/gitea/stars/alexma233/composia?gitea_url=https://codeberg.org&style=flat-square&label=Codeberg%20Stars" alt="Codeberg Stars" />
    </a>
    <a href="https://github.com/alexma233/composia">
      <img src="https://img.shields.io/github/stars/alexma233/composia?style=flat-square&label=GitHub%20Stars" alt="GitHub Stars" />
    </a>
    <a href="https://tangled.org/fur.im/composia">
      <img src="https://img.shields.io/badge/Tangled-View%20Repo-blue?style=flat-square" alt="Tangled" />
    </a>
  </p>

  <p>
    <a href="https://docs.composia.xyz">
      <strong>📚 Documentation</strong>
    </a>
  </p>
</div>

Composia is a self-hosted control plane for Docker Compose — define your services as plain files, deploy them to one or many nodes, and get unified visibility across your infrastructure.

**Unlike PaaS platforms, Composia stays out of your way.** Your configuration lives in standard `docker-compose.yaml` and `composia-meta.yaml` files that you own. The control plane coordinates and reports, but you always retain direct CLI and file-based access to every node.

A service definition looks like this:

```yaml
# composia-meta.yaml — declare what and where
name: my-app
nodes:
  - main
  - edge

# docker-compose.yaml — standard Compose, no lock-in
services:
  app:
    image: myapp:1.2.3
    ports:
      - "8080:80"
    volumes:
      - ./data:/app/data
```

See [Why Composia?](https://docs.composia.xyz/guide/why-composia) for how it compares to other tools.

## Stack

- Backend: Go
- Frontend: SvelteKit with Bun
- Runtime: Docker Compose
- State database: SQLite
- RPC: ConnectRPC

## Quick Start

See the documentation site for installation, configuration, deployment, and operations:

- [Quick Start](https://docs.composia.xyz/guide/quick-start)
- [Configuration Guide](https://docs.composia.xyz/guide/configuration)
- [Development Guide](https://docs.composia.xyz/guide/development)
- [Why Composia?](https://docs.composia.xyz/guide/why-composia)

## Development

This repository includes `mise.toml` for local tool versions.

For full setup and workflow details, see the [Development Guide](https://docs.composia.xyz/guide/development).

Common local commands:

```bash
mise install
mise run dev
mise run dev:down
mise run dev:logs
buf generate
```

## Binary Builds

Composia can run without Docker. Linux release packages include:

- `composia` — user-facing CLI
- `composia-controller` — controller runtime
- `composia-agent` — agent runtime

Darwin and Windows releases include only the `composia` CLI.

Build local binaries for the current platform:

```bash
sh ./scripts/build/binaries.sh
```

Cross-build by setting Go target variables:

```bash
VERSION=v0.1.5 GOOS=linux GOARCH=amd64 sh ./scripts/build/binaries.sh
VERSION=v0.1.5 GOOS=linux GOARCH=arm GOARM=7 sh ./scripts/build/binaries.sh
```

Release packaging is handled by GoReleaser:

```bash
goreleaser release --snapshot --clean
```

The release configuration builds pure binary archives for Linux, Darwin, and Windows. Linux releases include `.deb`, `.rpm`, Arch Linux binary packages, and the `composia-bin` AUR package. Nix users can install the Linux package from the flake:

```bash
nix profile install git+https://forgejo.alexma.top/alexma233/composia
```

An AUR source-build `PKGBUILD` template is available under `packaging/aur/`. The binary AUR package is published by GoReleaser.

Container images are split by runtime role:

```text
forgejo.alexma.top/alexma233/composia-cli
forgejo.alexma.top/alexma233/composia-controller
forgejo.alexma.top/alexma233/composia-agent
forgejo.alexma.top/alexma233/composia-web
```

## Repository Layout

```text
cmd/composia/         # user-facing CLI entrypoint
cmd/composia-agent/   # agent runtime entrypoint
cmd/composia-controller/ # controller runtime entrypoint
dev/                  # local development state and local-only config
gen/go/               # generated protobuf and Connect code
internal/             # backend packages
proto/                # protobuf definitions
web/                  # SvelteKit frontend
```

## Attributions

- [Dockman](https://github.com/RA341/dockman) - Docker management UI reference for Docker resource list/inspect page patterns (AGPL-3.0)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE) for details.

If you require a commercial license for use cases not permitted under AGPL-3.0, please contact the author.
