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
- Controller-agent ConnectRPC wiring exists for heartbeat, task pull, bundle download, task state, step state, log upload, backup reporting, and service runtime reporting.
- Agent heartbeat works and node state is persisted.
- Service repo scanning and `composia-meta.yaml` validation exist.
- Controller APIs expose read paths for services, tasks, nodes, backups, repo inspection, repo editing, and service secret editing.
- Agent can pull queued tasks and download bundles for service execution.
- Remote task execution exists for `deploy`, `update`, `stop`, `restart`, and `backup`.
- Task logs stream from agent to controller and are persisted under `controller.log_dir`.
- Repo write transactions and local Git commits exist for normal files and encrypted service secrets.
- age-based secret decrypt and re-encrypt helpers exist.
- The web UI is wired to real controller APIs for dashboard, services, nodes, tasks, backups, repo editing, and secret editing.
- The frontend style simplification away from the earlier bloated shell has started and should no longer be treated as untouched work.

Still partial or not aligned with `plan.md` yet:

- `PullNextTask` still behaves like short polling rather than the planned long-poll contract.
- Task `source` semantics are not fully aligned yet; several task creation paths still collapse to `cli` instead of preserving the documented caller source.
- Repo write handling is still local-only and does not yet implement remote tracking semantics, push reporting, sync state, or `SyncRepo`.
- `migrate` is still not implemented as the documented single-task workflow, and `awaiting_confirmation` is not yet exercised by a real controller flow.
- Backup execution only covers the currently implemented manual export path; restore-driven workflows and migration reuse are still incomplete.
- DNS, Caddy management, prune, and migrate are not implemented.
- CLI config and a real CLI command surface are not implemented yet.
- Scheduled update and backup execution are not implemented yet.
- The current web UI reads real controller state and the initial visual cleanup is underway, but it still lacks task action entry points, task log tailing, repo sync feedback, and stronger repo/secret editing ergonomics.

## Execution Rule

From this point forward, agents working in this repository should follow these rules:

1. Keep the implementation aligned with `plan.md`, even if that means tightening or replacing existing placeholder behavior.
2. Prefer finishing and correcting already-started foundation work before adding more APIs or UI pages.
3. Do not add new behavior that changes task semantics, repo semantics, or controller-agent responsibilities unless `plan.md` already defines it.
4. Treat migration, backup, DNS, secrets, and repo writes as architecture-sensitive work that must match the documented v1 semantics, not shortcut variants.

## Frontend Direction

The current frontend already covers real controller read paths, and the first round of style simplification has begun. The next UI work should treat operational completeness as the priority, then continue tightening the visual system.

1. Keep the calmer operations-console direction and continue removing leftover decorative styling where it still hurts scanability.
2. Prioritize missing controller actions before more aesthetic polish: deploy, update, stop, restart, backup, migrate, rerun, and other documented task entry points.
3. Add task log tailing so task detail pages expose real-time execution output instead of only showing the log path.
4. Expand repo and secret editing feedback to show commit results, revision changes, and future repo sync state once the backend exposes it.
5. Upgrade repo and secret editing ergonomics after the API semantics are ready; the baseline textareas are acceptable temporarily but are not the intended end state.
6. Continue increasing information density and scanability for services, nodes, tasks, backups, and repo views without regressing mobile usability.
7. Reuse a small set of layout and status patterns so the UI feels operational and systematic rather than ornamental.

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

Status: in progress

Goal: let the controller own Git-backed desired state changes exactly as documented.

1. Keep repo lock handling, validation, service conflict checks, and local commit creation aligned with `plan.md`.
2. Add optional remote sync behavior, push reporting, and repo sync state.
3. Add `RepoService.SyncRepo` with the documented clean-worktree requirements.
4. Extend `GetRepoHead` to return the sync-related state expected by the plan.

Deliverable:

- Desired-state file edits are safe, validated, committed, and consistent with the Git model in `plan.md`.

## Phase 5: Add Secret Handling

Status: in progress

Goal: implement the selected age-based secrets model without leaving plaintext in `controller.repo_dir`.

1. Keep controller-side decryption and re-encryption aligned with the configured age settings.
2. Keep `SecretService.GetServiceSecretEnv` and `SecretService.UpdateServiceSecretEnv` aligned with the same repo lock and conflict semantics as normal repo writes.
3. Ensure decrypted runtime files are only included in agent bundles and never persisted in the controller Git working tree.
4. Extend the remaining Git remote-sync semantics to secret writes as well.

Deliverable:

- Service secrets follow the documented age-based workflow.

## Phase 6: Replace Placeholder Backup Behavior

Status: in progress

Goal: turn the current backup scaffolding into the first real data-protection workflow.

1. Keep `data_protect` parsing and validation aligned with the documented v1 strategies.
2. Keep the currently implemented manual backup execution path limited to the documented v1 strategies.
3. Preserve one backup record per data item and keep backup query APIs stable.
4. Fill the remaining gaps around restore-side reuse, migration integration, and edge-case coverage.
5. Only add scheduling after manual backup behavior is correct.

Deliverable:

- Backup tasks produce real artifacts and real backup records.

## Phase 7: Add Read-Write Web UI on Real APIs

Status: in progress

Goal: keep the existing real controller UI, finish the missing controller interactions, and continue tightening it into a dense operations console.

1. Add the missing task action entry points on the real service, task, and node pages.
2. Add task log tailing to task detail pages.
3. Surface repo write results clearly, and later extend the same pages with repo sync state and push reporting once the backend supports them.
4. Keep repo and secret editing usable on both desktop and mobile, then improve editor ergonomics after the backend contract is complete.
5. Continue simplifying the dashboard, service, node, task, backup, repo, and secret pages into a denser and more consistent visual system.
6. Re-check all existing pages for consistent status badges, table/list density, and navigation patterns before adding more surface area.

Deliverable:

- The frontend is a real controller UI with a restrained, information-dense operational style.

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

Start with the remaining alignment work in Phases 1 through 4, then finish the missing operational UI interactions before adding more day-2 and migration surface area.

That is the smallest correct next milestone for the current codebase:

- remove architecture drift
- finish the task foundation
- make the first service actions reliable
- implement remote Git sync semantics and repo sync reporting
- finish the missing frontend task controls, log tailing, and repo/secret feedback

The initial frontend style refactor should be treated as underway, not as the primary blocker.

Do not expand DNS, scheduling, or more UI surface area until those alignment items are complete. Implement `migrate` only as the documented workflow, not as a shortcut variant.
