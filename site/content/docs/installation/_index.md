---
title: "Installation"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia has four runtime binaries and images:

| Component | Purpose |
|-----------|---------|
| `composia-controller` | Runs the API, task queue, desired-state Git repository, and controller-side integrations. |
| `composia-agent` | Runs on each Docker node and executes Docker Compose operations. |
| `composia-web` | Browser UI that talks to the controller. |
| `composia` | CLI for terminals, scripts, and automation. |

## Choose a method

| Method | Best for |
|--------|----------|
| [Docker Compose](docker-compose/) | Fast all-in-one deployment with controller, local agent, and web UI. |
| [Package Managers and Binaries](package-managers/) | Non-container installs, OS packages, Nix, AUR, and manual archives. |
| [Configuration](configuration/) | Config files, web environment variables, age key setup, and full global config reference. |

For source builds, see [Developer Guide: Source Build](/docs/developer-guide/source-build/).
