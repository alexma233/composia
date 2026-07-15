---
title: "Sauvegardes"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Composia automatise les sauvegardes via Rustic. Les tâches de sauvegarde et de restauration s'exécutent sur l'agent, tandis que le contrôleur génère la configuration d'exécution.

## Architecture

Les sauvegardes nécessitent un service d'infrastructure Rustic. Le dépôt doit déclarer exactement un service avec `infra.rustic` :

```yaml {filename="rustic/composia-meta.yaml"}
name: rustic
nodes:
  - main
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
```

Le service Compose Rustic est un conteneur Docker normal exécutant le binaire `rustic`. Il doit avoir un volume qui mappe `{StateDir}/data-protect` de l'agent vers le chemin défini dans `data_protect_dir`.

## Configuration du contrôleur

```yaml
controller:
  backup:
    default_schedule: "0 2 * * *"
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "0 1 * * Sun"
      prune_schedule: "0 3 * * Sun"
```

| Clé | Description |
|-----|-------------|
| `backup.default_schedule` | Planification cron par défaut pour les sauvegardes de service. |
| `rustic.main_nodes` | IDs des nœuds où les opérations Rustic s'exécutent. Chaque ID doit référencer un nœud configuré. |
| `rustic.maintenance.forget_schedule` | Planification cron pour `rustic forget`. |
| `rustic.maintenance.prune_schedule` | Planification cron pour `rustic prune`. |

## Protection des données de service

Définissez ce qu'il faut sauvegarder dans `composia-meta.yaml` sous `data_protect` :

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

### Stratégies de données

| Stratégie | Rôle |
|----------|---------|
| `files.copy` | Monte les chemins source en lecture seule dans le conteneur Rustic via bind mount pour la sauvegarde. Pour les données lisibles en direct. |
| `files.copy_after_stop` | Arrête le projet Compose, monte les chemins source en bind mount, sauvegarde, puis redémarre. Pour les données qui doivent être figées. |
| `database.pgdumpall` | Exécute `pg_dumpall` à l'intérieur du service Compose. Nécessite que `service` soit défini. |
| `database.pgimport` | Restaure un dump PostgreSQL via `psql`. Nécessite que `service` soit défini. |

### Champs d'action de données

| Clé | Type | Requis pour | Description |
|-----|------|-------------|-------------|
| `strategy` | `string` | Toutes | Stratégie de sauvegarde ou de restauration. |
| `service` | `string` | `database.*` | Nom du service Compose. |
| `include` | `[]string` | `files.*` | Chemins à inclure. Chemins de service (relatifs à la racine du service, commençant par `./` ou contenant `/`) ou noms de volumes Docker (nom simple sans séparateur de chemin). |

### Types de chemins d'inclusion

Les chemins peuvent référencer :

- **Chemins de service** : fichiers ou répertoires à l'intérieur du répertoire du service. Montés en lecture seule dans le conteneur Rustic via `-v`.
- **Volumes nommés** : noms de volumes Docker. Montés en lecture seule dans le conteneur Rustic via `-v` (aucun conteneur temporaire nécessaire).

## Planifications de sauvegarde

Activez les sauvegardes planifiées pour les éléments de données protégés :

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
    - name: uploads
      enabled: true
      schedule: "0 3 * * Sun"
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Doit référencer un `data_protect.data[].name` qui a une action de sauvegarde. |
| `provider` | `string` | Non | Nom du fournisseur de sauvegarde. |
| `enabled` | `bool` | Non | Activer ou désactiver cette sauvegarde. |
| `schedule` | `string` | Non | Expression cron. `"none"` désactive la planification tout en conservant l'entrée. |

Lorsque `schedule` est défini, le contrôleur planifie des tâches `backup` récurrentes. La valeur `backup.default_schedule` du contrôleur est utilisée comme valeur de repli si une entrée de service ne spécifie pas sa propre planification.

## Déroulement d'une sauvegarde

Une tâche de sauvegarde exécute ces étapes sur l'agent :

1. **Rendu** : télécharger le bundle de service et le bundle Rustic depuis le contrôleur. Lire `.composia-backup.json` généré par le contrôleur.
2. **Sauvegarde** : pour chaque élément de données dans la configuration d'exécution :
   - `files.*`: créer un répertoire de staging vide sous `data-protect`, ajouter des bind mounts `-v` pour chaque chemin ou volume inclus, puis exécuter `docker compose run -v ... rustic backup` avec des balises identifiant le service et l'élément de données. Aucune donnée n'est copiée dans le répertoire state de l'agent.
   - `database.pgdumpall`: exécuter `docker compose exec <service> pg_dumpall`, écrire le dump SQL dans un fichier de staging sous `data-protect`, puis exécuter `docker compose run rustic backup` sur le répertoire de staging.
   - Rapporter le résultat (ID du snapshot) au contrôleur.
3. La tâche se termine lorsque tous les éléments sont sauvegardés.

Les artefacts de sauvegarde sont identifiés par les IDs de snapshot Rustic. Les balises incluent `composia-service:<nom>` et `composia-data:<nom>` pour les opérations ultérieures de restauration et d'oubli.

## Restauration

Déclenchez une restauration via l'interface web depuis la page des sauvegardes ou via la CLI :

```bash
composia backup restore main <backup-id> --wait --follow --timeout 30m
```

The first argument is the target node. Use `--wait --follow` to block until the restore finishes and stream task logs.

Le processus de restauration :

1. **Rendu** : télécharger le bundle de service et le bundle Rustic. Lire `.composia-restore.json`.
2. **Restauration** : pour chaque élément :
   - `files.copy` / `files.copy_after_stop`: nettoyer les cibles de restauration (les cibles doivent exister), créer un répertoire de staging vide sous `data-protect`, monter chaque chemin cible ou volume Docker dans l'arborescence de staging via bind mount, puis exécuter `docker compose run -v ... rustic restore <snapshot_id> <staging_dir>`. Les données restaurées sont écrites directement dans les emplacements cibles — aucune étape de copie post-restauration.
   - `files.copy_after_stop`: arrête également le projet Compose avant la restauration et le redémarre après.
   - `database.pgimport`: exécuter `docker compose run rustic restore <snapshot_id>` dans un répertoire de staging, puis exécuter `docker compose exec <service> psql` avec le dump SQL restauré.

Les cibles de restauration pour les chemins de service `files.*` doivent déjà exister sur l'agent (utilisé pour déterminer la sémantique bind-mount fichier/répertoire). Les cibles de volumes Docker sont vidées avant la restauration.

## Maintenance Rustic

Les tâches de maintenance utilisent le service d'infrastructure Rustic :

- **`rustic_init`** : exécute `docker compose run rustic init` pour initialiser le dépôt Rustic. À utiliser une fois par configuration Rustic.
- **`rustic_forget`** : exécute `docker compose run rustic forget` avec des filtres de balises. Limité à un service, un élément de données ou l'ensemble du dépôt.
- **`rustic_prune`** : exécute `docker compose run rustic prune` pour supprimer les données non référencées.

Déclenchez la maintenance depuis l'interface web ou la CLI :

```bash
composia rustic init main --yes --wait --follow
composia rustic forget main --service my-app --data uploads --wait --follow
composia rustic prune main --wait --follow
```

Use `--wait --follow` when you want the CLI to wait for the maintenance task and stream logs.

## Voir aussi

- [Configuration des services](/docs/guide/service/) — protection des données et planification des sauvegardes.
- [Migration](/docs/guide/migrate/) — déplacer des services entre nœuds avec les données préservées via les sauvegardes.
