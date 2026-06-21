---
title: "Pourquoi Composia"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia est un plan de contrôle auto-hébergé pour Docker Compose. Définissez vos services sous forme de fichiers texte, déployez-les sur un ou plusieurs nœuds et obtenez une visibilité unifiée sur votre infrastructure.

## Ce n'est pas un PaaS

Contrairement aux plateformes PaaS auto-hébergées, Composia ne remplace pas vos fichiers Compose par son propre modèle applicatif. Votre configuration réside dans des fichiers standards `docker-compose.yaml` et `composia-meta.yaml` que vous possédez. Le plan de contrôle coordonne et rapporte, mais vous conservez toujours un accès direct en CLI et par fichiers à chaque nœud.

Éteignez Composia et vos commandes `docker compose` fonctionnent toujours. Chaque opération est construite sur des primitives Docker et Compose standard. Il n'y a aucune dépendance propriétaire.

## Comparaison

### Dockge, Dockman

Dockge et Dockman rendent la gestion de piles Compose individuelles plus confortable. Ils se concentrent sur la commodité mono-nœud avec une interface navigateur.

Composia partage l'approche orientée fichiers mais ajoute la coordination multi-nœud : déployez un service sur n'importe quel nœud configuré, obtenez une vue unifiée de tous les services et nœuds dans un seul tableau de bord, et utilisez un système de tâches qui enregistre chaque opération avec des logs complets. La CLI est conçue pour le scripting et l'automatisation, pas seulement pour une utilisation occasionnelle.

### Dokploy, Coolify

Dokploy et Coolify sont des plateformes PaaS auto-hébergées. Elles définissent leur propre modèle applicatif, gèrent des pipelines de build et abstraient l'infrastructure sous-jacente. Une fois que vous les adoptez, votre workflow de déploiement dépend de leurs abstractions.

Composia adopte l'approche inverse. Il fonctionne avec vos fichiers Compose existants dans votre propre structure de répertoires. Il n'y a pas de pipeline de build, pas de modèle applicatif à apprendre et pas de couche d'abstraction entre vous et Docker. Composia coordonne le travail que Docker effectue — il ne cache pas Docker derrière une abstraction de plateforme.

## Choix de conception

### Configuration basée sur des fichiers

Composia utilise SQLite pour l'état d'exécution et Git pour la configuration de l'état désiré. Toute la configuration reste basée sur des fichiers, sans PostgreSQL, sans MySQL et sans dépendance à une base de données externe.

Sauvegardez l'intégralité de votre installation Composia en sauvegardant votre dépôt Git et le fichier de base de données SQLite. Restaurez-les sur une nouvelle machine et vous êtes de nouveau en ligne. Pas de migrations de base de données, pas de pools de connexion, pas de serveur de base de données séparé.

### Fichiers standards, sans abstraction

Un service est un répertoire contenant `docker-compose.yaml` et `composia-meta.yaml`. Vous organisez les répertoires comme vous le souhaitez. Vous pouvez ajouter tout fichier dont un projet Compose a besoin : fichiers d'environnement, modèles de configuration, Caddyfile, scripts personnalisés.

Composia lit ces fichiers depuis votre dépôt Git et construit des bundles de service que les agents exécutent avec `docker compose`. Rien n'est converti, traduit ou réécrit. Vos fichiers Compose sont la source unique de vérité.

### Natif Git

Le contrôleur stocke l'état désiré dans un dépôt Git. Chaque modification est un commit avec un auteur et un message. Vous obtenez l'historique des versions, la capacité de rollback et la possibilité de synchroniser avec un dépôt distant. Utilisez n'importe quel workflow Git que vous connaissez déjà.

### CLI et API d'abord

Tout ce que vous pouvez faire dans l'interface web, vous pouvez le faire avec la CLI `composia`. La CLI utilise la même API publique que le frontend web. Le scripting, les pipelines CI et les agents IA communiquent avec Composia via la même interface.

L'interface web est une application SvelteKit qui appelle la même API du contrôleur. Il n'y a pas d'API de gestion séparée ni de points de terminaison internes.

## Ce que vous obtenez

**Déploiement multi-nœud.** Définissez sur quels nœuds un service doit s'exécuter dans `composia-meta.yaml`. Composia déploie le service sur tous les nœuds cibles et rapporte l'état de chacun.

**Tableau de bord web.** Parcourez et modifiez les fichiers du dépôt, visualisez les logs des conteneurs en direct, inspectez les ressources Docker (conteneurs, images, réseaux, volumes) et ouvrez des terminaux interactifs dans les conteneurs en cours d'exécution. Le tableau de bord fonctionne sur mobile.

**Sauvegarde et restauration.** Sauvegardes automatisées basées sur Rustic, avec exécutions planifiées, gestion des snapshots et restaurations à la demande. Protégez les fichiers, répertoires, volumes nommés et bases de données PostgreSQL.

**Gestion DNS.** Création automatique d'enregistrements DNS pour Cloudflare, AliDNS, DNSPod, Route53 et Huawei Cloud. Les enregistrements sont synchronisés au déploiement et supprimés à l'arrêt.

**Reverse proxy.** Intégration Caddy qui synchronise les configurations Caddyfile par service et déclenche les rechargements automatiquement. Les fichiers de configuration générés résident sur l'agent et sont importés par le service d'infrastructure Caddy.

**Mises à jour d'images.** Détection automatique des nouvelles versions d'images depuis les registres Docker et les releases GitHub, GitLab ou Forgejo. Prend en charge le filtrage par semver, date, regex et latest. Appliquez les mises à jour automatiquement ou révisez-les avant application.

**Notifications.** Notifications par e-mail (SMTP), Telegram et Alertmanager pour les résultats de tâches, les événements de sauvegarde, les mises à jour d'images et les changements d'état des nœuds. Filtrez par type d'événement et source de tâche.

**Secrets chiffrés.** Chiffrement basé sur age pour les fichiers de secrets de service. Les secrets sont stockés chiffrés dans le dépôt et déchiffrés uniquement sur le contrôleur. Les agents reçoivent le contenu déchiffré dans les bundles de service sans jamais accéder à la clé privée.

**Système de tâches.** Chaque opération est une tâche tracée avec progression par étape, sortie complète des logs et état d'achèvement. Relancez les tâches, inspectez les étapes des tâches et suivez les logs en temps réel.

**Métriques Prometheus.** Le contrôleur expose des métriques Prometheus sur son serveur HTTP.

## À qui il s'adresse

Composia est conçu pour les utilisateurs expérimentés et les équipes d'exploitation qui :

- Utilisent déjà Docker Compose et souhaitent une coordination multi-nœud sans changer leur workflow.
- Préfèrent la configuration en texte brut dans Git plutôt que de cliquer dans un formulaire web.
- Veulent de l'automatisation (sauvegardes, DNS, mises à jour) mais refusent de confier leurs fichiers Compose à une plateforme.
- Ont besoin d'une CLI qu'ils peuvent scripter et intégrer, pas seulement une interface navigateur.
- Apprécient une configuration basée sur des fichiers, sans dépendance propriétaire et à faibles dépendances.
