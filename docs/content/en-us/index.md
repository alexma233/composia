---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose Control Plane"
  tagline: A platform-agnostic Docker Compose control plane for multi-node self-hosted infrastructure that keeps file-based and CLI workflows intact
  image:
    src: /logo.svg
    alt: Composia
  actions:
    - theme: brand
      text: Quick Start
      link: /guide/quick-start
    - theme: alt
      text: Why Composia?
      link: /guide/why-composia
    - theme: alt
      text: View on Forgejo
      link: https://forgejo.alexma.top/alexma233/composia

features:
  - title: 🐳 Native Docker Compose
    details: Keeps Docker Compose and plain files at the center instead of moving your configuration into a private platform model.
  - title: 🎛️ Single Control Plane
    details: Coordinates services, nodes, tasks, and state through one control plane while preserving direct access to the underlying system.
  - title: 🤖 Multi-Agent Architecture
    details: Support for one or more execution agents that can scale horizontally to manage large infrastructure.
  - title: 📋 Runtime Visibility
    details: Unified visibility into service status, task logs, node summaries, disk capacity, and Docker inventory counts.
  - title: 🧾 File-First
    details: Desired state stays in repositories and normal files so it can be reviewed, migrated, and operated with standard CLI tools.
  - title: 🔒 Secure & Reliable
    details: Released under AGPL-3.0 open source license with transparent, auditable code and private deployment support.
  - title: 🚫 No Platform Lock-In
    details: The control plane coordinates and reports, but it does not take ownership of your infrastructure away from the operator.
---
