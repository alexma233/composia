---
title: "Mises à jour d'images"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia détecte les nouvelles balises d'images et peut appliquer les mises à jour automatiquement. Les tâches de vérification d'images s'exécutent sur l'agent et rapportent les résultats au contrôleur.

## Fonctionnement

Le contrôleur planifie des tâches `image_check` périodiques selon la configuration de mise à jour du service. Chaque vérification :

1. L'agent télécharge le bundle de service.
2. Lit `docker compose config --format json` pour découvrir les images en cours d'exécution.
3. Rapporte les empreintes locales et distantes pour chaque image.
4. Pour les images configurées dans `update.images`, vérifie les nouvelles balises candidates en utilisant les sources de découverte configurées.
5. Rapporte les résultats au contrôleur. Le contrôleur enregistre les mises à jour disponibles et peut les appliquer automatiquement.

## Valeurs par défaut du contrôleur

Les valeurs par défaut globales sont définies dans la configuration du contrôleur :

```yaml
controller:
  updates:
    default_check_schedule: "0 */6 * * *"
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
        token: "REPLACE"
        api_url: "https://api.github.com"
```

La section `update` au niveau du service remplace ces valeurs par défaut.

## Configuration du service

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
    upstream-gh:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    api:
      image: ghcr.io/example/api
      current:
        env:
          file: .env
          key: API_VERSION
      discovery: upstream-gh
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update` niveau supérieur

| Clé | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Active les vérifications de mise à jour pour ce service. |
| `auto_apply` | `bool` | Applique automatiquement les mises à jour détectées. |
| `check_schedule` | `string` | Planification cron pour les vérifications de mise à jour. |
| `backup_before_update` | `bool` | Exécute une sauvegarde avant d'appliquer une mise à jour. |
| `backup_data` | `[]object` | Éléments de données protégés à sauvegarder avant la mise à jour. Chaque élément a un `name` et un `enabled` optionnel. |
| `digest_pin` | `bool` | Épingler les images par empreinte pour la reproductibilité. |
| `discovery_sources` | `map[string]object` | Configurations de découverte nommées réutilisables. |
| `images` | `map[string]object` | Configuration de mise à jour par image. Les clés sont des noms arbitraires correspondant aux images à vérifier. |

### `images.<nom>`

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `image` | `string` | Oui | Référence complète de l'image, par exemple `ghcr.io/example/api`. |
| `auto_apply` | `bool` | Non | Remplacement auto-apply par image. |
| `check_schedule` | `string` | Non | Planification de vérification par image. |
| `backup_before_update` | `bool` | Non | Activation de la sauvegarde par image. |
| `digest_pin` | `bool` | Non | Activation de l'épinglage par empreinte par image. |
| `current` | `object` | Oui | Comment trouver la version actuellement déployée. |
| `discovery` | `object` ou `string` | Oui | Configuration de découverte ou référence à une entrée nommée `discovery_sources`. |
| `filter` | `object` | Cond. | Filtre de version. Requis sauf si le mode de découverte est `digest`. |

### `current`

Exactement une de ces sources doit être spécifiée :

**Balise statique :**

```yaml
current:
  tag: "v1.2.3"
```

**Fichier d'environnement :**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

Le chemin `file` est relatif au répertoire du service. Composia lit le fichier, cherche les lignes `CLE=VALEUR` et extrait la valeur.

**Fichier YAML :**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

Le `path` est un chemin séparé par des points dans l'arborescence du document YAML. La valeur à ce chemin doit être un scalaire.

### Découverte

Les sources de découverte peuvent être :

**Référence nommée** vers une entrée `discovery_sources` :

```yaml
discovery: upstream-gh
```

**Définition en ligne :**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

Types de sources de découverte :

| Type | Clés requises | Comportement |
|------|---------------|----------|
| `probe` | Aucune | Sondage semver : recherche les versions supérieures en sondant les manifestes du registre. Nécessite un filtre `semver`. |
| `registry` | Aucune | Liste toutes les balises du registre d'images. |
| `auto` | Aucune (`repo_url` optionnel) | Essaie `probe` puis `registry` comme découverte fusionnée. Doit être la seule source dans une configuration de découverte. |
| `digest` | Aucune | Compare uniquement l'empreinte distante avec l'empreinte locale. Pas de comparaison de balises. `filter` doit être omis. Doit être la seule source. |
| `github` | `repo` (`owner/repo`) | Interroge les releases GitHub. Traité côté contrôleur. |
| `gitlab` | `project` | Interroge les releases GitLab. Traité côté contrôleur. |
| `forgejo` | `repo` (`owner/repo`) | Interroge les releases Forgejo. Traité côté contrôleur. |

`combine` accepte `merge` (union de tous les résultats des sources) ou `first_success` (la première source qui retourne des résultats gagne).

`include_prerelease` inclut les versions préliminaires dans les requêtes de releases GitHub, GitLab et Forgejo.

### Filtre

| Type | Clés requises | Comportement |
|------|---------------|----------|
| `semver` | Aucune | Filtrer par version sémantique. `allow` peut contenir `patch`, `minor`, `major`. |
| `date` | `format` | Analyser les balises comme des dates en utilisant le format donné. |
| `regex` | `pattern`, `order` | Filtrer par expression régulière. `order` doit être `numeric` ou `lexicographic`. |
| `latest` | Aucune | Prendre la dernière balise sans filtrer. |

#### Sondage semver

Avec `type: probe` et un filtre `semver`, Composia recherche les balises candidates en construisant des numéros de version et en vérifiant si le manifeste du registre correspondant existe. Il sonde les incréments patch, minor et major selon la liste `allow`, en utilisant une recherche exponentielle avec raffinement binaire pour trouver la version disponible la plus élevée.

## Mode empreinte

Lorsque toutes les sources de découverte dans une configuration ont `type: digest`, aucune comparaison de balises n'est effectuée. Composia compare uniquement l'empreinte de l'image distante avec l'empreinte locale :

```yaml
discovery:
  sources:
    - type: digest
```

Lorsque `digest` est défini comme mode de découverte, `filter` doit être omis. Si une empreinte diffère, une mise à jour est considérée comme disponible.

## Observations d'images

Pendant les tâches de déploiement et de mise à jour, l'agent collecte également des observations d'images pour tous les services Compose. Celles-ci incluent les empreintes locales et distantes, rapportées au contrôleur indépendamment de la présence de `update.images`. Cela fournit une visibilité sur l'état des images dans l'interface web et la CLI.
