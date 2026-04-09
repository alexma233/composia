---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose with Magic"
  tagline: A self-hosted service manager built around service definitions, a single control plane, and one or more execution agents
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
  - title: 📊 Built-in Monitoring
    details: Real-time monitoring of service status, resource usage, and logs to quickly discover and resolve issues.
  - title: 🔒 Secure & Reliable
    details: Released under AGPL-3.0 open source license with transparent, auditable code and private deployment support.
  - title: ⚡ Modern Tech Stack
    details: Go backend + SvelteKit frontend for high performance, low resource usage, and rapid response.
---
