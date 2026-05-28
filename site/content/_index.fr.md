---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Libre et open source</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Vos fichiers Compose, partout
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Un système d'orchestration auto-hébergé conçu pour les utilisateurs expérimentés.&nbsp;<br class="hx:sm:block hx:hidden" />Définissez les services en texte brut, versionnez-les avec Git, sans base de données et sans dépendance propriétaire.&nbsp;<br class="hx:sm:block hx:hidden" />Sauvegardes, DNS, reverse proxy et mises à jour d'images — tout inclus.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Démarrage rapide" link="docs" >}}
{{< hextra/hero-button text="Dépôt" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="Compose multi-nœud"
  subtitle="Déployez des services sur n'importe quel nœud à partir d'une configuration simple. Un mode de connexion unique fonctionne à travers NAT, pare-feu et CDN."
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Fichiers standards, sans dépendance"
  subtitle="docker-compose.yaml + composia-meta.yaml, stockés dans votre dépôt Git. Formats ouverts, stockage sans base de données, contrôle manuel à tout moment."
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Interface web facile à utiliser"
  subtitle="Navigation et édition de fichiers, logs en direct, vues des ressources Docker et terminaux interactifs. Adapté aux mobiles, avec tout ce dont vous avez besoin pour gérer les services depuis un navigateur."
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI et API publique"
  subtitle="Une CLI complète prête pour les scripts d'automatisation et les agents IA. Les API publiques facilitent la création de clients tiers."
  icon="terminal"
>}}

{{< hextra/feature-card
  title="Sauvegarde et restauration"
  subtitle="Sauvegardes automatisées basées sur Rustic, avec exécutions planifiées, gestion des snapshots et restaurations à la demande."
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS et reverse proxy"
  subtitle="La gestion DNS Cloudflare et le reverse proxy Caddy fonctionnent immédiatement. Synchronisez et rechargez automatiquement votre Caddyfile."
  icon="globe"
>}}

{{< hextra/feature-card
  title="Détection des mises à jour d'images"
  subtitle="Détectez automatiquement les nouvelles balises d'images Docker et appliquez les mises à jour. Prend en charge plusieurs stratégies de versionnement et peut récupérer les dernières balises depuis GitHub, Forgejo et plus encore."
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="Notifications intégrées"
  subtitle="Notifications par e-mail, Telegram et Alertmanager pour les résultats de tâches, les événements de sauvegarde, les mises à jour d'images et les changements d'état des nœuds."
  icon="bell"
>}}

{{< hextra/feature-card
  title="Et plus encore…"
  icon="sparkles"
  subtitle="Système de tâches / secrets chiffrés / déploiements automatiques / métriques Prometheus / support multi-plateforme / accessibilité / et plus encore…"
>}}

{{< /hextra/feature-grid >}}
