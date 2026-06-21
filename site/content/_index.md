---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Free and open source</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Your Compose files, everywhere
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  A self-hosted orchestration system crafted for power users. Define services in plain text, keep them in Git, with all configuration file-based and lock-in-free.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Quick Start" link="docs" >}}
{{< hextra/hero-button text="Repository" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
{{< hextra/hero-button text="Why Composia" link="docs/about/why-composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="Multi-node Compose"
  subtitle="Deploy services to any node from a simple configuration. Unique connection mode work across NAT, firewalls, and CDNs."
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Standard files, no lock-in"
  subtitle="docker-compose.yaml + composia-meta.yaml, stored in your Git repository. Open formats, with all configuration file-based, manual control anytime."
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Easy-to-use web dashboard"
  subtitle="File browsing and editing, live logs, Docker resource views, and interactive terminals. Mobile-friendly, with everything you need to manage services from a browser."
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI and public API"
  subtitle="A full-featured CLI ready for automation scripts and AI agents. Public APIs make third-party clients easy to build."
  icon="terminal"
>}}

{{< hextra/feature-card
  title="Backup and restore"
  subtitle="Automated backups powered by Rustic, with scheduled runs, snapshot management, and on-demand restores."
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS and reverse proxy"
  subtitle="Cloudflare DNS management and Caddy reverse proxying work out of the box. Automatically sync and reload your Caddyfile."
  icon="globe"
>}}

{{< hextra/feature-card
  title="Image update detection"
  subtitle="Automatically detect new Docker image tags and apply updates. Supports multiple versioning strategies and can fetch the latest tags from GitHub, Forgejo, and more."
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="Built-in notifications"
  subtitle="Email, Telegram, and Alertmanager notifications for task results, backup events, image updates, and node status changes."
  icon="bell"
>}}

{{< hextra/feature-card
  title="And more…"
  icon="sparkles"
  subtitle="Task system / encrypted secrets / automatic deployments / Prometheus metrics / cross-platform support / accessibility / and more…"
>}}

{{< /hextra/feature-grid >}}
