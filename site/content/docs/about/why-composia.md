---
title: "Why Composia"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia is a self-hosted control plane for Docker Compose. Define your services as plain files, deploy them to one or many nodes, and get unified visibility across your infrastructure.

## It is not a PaaS

Unlike self-hosted PaaS platforms, Composia does not replace your Compose files with its own application model. Your configuration lives in standard `docker-compose.yaml` and `composia-meta.yaml` files that you own. The control plane coordinates and reports, but you always retain direct CLI and file-based access to every node.

Turn off Composia and your `docker compose` commands still work. Every operation is built on standard Docker and Compose primitives. There is no lock-in.

## How it compares

### Dockge, Dockman

Dockge and Dockman make managing individual Compose stacks more comfortable. They focus on single-node convenience with a browser UI.

Composia shares the file-first approach but adds multi-node coordination: deploy a service to any configured node, get a unified view of all services and nodes in one dashboard, and use a task system that records every operation with full logs. The CLI is built for scripting and automation, not just occasional use.

### Dokploy, Coolify

Dokploy and Coolify are self-hosted PaaS platforms. They define their own application model, manage build pipelines, and abstract away the underlying infrastructure. Once you adopt them, your deployment workflow depends on their abstractions.

Composia takes the opposite approach. It works with your existing Compose files in your own directory structure. There is no build pipeline, no application model to learn, and no abstraction layer between you and Docker. Composia coordinates the work that Docker does -- it does not hide Docker behind a platform abstraction.

## Design decisions

### File-based configuration

Composia uses SQLite for runtime state and Git for desired-state configuration. All configuration stays file-based, and there is no PostgreSQL, no MySQL, no external database dependency.

Back up your entire Composia installation by backing up your Git repository and the SQLite database file. Restore them to a new machine and you are back online. No database migrations, no connection pools, no separate database server.

### Standard files, no abstraction

A service is a directory containing `docker-compose.yaml` and `composia-meta.yaml`. You organize directories however you want. You can add any file a Compose project needs: env files, config templates, Caddyfile, custom scripts.

Composia reads these files from your Git repository and builds service bundles that agents execute with `docker compose`. Nothing is converted, translated, or rewritten. Your compose files are the single source of truth.

### Git-native

The controller stores the desired state in a Git repository. Every change is a commit with an author and message. You get version history, rollback capability, and the ability to sync with a remote repository. Use any Git workflow you already know.

### CLI and API first

Everything you can do in the web UI, you can do with the `composia` CLI. The CLI uses the same public API as the web frontend. Scripting, CI pipelines, and AI agents talk to Composia through the same interface.

The web UI is a SvelteKit application that calls the same controller API. There is no separate management API or internal-only endpoints.

## What you get

**Multi-node deployment.** Define which nodes a service should run on in `composia-meta.yaml`. Composia deploys the service to all target nodes and reports status from each one.

**Web dashboard.** Browse and edit repo files, view live container logs, inspect Docker resources (containers, images, networks, volumes), and open interactive terminals into running containers. The dashboard works on mobile.

**Backup and restore.** Automated backups powered by Rustic, with scheduled runs, snapshot management, and on-demand restores. Protect files, directories, named volumes, and PostgreSQL databases.

**DNS management.** Automatic DNS record creation for Cloudflare, AliDNS, DNSPod, Route53, and Huawei Cloud. Records are synced on deploy and removed on stop.

**Reverse proxy.** Caddy integration that syncs per-service Caddyfile configurations and triggers reloads automatically. Generated config files live on the agent and are imported by the Caddy infrastructure service.

**Image updates.** Automatic detection of new image versions from Docker registries and GitHub, GitLab, or Forgejo releases. Supports semver, date, regex, and latest filtering. Apply updates automatically or review before applying.

**Notifications.** Email (SMTP), Telegram, and Alertmanager notifications for task results, backup events, image updates, and node status changes. Filter by event type and task source.

**Encrypted secrets.** Age-based encryption for service secret files. Secrets are stored encrypted in the repository and decrypted only on the controller. Agents receive decrypted content in service bundles without ever accessing the private key.

**Task system.** Every operation is a tracked task with step-level progress, full log output, and completion status. Rerun tasks, inspect task steps, and follow logs in real time.

**Prometheus metrics.** The controller exposes Prometheus metrics on its HTTP server.

## Who it is for

Composia is built for power users and operations teams who:

- Already use Docker Compose and want multi-node coordination without changing their workflow.
- Prefer plain-text configuration in Git over clicking through a web form.
- Want automation (backups, DNS, updates) but refuse to hand their Compose files to a platform.
- Need a CLI they can script and integrate, not just a browser UI.
- Value file-based configuration, lock-in-free, and low-dependency infrastructure.
