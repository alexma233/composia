---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Frei und Open Source</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Deine Compose-Dateien, überall
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Ein selbst gehostetes Orchestrierungssystem für Power-User. Definiere Dienste als Klartext, verwalte sie in Git, mit vollständig dateibasierter Konfiguration und ohne Lock-in.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Schnellstart" link="docs" >}}
{{< hextra/hero-button text="Repository" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
{{< hextra/hero-button text="Warum Composia" link="docs/about/why-composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="Multi-Node Compose"
  subtitle="Deploy Dienste auf beliebige Nodes mit einer einfachen Konfiguration. Einzigartiger Verbindungsmodus funktioniert über NAT, Firewalls und CDNs hinweg."
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Standard-Dateien, kein Lock-in"
  subtitle="docker-compose.yaml + composia-meta.yaml, gespeichert in deinem Git-Repository. Offene Formate, vollständig dateibasierte Konfiguration, manuelle Kontrolle jederzeit."
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Einfach zu bedienendes Web-Dashboard"
  subtitle="Datei-Browsing und -Bearbeitung, Live-Logs, Docker-Ressourcenansichten und interaktive Terminals. Mobil-freundlich, mit allem, was du brauchst, um Dienste vom Browser aus zu verwalten."
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI und öffentliche API"
  subtitle="Eine voll ausgestattete CLI, bereit für Automatisierungsskripte und KI-Agenten. Öffentliche APIs machen Drittanbieter-Clients einfach zu erstellen."
  icon="terminal"
>}}

{{< hextra/feature-card
  title="Backup und Restore"
  subtitle="Automatisierte Backups mit Rustic, mit geplanten Durchläufen, Snapshot-Management und On-Demand-Restores."
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS und Reverse-Proxy"
  subtitle="Cloudflare-DNS-Management und Caddy-Reverse-Proxying funktionieren sofort. Synchronisiere und lade deine Caddyfile automatisch neu."
  icon="globe"
>}}

{{< hextra/feature-card
  title="Image-Update-Erkennung"
  subtitle="Erkenne automatisch neue Docker-Image-Tags und wende Updates an. Unterstützt mehrere Versionierungsstrategien und kann die neuesten Tags von GitHub, Forgejo und mehr abrufen."
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="Integrierte Benachrichtigungen"
  subtitle="E-Mail-, Telegram- und Alertmanager-Benachrichtigungen für Aufgabenergebnisse, Backup-Ereignisse, Image-Updates und Node-Statusänderungen."
  icon="bell"
>}}

{{< hextra/feature-card
  title="Und mehr…"
  icon="sparkles"
  subtitle="Aufgabensystem / verschlüsselte Secrets / automatische Deployments / Prometheus-Metriken / plattformübergreifende Unterstützung / Barrierefreiheit / und mehr…"
>}}

{{< /hextra/feature-grid >}}
