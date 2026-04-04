# Composia Implementation Steps

This document turns `plan.md` into a practical execution order for the current repository state.

## Source of Truth

- `plan.md` is the product and architecture source of truth.
- `steps.md` is only an implementation ordering document.
- If `steps.md` and `plan.md` ever conflict, follow `plan.md`.
- Do not treat scaffolding, placeholders, or partial APIs as "done" if their behavior still differs from `plan.md`.
- Before adding new surface area, first remove any architectural drift from the current implementation.

## Current Repository State

The repository is no longer at the initial scaffold stage.

Implemented or mostly implemented:

- `composia controller` and `composia agent` subcommands exist.
- Shared `config.yaml` loading and validation exist for the v1 `controller` and `agent` model.
- Controller startup initializes local directories and opens SQLite.
- SQLite schema exists for `nodes`, `services`, `tasks`, `task_steps`, and `backups`.
- Minimal controller-agent ConnectRPC wiring exists.
- Agent heartbeat works and node state is persisted.
- Service repo scanning and `composia-meta.yaml` validation exist.
- Controller APIs already expose read paths for services, tasks, nodes, backups, and repo inspection.
- Agent can pull queued tasks and download bundles for service execution.
- Basic remote task execution exists for `deploy`, `update`, `stop`, and `restart`.

Still partial or not aligned with `plan.md` yet:

- Service runtime state is still inferred from completed tasks instead of coming from explicit agent status reporting.
- Task log upload is not yet implemented with the planned streaming and resume semantics.
- Backup execution is still placeholder behavior.
- Repo write APIs, Git write transactions, and sync handling are not implemented.
- Secret APIs and age-based secret workflows are not implemented.
- DNS, Caddy management, prune, and migrate are not implemented.
- The web UI is still a placeholder shell.
- There is leftover controller-side worker code that should not become a second execution architecture.

## Execution Rule

From this point forward, agents working in this repository should follow these rules:

1. Keep the implementation aligned with `plan.md`, even if that means tightening or replacing existing placeholder behavior.
2. Prefer finishing and correcting already-started foundation work before adding more APIs or UI pages.
3. Do not add new behavior that changes task semantics, repo semantics, or controller-agent responsibilities unless `plan.md` already defines it.
4. Treat migration, backup, DNS, secrets, and repo writes as architecture-sensitive work that must match the documented v1 semantics, not shortcut variants.

## Phase 1: Remove Architecture Drift

Status: in progress

Goal: make the current backend match the controller-agent contract described in `plan.md` before expanding the feature surface.

1. Keep `controller` as the durable state owner and task scheduler.
2. Keep `agent` as the execution side through `PullNextTask` and `GetServiceBundle`.
3. Remove, rewrite, or clearly retire leftover controller-local worker paths that imply a competing execution model.
4. Add explicit agent-to-controller service runtime reporting instead of deriving runtime state only from terminal task results.
5. Move task log upload toward the planned streaming contract so the protocol shape does not drift further.

Deliverable:

- The running architecture is internally consistent and matches `plan.md`.

## Phase 2: Finish the Task Foundation

Status: in progress

Goal: make the existing task system reliable and strictly conform to the documented v1 task model.

1. Keep every task bound to a specific `repo_revision`.
2. Enforce global serial execution semantics as documented.
3. Preserve the `pending`, `running`, `awaiting_confirmation`, and terminal state rules exactly as described in `plan.md`.
4. Keep task step summaries and per-task logs under `controller.log_dir`.
5. Preserve restart recovery behavior for `running` tasks and keep `awaiting_confirmation` tasks intact.
6. Keep service-level conflict checks aligned with the repo write conflict rules in `plan.md`.

Deliverable:

- Tasks are durable, observable, and semantically consistent with the plan.

## Phase 3: Stabilize the First Real Service Actions

Status: in progress

Goal: finish the already-started day-1 service operations before adding broader workflows.

1. Keep `deploy` as the first fully supported end-to-end task.
2. Finish `update`, `stop`, and `restart` so their task steps and runtime effects match the plan.
3. Replace runtime-status guessing with agent-reported runtime state.
4. Add stronger end-to-end tests around bundle download, task execution, and task state persistence.

Deliverable:

- `deploy`, `update`, `stop`, and `restart` are trustworthy controller-agent flows.

## Phase 4: Add Safe Desired-State Repo Writes

Status: pending

Goal: let the controller own Git-backed desired state changes exactly as documented.

1. Add repo lock handling.
2. Add `RepoService.UpdateRepoFile`.
3. Add repo validation during write transactions.
4. Add service conflict checks for writes that touch locked service directories.
5. Add commit creation with the configured author behavior.
6. Add optional remote sync behavior, push reporting, and repo sync state.
7. Add `RepoService.SyncRepo` with the documented clean-worktree requirements.

Deliverable:

- Desired-state file edits are safe, validated, committed, and consistent with the Git model in `plan.md`.

## Phase 5: Add Secret Handling

Status: pending

Goal: implement the selected age-based secrets model without leaving plaintext in `controller.repo_dir`.

1. Add controller-side decryption for `.secret.env.enc`.
2. Add controller-side re-encryption using the configured recipients.
3. Add `SecretService.GetServiceSecretEnv`.
4. Add `SecretService.UpdateServiceSecretEnv`.
5. Reuse the same repo lock, validation, commit, and conflict rules as normal repo writes.
6. Ensure decrypted runtime files are only included in agent bundles and never persisted in the controller Git working tree.

Deliverable:

- Service secrets follow the documented age-based workflow.

## Phase 6: Replace Placeholder Backup Behavior

Status: pending

Goal: turn the current backup scaffolding into the first real data-protection workflow.

1. Keep `data_protect` parsing and validation aligned with the documented v1 strategies.
2. Replace placeholder backup execution with real strategy execution.
3. Support the documented v1 strategies only.
4. Persist one backup record per data item.
5. Add backup querying paths needed by the API and UI.
6. Only add scheduling after manual backup behavior is correct.

Deliverable:

- Backup tasks produce real artifacts and real backup records.

## Phase 7: Add Read-Write Web UI on Real APIs

Status: pending

Goal: replace the placeholder frontend with the actual control-plane UI described in `plan.md`.

1. Add service list and service detail pages.
2. Add node list and node detail pages.
3. Add task history and task detail pages, including log tailing.
4. Add backup views.
5. Add repo browsing and editing pages.
6. Add service secret editing pages.
7. Keep the UI responsive on mobile from the start.

Deliverable:

- The frontend becomes a real controller UI instead of a landing page shell.

## Phase 8: Add DNS, Caddy, and Node Operations

Status: pending

Goal: implement the documented day-2 operational actions after repo writes and base service flows are stable.

1. Add `dns_update` task behavior.
2. Add `caddy_reload` task behavior.
3. Add `prune` task behavior.
4. Add the related controller public APIs.
5. Keep task steps, logging, and node targeting aligned with the task model.

Deliverable:

- Day-2 node and ingress operations work through the same task system.

## Phase 9: Add Migration Last

Status: pending

Goal: implement the most complex v1 workflow only after the rest of the platform semantics are stable.

1. Add `MigrateService(target_node_id)`.
2. Validate `migrate.data[]` exactly as documented.
3. Export selected data from the source node.
4. Transfer artifacts and runtime files to the target node.
5. Restore data and start the target service.
6. Refresh Caddy and DNS as documented.
7. Persist the repo `node` change only after runtime cutover succeeds.
8. Use `awaiting_confirmation` exactly as defined in `plan.md`.
9. Apply the documented repo conflict and reconciliation rules during `persist_repo`.

Deliverable:

- Migration works with explicit operator confirmation and the same failure boundaries described in the plan.

## Recommended Immediate Next Step

Start with Phases 1 through 3 only.

That is the smallest correct next milestone for the current codebase:

- remove architecture drift
- finish the task foundation
- make the first service actions reliable

Do not start secrets, backup, DNS, migrate, or rich UI work until those alignment items are complete.
