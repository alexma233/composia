---
title: "Configuration des services"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Chaque service réside dans un répertoire de premier niveau à l'intérieur du dépôt du contrôleur. Un répertoire de service contient `composia-meta.yaml` et un ou plusieurs fichiers Docker Compose.

Service minimal :

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

Avec le comportement par défaut, Composia cherche `docker-compose.yaml` dans le même répertoire.

## Clés de niveau supérieur

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Nom unique du service. |
| `project_name` | `string` | Non | Remplacement du nom de projet Docker Compose. Par défaut, un nom de service normalisé. |
| `compose_files` | `[]string` | Non | Chemins des fichiers Compose relatifs au répertoire du service. |
| `enabled` | `bool` | Non | Si le service est actif. Par défaut `true`. |
| `nodes` | `[]string` | Oui | IDs des nœuds cibles. Chacun doit exister dans `controller.nodes`. |
| `infra` | `object` | Non | Déclare ce service comme infrastructure Caddy, Rustic ou config-only. |
| `network` | `object` | Non | Paramètres Caddy et DNS. |
| `update` | `object` | Non | Paramètres de mise à jour d'images. |
| `data_protect` | `object` | Non | Définitions de données de sauvegarde et de restauration. |
| `backup` | `object` | Non | Sauvegardes planifiées pour les données protégées. |
| `migrate` | `object` | Non | Données protégées activées pour la migration. |
| `auto_deploy` | `bool` | Non | Déployer automatiquement ce service après des modifications du dépôt. |

Les entrées `compose_files` doivent être des chemins relatifs, doivent rester à l'intérieur du répertoire du service et ne doivent pas être dupliquées.

## Services d'infrastructure

### `infra.caddy`

Déclare le service d'infrastructure Caddy du dépôt.

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| Clé | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Nom du service Compose. Par défaut `caddy`. |
| `config_dir` | `string` | Répertoire de configuration Caddy. Par défaut `/etc/caddy`. |

Un seul service peut être déclaré comme infrastructure Caddy.

### `infra.rustic`

Déclare le service d'infrastructure Rustic du dépôt.

```yaml
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-version
      - "2"
```

| Clé | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Nom du service Compose. Par défaut `rustic`. |
| `profile` | `string` | Nom du profil Rustic. |
| `data_protect_dir` | `string` | Chemin dans le conteneur mappé vers `{StateDir}/data-protect` de l'agent. |
| `init_args` | `[]string` | Arguments supplémentaires passés à `rustic init`. Les entrées vides sont rejetées. |

Un seul service peut être déclaré comme infrastructure Rustic.

### `infra.config`

Déclare un service d'infrastructure de configuration uniquement.

```yaml
infra:
  config: {}
```

Les services config-only ne peuvent pas être combinés avec `infra.caddy` ou `infra.rustic`. Leurs actions `data_protect` ne peuvent utiliser que `files.copy`.

## Réseau

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `enabled` | `bool` | Non | Active la gestion Caddy. Par défaut `false`. |
| `source` | `string` | Cond. | Chemin du Caddyfile relatif au répertoire du service. Requis lorsque activé. |

### `network.dns`

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: Managed by Composia
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Oui | `cloudflare`, `alidns`, `dnspod`, `route53` ou `huaweicloud`. |
| `hostname` | `string` | Oui | Nom d'hôte DNS. |
| `record_type` | `string` | Non | Vide, `A`, `AAAA` ou `CNAME`. |
| `value` | `string` | Non | Valeur de l'enregistrement DNS. Les services multi-nœuds devraient le définir explicitement. |
| `proxied` | `bool` | Non | Bascule de proxy spécifique au fournisseur, actuellement pertinent pour Cloudflare. |
| `ttl` | `uint32` | Non | TTL DNS. |
| `comment` | `string` | Non | Commentaire de l'enregistrement DNS. |

## Mises à jour d'images

```yaml
update:
  enabled: true
  auto_apply: false
  check_schedule: "0 */6 * * *"
  backup_before_update: true
  digest_pin: false
  backup_data:
    - name: db
      enabled: true
  discovery_sources:
    upstream:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    app:
      image: ghcr.io/example/app
      current:
        env:
          file: .env
          key: APP_VERSION
      discovery: upstream
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update`

| Clé | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Active les vérifications de mise à jour pour ce service. |
| `auto_apply` | `bool` | Applique automatiquement les mises à jour détectées. |
| `check_schedule` | `string` | Planification cron pour les vérifications de mise à jour. |
| `backup_before_update` | `bool` | Exécute des sauvegardes avant d'appliquer les mises à jour. |
| `backup_data` | `[]object` | Éléments de données protégés à sauvegarder avant la mise à jour. |
| `digest_pin` | `bool` | Épingler les images par empreinte. |
| `discovery_sources` | `map[string]object` | Sources de découverte réutilisables. Les sources nommées ne peuvent pas référencer une autre source. |
| `images` | `map[string]object` | Définitions de mise à jour par image. |

### `update.backup_data[]`

| Clé | Type | Description |
|-----|------|-------------|
| `name` | `string` | Nom de l'élément de données protégé. |
| `enabled` | `bool` | Inclure ou exclure cet élément. |

### `update.images.<nom>`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `image` | `string` | Oui | Dépôt d'image. |
| `auto_apply` | `bool` | Non | Remplacement auto-apply par image. |
| `check_schedule` | `string` | Non | Planification de vérification par image. |
| `backup_before_update` | `bool` | Non | Activation de la sauvegarde par image. |
| `digest_pin` | `bool` | Non | Activation de l'épinglage par empreinte par image. |
| `current` | `object` | Oui | Source de la version actuelle. |
| `discovery` | `object` ou `string` | Oui | Configuration de découverte ou référence à une source de découverte nommée. |
| `filter` | `object` | Cond. | Requis sauf si la découverte est `digest`. |

### `current`

Spécifiez exactement l'une des options suivantes :

| Clé | Description |
|-----|-------------|
| `tag` | Balise actuelle statique. |
| `env.file` + `env.key` | Lire la balise actuelle depuis un fichier d'environnement. `file` doit être relatif et rester à l'intérieur du répertoire du service. |
| `yaml.file` + `yaml.path` | Lire la balise actuelle depuis un fichier YAML. `file` doit être relatif et rester à l'intérieur du répertoire du service. |

### `discovery`

| Clé | Type | Description |
|-----|------|-------------|
| `sources` | `[]object` | Au moins une source. |
| `combine` | `string` | Vide, `merge` ou `first_success`. |
| `include_prerelease` | `bool` | Inclure les versions préliminaires. |

Types de sources de découverte :

| Type | Clés requises | Notes |
|------|---------------|-------|
| `auto` | Aucune | `repo_url` est optionnel et doit être une URL valide si défini. Doit être la seule source. |
| `probe` | Aucune | Nécessite un filtre `semver` lorsqu'un filtre est présent. |
| `registry` | Aucune | Découverte des balises du registre. |
| `digest` | Aucune | Doit être la seule source. `filter` doit être omis. |
| `github` | `repo` | `repo` est `owner/repo`. |
| `gitlab` | `project` | ID ou chemin du projet GitLab. |
| `forgejo` | `repo` | `repo` est `owner/repo`. |

### `filter`

| Type | Clés requises | Notes |
|------|---------------|-------|
| `semver` | Aucune | `allow` peut contenir `patch`, `minor`, `major`. |
| `date` | `format` | Format de date utilisé pour analyser les balises. |
| `regex` | `pattern`, `order` | `order` doit être `numeric` ou `lexicographic`. |
| `latest` | Aucune | Utilise le dernier candidat. |

## Protection des données

```yaml
data_protect:
  data:
    - name: db
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres
    - name: uploads
      backup:
        strategy: files.copy_after_stop
        include:
          - ./uploads
      restore:
        strategy: files.copy
        include:
          - ./uploads
```

### `data_protect.data[]`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Nom unique de l'élément de données. |
| `backup` | `object` | Non | Action de sauvegarde. |
| `restore` | `object` | Non | Action de restauration. |

### Action de données

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `strategy` | `string` | Oui | `files.copy`, `files.copy_after_stop`, `database.pgdumpall` ou `database.pgimport`. |
| `service` | `string` | Cond. | Requis pour les stratégies `database.*`. Nom du service Compose. |
| `include` | `[]string` | Cond. | Requis pour les stratégies `files.*`. `./...` ou les chemins contenant `/` sont des chemins de service ; les noms simples sont des volumes Docker. |

## Sauvegardes

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Doit référencer un `data_protect.data[].name` avec une action de sauvegarde. |
| `provider` | `string` | Non | Nom du fournisseur de sauvegarde. |
| `enabled` | `bool` | Non | Activer ou désactiver cette entrée de sauvegarde. |
| `schedule` | `string` | Non | Planification cron. |

## Migration

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Doit référencer un `data_protect.data[].name` avec les deux actions de sauvegarde et de restauration. |
| `enabled` | `bool` | Non | Activer ou désactiver la migration pour cet élément. |
