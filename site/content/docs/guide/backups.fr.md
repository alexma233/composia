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

Le service Compose Rustic est un conteneur Docker normal exécutant le binaire `rustic`. Il doit avoir un volume pour le répertoire de protection des données.

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
| `files.copy` | Copier des fichiers et répertoires. À utiliser pour les données lisibles en direct. |
| `files.copy_after_stop` | Arrêter le projet Compose, copier les fichiers, redémarrer. À utiliser pour les données qui doivent être figées. |
| `database.pgdumpall` | Exécuter `pg_dumpall` à l'intérieur du service Compose. Nécessite que `service` soit défini. |
| `database.pgimport` | Restaurer un dump PostgreSQL via `psql`. Nécessite que `service` soit défini. |

### Champs d'action de données

| Clé | Type | Requis pour | Description |
|-----|------|-------------|-------------|
| `strategy` | `string` | Toutes | Stratégie de sauvegarde ou de restauration. |
| `service` | `string` | `database.*` | Nom du service Compose. |
| `include` | `[]string` | `files.*` | Chemins à inclure, relatifs au répertoire du service. Reste à l'intérieur de la racine du service. |

### Types de chemins d'inclusion

Les chemins peuvent référencer :

- **Chemins de service** : fichiers ou répertoires à l'intérieur du répertoire du service. Copiés directement.
- **Volumes nommés** : noms de volumes Docker. Sauvegardés en lançant un conteneur temporaire qui monte le volume.

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
   - Préparer les données selon la stratégie de sauvegarde (`files.copy`, `files.copy_after_stop`, `database.pgdumpall`).
   - Exécuter `docker compose run rustic backup` avec des balises identifiant le service et l'élément de données.
   - Rapporter le résultat (ID du snapshot) au contrôleur.
3. La tâche se termine lorsque tous les éléments sont sauvegardés.

Les artefacts de sauvegarde sont identifiés par les IDs de snapshot Rustic. Les balises incluent `composia-service:<nom>` et `composia-data:<nom>` pour les opérations ultérieures de restauration et d'oubli.

## Restauration

Déclenchez une restauration via l'interface web depuis la page des sauvegardes ou via la CLI :

```bash
composia backup restore <backup-id>
```

Le processus de restauration :

1. **Rendu** : télécharger le bundle de service et le bundle Rustic. Lire `.composia-restore.json`.
2. **Restauration** : pour chaque élément :
   - Exécuter `docker compose run rustic restore <snapshot_id> <target_dir>`.
   - Appliquer les données restaurées selon la stratégie de restauration :
     - `files.copy` : remplacer les fichiers dans le répertoire du service.
     - `files.copy_after_stop` : arrêter Compose, remplacer les fichiers, redémarrer Compose.
     - `database.pgimport` : exécuter `docker compose exec <service> psql` avec le dump SQL restauré.

## Maintenance Rustic

Les tâches de maintenance utilisent le service d'infrastructure Rustic :

- **`rustic_init`** : exécute `docker compose run rustic init` pour initialiser le dépôt Rustic. À utiliser une fois par configuration Rustic.
- **`rustic_forget`** : exécute `docker compose run rustic forget` avec des filtres de balises. Limité à un service, un élément de données ou l'ensemble du dépôt.
- **`rustic_prune`** : exécute `docker compose run rustic prune` pour supprimer les données non référencées.

Déclenchez la maintenance depuis l'interface web ou la CLI :

```bash
composia node init-rustic main
composia node forget-rustic main
composia node prune-rustic main
```

## Voir aussi

- [Configuration des services](/docs/guide/service/) — protection des données et planification des sauvegardes.
- [Migration](/docs/guide/migrate/) — déplacer des services entre nœuds avec les données préservées via les sauvegardes.
