---
title: "Configuration"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Cette page couvre la configuration au niveau de l'installation : configuration du contrôleur, configuration de l'agent, variables d'environnement web et configuration des clés age.

Les définitions de service résident dans `composia-meta.yaml`. Voir le [Guide des services](/docs/guide/service/) pour ce fichier.

## Structure du fichier de configuration

Le contrôleur et l'agent utilisent le même format de fichier YAML. Un fichier peut contenir l'une ou l'autre section, ou les deux :

```yaml
controller:
  # paramètres du contrôleur

agent:
  # paramètres de l'agent
```

Au moins l'un de `controller` ou `agent` doit être présent.

Lorsque le même fichier de configuration contient les deux sections, l'agent local est traité comme le nœud intégré :

- `agent.node_id` doit être `main`.
- `controller.nodes` doit inclure une entrée avec `id: main`.
- `controller.repo_dir` et `agent.repo_dir` ne doivent pas être le même chemin.

## Modèle de configuration complet

Ce modèle montre chaque clé prise en charge au niveau de l'installation. C'est une référence de structure, pas un défaut à copier-coller. Supprimez les sections que vous n'utilisez pas, supprimez les éléments de liste vides et utilisez soit des valeurs en ligne, soit des valeurs `_file` pour chaque champ de type secret.

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"

  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      token_file: ""
      enabled: true
      comment: "Web UI access token"

  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      public_ipv4: ""
      public_ipv6: ""
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
      token_file: ""

  git:
    remote_url: ""
    branch: "main"
    pull_interval: ""
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: ""
      token: ""
      token_file: ""

  backup:
    default_schedule: ""

  updates:
    default_check_schedule: ""
    auto_apply: false
    backup_before_update: true
    digest_pin: false
    semver:
      default_allow:
        - patch
        - minor
    forge_auth:
      github:
        url: "https://github.com"
        token: ""
        token_file: ""
        api_url: "https://api.github.com"
      gitlab:
        url: "https://gitlab.com"
        token: ""
        token_file: ""
        api_url: "https://gitlab.com/api/v4"
      forgejo:
        url: "https://forgejo.example.com"
        token: ""
        token_file: ""
        api_url: ""

  auto_deploy:
    infra: false
    services: false

  dns:
    cloudflare:
      api_token: ""
      api_token_file: ""
      zones: []
    alidns:
      access_key_id: ""
      access_key_id_file: ""
      access_key_secret: ""
      access_key_secret_file: ""
      security_token: ""
      security_token_file: ""
      region_id: ""
      zones: []
    dnspod:
      secret_id: ""
      secret_id_file: ""
      secret_key: ""
      secret_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      zones: []
    route53:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      profile: ""
      hosted_zone_id: ""
      zones: []
    huaweicloud:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      region_id: ""
      zones: []

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: ""
      prune_schedule: ""

  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: ""
    armor: true

  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: false
      host: ""
      port: 587
      encryption: starttls
      username: ""
      password: ""
      password_file: ""
      from: ""
      to: []
      on: []
      task_sources: []
    telegram:
      enabled: false
      bot_token: ""
      bot_token_file: ""
      chat_id: ""
      on: []
      task_sources: []

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  controller_headers:
    - name: ""
      value: ""
      value_file: ""
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  token_file: ""
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: ""
```

Ne conservez pas les éléments de liste vides comme `controller_headers` avec un `name` vide. Ils sont affichés uniquement pour documenter la structure d'objet prise en charge.

Le jeton d'accès web et le jeton d'agent principal doivent être différents.

## Configuration des clés age

`controller.secrets` est optionnel. Configurez-le uniquement si vous utilisez les secrets chiffrés gérés par Composia.

Lorsque `controller.secrets` est configuré, `identity_file` est requis. `recipient_file` est optionnel. S'il est omis, Composia dérive le destinataire de la clé privée.

Générez une clé privée :

```bash
age-keygen -o age-identity.key
```

Fichier de destinataires optionnel :

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

Utilisez la clé privée dans la configuration :

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

Ou utilisez les deux fichiers :

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` est optionnel et vaut `true` par défaut.

## Référence de configuration du contrôleur

### Clés requises

| Clé | Type | Description |
|-----|------|-------------|
| `listen_addr` | `string` | Adresse d'écoute du contrôleur, par exemple `":7001"` ou `"127.0.0.1:7001"`. |
| `repo_dir` | `string` | Chemin du dépôt Git d'état désiré. |
| `state_dir` | `string` | Chemin d'état du contrôleur. |
| `log_dir` | `string` | Répertoire des journaux de tâches. |
| `nodes` | `[]object` | Nœuds d'agent configurés. La clé doit être présente, même si vide. |

### Clés optionnelles de niveau supérieur

| Clé | Type | Description |
|-----|------|-------------|
| `access_tokens` | `[]object` | Jetons API pour l'interface web, la CLI et les clients externes. |
| `backup` | `object` | Valeurs par défaut globales de sauvegarde. |
| `git` | `object` | Synchronisation distante du dépôt d'état désiré. |
| `notifications` | `object` | Notifications Alertmanager, SMTP et Telegram. |
| `dns` | `object` | Identifiants des fournisseurs DNS. |
| `rustic` | `object` | Paramètres de maintenance Rustic. |
| `secrets` | `object` | Paramètres de chiffrement age. |
| `updates` | `object` | Valeurs par défaut de mise à jour d'images et authentification des API forges. |
| `auto_deploy` | `object` | Bascule globales de déploiement automatique. |

### `nodes[]`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `id` | `string` | Oui | ID unique du nœud. |
| `display_name` | `string` | Non | Nom affiché dans l'interface. |
| `enabled` | `bool` | Non | Désactiver un nœud sans le supprimer. |
| `public_ipv4` | `string` | Non | IPv4 publique utilisée par les workflows DNS. |
| `public_ipv6` | `string` | Non | IPv6 publique utilisée par les workflows DNS. |
| `token` | `string` | Oui* | Jeton d'authentification de l'agent. |
| `token_file` | `string` | Non | Lire le jeton depuis un fichier. |

*Utilisez soit `token`, soit `token_file`, pas les deux.

### `access_tokens[]`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Nom du jeton. |
| `token` | `string` | Oui* | Valeur du jeton. |
| `token_file` | `string` | Non | Lire le jeton depuis un fichier. |
| `enabled` | `bool` | Non | Désactiver un jeton sans le supprimer. |
| `comment` | `string` | Non | Note administrative. |

Les jetons d'accès ne doivent pas dupliquer les jetons de nœud ou d'autres jetons d'accès.

### `git`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `remote_url` | `string` | Non | URL du dépôt Git distant. |
| `branch` | `string` | Non | Branche à synchroniser. |
| `pull_interval` | `string` | Cond. | Requis lorsque `remote_url` est défini. |
| `author_name` | `string` | Non | Nom de l'auteur des commits pour les écritures du contrôleur. |
| `author_email` | `string` | Non | E-mail de l'auteur des commits. |
| `auth.username` | `string` | Non | Nom d'utilisateur Git. |
| `auth.token` | `string` | Non | Jeton Git. |
| `auth.token_file` | `string` | Non | Lire le jeton Git depuis un fichier. |

### `secrets`

Cette section entière est optionnelle. Si la section est présente, ces règles s'appliquent :

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Oui | Doit être `age`. |
| `identity_file` | `string` | Oui | Chemin de la clé privée age. |
| `recipient_file` | `string` | Non | Chemin du fichier de destinataires age. Si omis, le destinataire est dérivé de `identity_file`. |
| `armor` | `bool` | Non | Sortie chiffrée en armure ASCII. Par défaut `true`. |

### `backup`

| Clé | Type | Description |
|-----|------|-------------|
| `default_schedule` | `string` | Planification cron par défaut pour les sauvegardes de service. |

### `updates`

| Clé | Type | Description |
|-----|------|-------------|
| `default_check_schedule` | `string` | Planification cron par défaut pour les vérifications de mise à jour d'images. |
| `auto_apply` | `bool` | Appliquer les mises à jour automatiquement par défaut. |
| `backup_before_update` | `bool` | Sauvegarder les données avant d'appliquer les mises à jour. |
| `digest_pin` | `bool` | Épingler les images par empreinte. |
| `semver.default_allow` | `[]string` | Niveaux d'incrément semver autorisés : `patch`, `minor`, `major`. |
| `forge_auth.github` | `object` ou `[]object` | Authentification API GitHub. |
| `forge_auth.gitlab` | `object` ou `[]object` | Authentification API GitLab. |
| `forge_auth.forgejo` | `object` ou `[]object` | Authentification API Forgejo. |

Chaque entrée d'authentification forge prend en charge :

| Clé | Type | Description |
|-----|------|-------------|
| `url` | `string` | URL de base de la forge. |
| `token` | `string` | Jeton API. |
| `token_file` | `string` | Lire le jeton API depuis un fichier. |
| `api_url` | `string` | Remplacement de l'URL de l'API. |

### `auto_deploy`

| Clé | Type | Description |
|-----|------|-------------|
| `infra` | `bool` | Déployer automatiquement les services d'infrastructure après des modifications Git. |
| `services` | `bool` | Déployer automatiquement les services ordinaires après des modifications Git. |

### `dns`

| Clé fournisseur | Clés d'identifiants | Clés communes |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`, `api_token_file` | `zones` |
| `alidns` | `access_key_id`, `access_key_id_file`, `access_key_secret`, `access_key_secret_file`, `security_token`, `security_token_file`, `region_id` | `zones` |
| `dnspod` | `secret_id`, `secret_id_file`, `secret_key`, `secret_key_file`, `session_token`, `session_token_file`, `region` | `zones` |
| `route53` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `session_token`, `session_token_file`, `region`, `profile`, `hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `region_id` | `zones` |

### `rustic`

| Clé | Type | Description |
|-----|------|-------------|
| `main_nodes` | `[]string` | IDs des nœuds qui exécutent les opérations Rustic. Chacun doit référencer `controller.nodes`. |
| `maintenance.forget_schedule` | `string` | Planification cron pour `rustic forget`. |
| `maintenance.prune_schedule` | `string` | Planification cron pour `rustic prune`. |

### `notifications.alertmanager`

| Clé | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Activé par défaut lorsque la section existe. |
| `listen_path` | `string` | Chemin du webhook. Par défaut `/api/v1/alerts`. Doit commencer par `/`. |

### `notifications.smtp`

| Clé | Type | Requis si activé | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Non | Activé par défaut lorsque la section existe. |
| `host` | `string` | Oui | Hôte SMTP. |
| `port` | `int` | Oui | Port SMTP, de 1 à 65535. |
| `encryption` | `string` | Non | `none`, `starttls` ou `ssl_tls`. Par défaut `starttls`. |
| `username` | `string` | Non | Nom d'utilisateur SMTP. |
| `password` | `string` | Non | Mot de passe SMTP. |
| `password_file` | `string` | Non | Lire le mot de passe depuis un fichier. |
| `from` | `string` | Oui | Adresse de l'expéditeur. |
| `to` | `[]string` | Oui | Liste des destinataires. |
| `on` | `[]string` | Non | Filtres d'événements de notification. |
| `task_sources` | `[]string` | Non | Filtres de source de tâche : `web`, `cli`, `others`, `schedule`, `system`. |

### `notifications.telegram`

| Clé | Type | Requis si activé | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Non | Activé par défaut lorsque la section existe. |
| `bot_token` | `string` | Oui* | Jeton du bot Telegram. |
| `bot_token_file` | `string` | Non | Lire le jeton du bot depuis un fichier. |
| `chat_id` | `string` | Oui | ID de la discussion cible. |
| `on` | `[]string` | Non | Filtres d'événements de notification. |
| `task_sources` | `[]string` | Non | Filtres de source de tâche. |

## Référence de configuration de l'agent

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `controller_addr` | `string` | Oui | URL du contrôleur accessible depuis l'agent. |
| `controller_grpc` | `bool` | Non | Utiliser gRPC au lieu de Connect sur HTTP. |
| `controller_headers` | `[]object` | Non | En-têtes HTTP supplémentaires envoyés au contrôleur. |
| `node_id` | `string` | Oui | ID de nœud de cet agent. Doit correspondre à `controller.nodes[].id`. |
| `token` | `string` | Oui* | Jeton de nœud correspondant à la configuration du contrôleur. |
| `token_file` | `string` | Non | Lire le jeton de nœud depuis un fichier. |
| `repo_dir` | `string` | Oui | Chemin du dépôt de services de l'agent. |
| `state_dir` | `string` | Oui | Répertoire d'état de l'agent. |
| `caddy` | `object` | Non | Paramètres Caddy côté agent. |

*Utilisez soit `token`, soit `token_file`, pas les deux.

### `controller_headers[]`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Nom de l'en-tête HTTP. Les noms d'en-tête sont dédupliqués sans tenir compte de la casse. |
| `value` | `string` | Oui* | Valeur de l'en-tête. |
| `value_file` | `string` | Non | Lire la valeur de l'en-tête depuis un fichier. |

### `caddy`

| Clé | Type | Description |
|-----|------|-------------|
| `generated_dir` | `string` | Répertoire de configuration Caddy généré. Par défaut `<state_dir>/caddy/generated`. |

## Variables d'environnement web

Le serveur web lit les variables d'environnement. Dans Docker Compose, celles-ci sont définies via `.env`.

| Variable | Requise | Description |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | Oui | Adresse du contrôleur depuis le processus du serveur web. Dans Docker Compose : `http://controller:7001`. |
| `WEB_BROWSER_CONTROLLER_ADDR` | Oui | Adresse du contrôleur depuis le navigateur. |
| `WEB_CONTROLLER_ACCESS_TOKEN` | Oui | Jeton d'accès au contrôleur. Doit correspondre à `controller.access_tokens[].token`. |
| `WEB_CONTROLLER_HEADERS` | Non | Objet JSON d'en-têtes supplémentaires envoyés par le serveur web lors des appels au contrôleur. |
| `WEB_LOGIN_USERNAME` | Oui | Nom d'utilisateur de connexion web. |
| `WEB_LOGIN_PASSWORD_HASH` | Oui | Hachage de mot de passe Argon2. |
| `WEB_SESSION_SECRET` | Oui | Secret aléatoire de signature de session. |
| `ORIGIN` | Dépend du déploiement | Origine publique du serveur web. |
| `HOST` | Non | Adresse de liaison de l'hôte. |
| `PORT` | Non | Port du serveur web. |

## Valeurs en ligne et valeurs `_file`

De nombreux champs de type secret prennent en charge à la fois les valeurs en ligne et les références de fichiers. Exemples :

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

Utilisez une seule forme. Si les deux sont définies, le démarrage échoue.
