---
title: "Migration"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

Migrez un service d'un nœud à un autre tout en préservant l'intégrité des données. La tâche de migration orchestre les étapes de sauvegarde, arrêt, restauration, démarrage et mise à jour DNS à travers les nœuds source et cible.

## Configuration

Les éléments de données transportés pendant la migration doivent avoir à la fois une action `backup` et une action `restore` dans `data_protect`. Déclarez-les dans `migrate` :

```yaml
name: my-app
nodes:
  - main

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

migrate:
  data:
    - name: uploads
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `name` | `string` | Oui | Doit référencer un `data_protect.data[].name` avec les deux actions de sauvegarde et de restauration. |
| `enabled` | `bool` | Non | Activer ou désactiver la migration pour cet élément. |

## Exécuter la migration

**Interface web :**
1. Ouvrez la page de détail du service.
2. Utilisez les contrôles de migration pour sélectionner les nœuds source et cible.
3. Cliquez sur **Migrate**.

**CLI :**

```bash
composia service my-app migrate --source main --target edge-1 --wait --follow --timeout 30m
```

## Étapes de la migration

1. **Exporter les données** — exécuter une tâche de sauvegarde sur le nœud source pour chaque élément de données configuré.
2. **Arrêter l'instance source** — exécuter `docker compose down`, supprimer la configuration Caddy.
3. **Recharger Caddy sur la source** — supprimer l'entrée de proxy de l'instance Caddy source.
4. **Restaurer les données sur la cible** — exécuter une tâche de restauration sur le nœud cible pour chaque élément de données.
5. **Déployer sur la cible** — exécuter `docker compose up -d`, synchroniser la configuration Caddy.
6. **Recharger Caddy sur la cible** — appliquer l'entrée de proxy sur l'instance Caddy cible.
7. **Mettre à jour le DNS** — mettre à jour les enregistrements DNS pour pointer vers le nœud cible.
8. **Écrire la configuration** — mettre à jour `nodes` dans `composia-meta.yaml`, commiter dans Git.

## Considérations

- Le service doit être déployé sur le nœud source et le nœud cible doit être en ligne.
- La migration entraîne une brève interruption de service. Effectuez-la pendant les heures creuses.
- L'instance source est arrêtée avant le transfert de données pour garantir la cohérence.
- Pour les bases de données, utilisez les stratégies d'export (`database.pgdumpall` / `database.pgimport`).

## Rollback

State rollback is currently available in the Web UI only. Open the migration task details, choose the recovery actions that match the failed step, and start rollback there.

| Action | Description |
|--------|-------------|
| `deploy_source` | Redeploy the service on the original source node. |
| `stop_target` | Stop and clean up the service on the target node. |
| `rollback_dns` | Sync DNS records back to the source node. |

The CLI does not have a `task rollback` command yet. You can still inspect and follow the migration task with:

```bash
composia task wait <task-id> --follow --timeout 30m
```

## Voir aussi

- [Sauvegardes](/docs/guide/backups/) — configuration Rustic et configuration des sauvegardes.
- [Configuration des services](/docs/guide/service/) — référence des champs `data_protect` et `migrate`.
