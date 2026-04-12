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
    <a href="https://docs.composia.io">
      <strong>📚 Documentation</strong>
    </a>
  </p>
</div>

Composia is a platform-agnostic Docker Compose control plane for self-hosted infrastructure.

It is built for operators who want multi-node coordination, task execution, and operational visibility without giving up direct ownership of their files, their CLI workflows, or their underlying systems.

Composia keeps desired state in plain files, stays close to standard Docker Compose workflows, and treats the control plane as an enhancement layer rather than the only way to operate services.

If you want the rationale and how Composia differs from Compose managers and self-hosted PaaS platforms, see [Why Composia?](https://docs.composia.io/guide/why-composia).

## Stack

- Backend: Go
- Frontend: SvelteKit with Bun
- Runtime: Docker Compose
- State database: SQLite
- RPC: ConnectRPC

## Quick Start

See the documentation site for installation, configuration, deployment, and operations:

- [Quick Start](https://docs.composia.io/guide/quick-start)
- [Configuration Guide](https://docs.composia.io/guide/configuration)
- [Development Guide](https://docs.composia.io/guide/development)
- [Why Composia?](https://docs.composia.io/guide/why-composia)

## Development

This repository includes `mise.toml` for local tool versions.

For full setup and workflow details, see the [Development Guide](https://docs.composia.io/guide/development).

Common local commands:

```bash
mise install
mise run dev
mise run dev:down
mise run dev:logs
buf generate
```

## Repository Layout

```text
cmd/composia/         # composia entrypoint
dev/                  # local development state and local-only config
gen/go/               # generated protobuf and Connect code
internal/             # backend packages
proto/                # protobuf definitions
web/                  # SvelteKit frontend
plan.md               # product and architecture notes
```

## Attributions

- [Dockman](https://github.com/RA341/dockman) - Docker management UI reference for Docker resource list/inspect page patterns (AGPL-3.0)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE) for details.

If you require a commercial license for use cases not permitted under AGPL-3.0, please contact the author.
