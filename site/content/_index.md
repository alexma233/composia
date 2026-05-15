---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Free & Open Source</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Composia — Self-Hosted Docker Compose Control Plane
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Define services as plain files, deploy them to one or many nodes, and get unified visibility across your infrastructure — without lock-in.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get Started" link="docs" >}}
{{< hextra/hero-button text="View on Forgejo" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="Multi-Node Orchestration"
  subtitle="Deploy services to one or many nodes from a single Git repo. Agent-based pull model works through NAT and firewalls."
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Standard Files, Zero Lock-In"
  subtitle="docker-compose.yaml + composia-meta.yaml in your own Git repo. No proprietary formats. SSH into any node, anytime."
  icon="document"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Web Dashboard"
  subtitle="Real-time service status, container log streaming, interactive terminal exec, built-in file tree and YAML editor."
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI & Public API"
  subtitle="Feature-complete terminal client for automation and scripting. Public ConnectRPC API — AI agent ready, third-party friendly."
  icon="terminal"
>}}

{{< hextra/feature-card
  title="Backup & Restore"
  subtitle="Automated Rustic backups with scheduling, snapshot management, and on-demand restore."
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS & Reverse Proxy"
  subtitle="Cloudflare DNS management and Caddy reverse proxy out of the box. Automatic Caddyfile sync and reload."
  icon="globe"
>}}

{{< /hextra/feature-grid >}}
