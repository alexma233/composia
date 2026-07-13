<h1 align="center">Composia</h1>

<p align="center">
  <img src="./branding/icon/svg/64px.svg" alt="Composia logo" width="96" />
</p>

<p align="center">
  <a href="https://forgejo.alexma.top/alexma233/composia">
    <img src="https://img.shields.io/badge/AlexMa's%20Forgejo-View%20Repo-blue?style=for-the-badge" alt="AlexMa's Forgejo" />
  </a>
</p>

<p align="center">
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

<p align="center">
  <a href="https://composia.xyz"><strong>Documentation</strong></a>
</p>

**Your Compose files, everywhere.**

A self-hosted orchestration system crafted for power users. Define services in plain text, keep them in Git, with all configuration file-based and lock-in-free. Backups, DNS, reverse proxying, and image updates — all included.

Unlike PaaS platforms, Composia stays out of your way. Your configuration lives in standard `docker-compose.yaml` and `composia-meta.yaml` files that you own. The control plane coordinates and reports, but you always retain direct CLI and file-based access to every node.

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

## Features

- **Multi-node Compose** — deploy services to any node from a simple configuration; works across NAT, firewalls, and CDNs
- **Standard files, no lock-in** — `docker-compose.yaml` + `composia-meta.yaml` in your own Git repository; open formats, manual control anytime
- **Web dashboard** — file browsing and editing, live logs, Docker resource views, and interactive terminals; mobile-friendly
- **CLI and public API** — full-featured CLI ready for automation and AI agents; public APIs for third-party clients
- **Backup and restore** — automated backups powered by Rustic, with scheduled runs, snapshot management, and on-demand restores
- **DNS and reverse proxy** — Cloudflare DNS management and Caddy reverse proxying out of the box; auto-sync and reload your Caddyfile
- **Image update detection** — automatically detect new Docker image tags and apply updates; supports multiple versioning strategies
- **Built-in notifications** — Email, Telegram, and Alertmanager notifications for task results, backup events, image updates, and node status changes
- **And more…** — task system, encrypted secrets, automatic deployments, Prometheus metrics, cross-platform support, accessibility

## Stack

| Component  | Technology                  |
| ---------- | --------------------------- |
| Backend    | Go                          |
| Frontend   | SvelteKit (Deno)            |
| Runtime    | Docker Compose              |
| State      | SQLite                      |
| RPC        | ConnectRPC                  |
| Web UI     | shadcn-svelte               |

## Quick Start

- [Installation](https://composia.xyz/docs/installation/docker-compose/)
- [Configuration Guide](https://composia.xyz/docs/installation/configuration/)
- [Development Guide](https://composia.xyz/docs/developer-guide/source-build/)
- [Why Composia?](https://composia.xyz/docs/about/why-composia/)

## Development

```bash
mise install
mise run setup
mise run dev
```

Common local tasks:

```bash
mise run dev:docs   # docs only
mise run dev:all    # app + docs
mise run check      # local pre-commit checks
mise run check:full # slower race/lint/vulnerability checks
mise run gen        # protobuf code + API docs
mise run e2e        # CLI, controller, and web e2e tests
```

## Repository Layout

```text
cmd/
  composia/            user-facing CLI
  composia-agent/      agent runtime
  composia-controller/  controller runtime
dev/                    local development config and state
gen/go/                 generated protobuf and Connect code
internal/
  app/                  application entrypoints
    agent/              agent server
    cli/                CLI commands
    controller/         controller server
    notify/             notification dispatch
  core/                 domain logic
    backup/             backup and restore operations
    config/             configuration loading
    notify/             notification event system
    repo/               git repo and service file management
    schedule/           cron-based task scheduling
    task/               task execution engine
  platform/             infrastructure
    configpath/         config file resolution
    rpcutil/            RPC helpers
    secret/             age encryption and decryption
    store/              SQLite persistence layer
  version/              build version
proto/                  protobuf definitions
web/                    SvelteKit frontend
  src/routes/
    backups/            backup management
    login/              authentication
    logout/             session termination
    nodes/              node management
    services/           service management
    settings/           system settings
    tasks/              task and schedule management
```

## Attributions

- [Dockman](https://github.com/RA341/dockman) — Docker management UI reference for resource list and inspect page patterns (AGPL-3.0)
- [Twemoji](https://github.com/twitter/twemoji) — the Composia logo is adapted from Twemoji graphics, licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)

## License

Source code is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE).

Documentation and website content (including Markdown files and `site/content/`) are licensed under [Creative Commons Attribution 4.0 International](https://creativecommons.org/licenses/by/4.0/). See [LICENSES/CC-BY-4.0.txt](LICENSES/CC-BY-4.0.txt).

When reusing documentation or website content, provide attribution to the Composia project, link to the original repository when reasonably practicable, link to the CC BY 4.0 license, and indicate if changes were made.

The Composia logo and derived site icons are licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) with attribution to Twemoji as described above.

This documentation license does not apply to source code, configuration files, generated files, third-party materials, trademarks, service marks, or project logos except where explicitly stated.

For commercial licensing outside AGPL-3.0 scope, contact the author.
