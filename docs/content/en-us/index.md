---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose Control Plane"
  tagline: A self-hosted service management platform that uses service definitions and execution agents to operate Docker Compose across multiple nodes
  image:
    src: /logo.svg
    alt: Composia
  actions:
    - theme: brand
      text: Quick Start
      link: /guide/quick-start
    - theme: alt
      text: View on Forgejo
      link: https://forgejo.alexma.top/alexma233/composia

features:
  - title: 🐳 Native Docker Compose
    details: Built around Docker Compose, with a small `composia-meta.yaml` file for Composia-specific metadata.
  - title: 🎛️ Single Control Plane
    details: Centralized control plane manages all services and nodes, providing a unified view and control interface.
  - title: 🤖 Multi-Agent Architecture
    details: Support for one or more execution agents that can scale horizontally to manage large infrastructure.
  - title: 📋 Runtime Visibility
    details: Unified visibility into service status, task logs, node summaries, disk capacity, and Docker inventory counts.
  - title: 🔒 Secure & Reliable
    details: Released under AGPL-3.0 open source license with transparent, auditable code and private deployment support.
  - title: ⚡ Modern Tech Stack
    details: Built with Go, SvelteKit, ConnectRPC, SQLite, and Docker Compose.
---
