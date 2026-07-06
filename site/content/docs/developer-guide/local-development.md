---
title: "Local Development"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

Use `mise` as the single entrypoint for local development.

## Setup

```bash
mise install
mise run setup
```

`mise run setup` prepares local files under `dev/`, creates development tokens, creates age key files, and initializes the controller repository when needed. Existing local files are not overwritten.

## App stack

Start the controller, agent, and web UI:

```bash
mise run dev
```

- Controller: <http://127.0.0.1:7001>
- Web UI: <http://127.0.0.1:5173>

Follow logs or stop the stack:

```bash
mise run dev:logs
mise run dev:down
```

## Docs

Docs are not started with the app stack by default.

```bash
mise run dev:docs # docs only, on :5174
mise run dev:all  # app + docs
```

## Checks

Run the standard local check suite before committing:

```bash
mise run check
```

Run slower backend checks when needed:

```bash
mise run check:full
```

## Protobuf generation

After changing `proto/**`, regenerate checked-in protobuf output and API docs:

```bash
mise run gen
```

Commit the generated files under `gen/go/`, `web/src/lib/gen/`, and `site/content/docs/developer-guide/api/`.

## E2E

Run all local e2e tests against the real controller and agent fixture stack:

```bash
mise run e2e
```
