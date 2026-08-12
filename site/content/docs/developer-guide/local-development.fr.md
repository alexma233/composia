---
title: "Développement local"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

Utilisez `mise` comme point d'entrée unique pour le développement local.

## Configuration

```bash
mise install
mise run setup
```

`mise run setup` prépare les fichiers locaux sous `dev/`, crée les jetons de développement et les clés age, puis initialise le dépôt du contrôleur si nécessaire. Les fichiers locaux existants ne sont pas remplacés.

## Pile applicative

Démarrez le contrôleur, l'agent et l'interface web :

```bash
mise run dev
```

- Contrôleur : <http://127.0.0.1:7001>
- Interface web : <http://127.0.0.1:5173>

Suivez les journaux ou arrêtez la pile :

```bash
mise run dev:logs
mise run dev:down
```

## Documentation

La documentation ne démarre pas avec la pile applicative par défaut.

```bash
mise run dev:docs # documentation seule sur :5174
mise run dev:all  # application et documentation
```

## Vérifications

Exécutez les vérifications locales standard avant de valider :

```bash
mise run check
```

Exécutez les vérifications backend plus lentes si nécessaire :

```bash
mise run check:full
```

## Génération Protobuf

Après une modification de `proto/**`, régénérez les sorties Protobuf suivies et la documentation API :

```bash
mise run gen
```

Validez les fichiers générés sous `gen/go/`, `web/src/lib/gen/` et `site/content/docs/developer-guide/api/`.

## E2E

Exécutez tous les tests E2E locaux avec la vraie pile de fixtures du contrôleur et de l'agent :

```bash
mise run e2e
```
