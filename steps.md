# Composia Implementation Steps

This document turns `plan.md` into a practical execution order for the current repository state.

## Current Baseline

- Backend is still a minimal Go entrypoint.
- Frontend is still a placeholder SvelteKit shell.
- Config examples use an older `main` model and must be aligned with the `controller` and `agent` model defined in `plan.md`.
- No internal backend packages exist yet.

## Phase 1: Bootstrap the Backend Skeleton

Goal: make `composia controller` and `composia agent` start with the v1 config model.

1. Replace the current `-role` switch with subcommands:
   - `composia controller`
   - `composia agent`
2. Add a shared config loader for `config.yaml`.
3. Implement config validation for the minimal required fields.
4. Update development config examples to the v1 `controller` and `agent` structure.
5. Add startup initialization for:
   - `state_dir`
   - `log_dir`
   - `repo_dir` existence checks where appropriate

Deliverable:

- Both roles start successfully with validated config files.

## Phase 2: Add SQLite State Storage

Goal: establish the controller runtime state foundation.

1. Create a small storage package for SQLite initialization.
2. Add schema creation and migration handling.
3. Start with the tables that unblock the first milestones:
   - `nodes`
   - `tasks`
   - `task_steps`
4. Optionally create the full v1 schema early if the migration path stays simple.
5. Add repository methods for basic node status persistence.

Deliverable:

- Controller creates and opens its SQLite database on startup.

## Phase 3: Define Minimal ConnectRPC Contracts

Goal: create the first real controller-agent protocol.

1. Add protobuf definitions for a minimal v1 API.
2. Start with only the smallest useful surface:
   - `AgentReportService.ReportHeartbeat`
   - `SystemService.GetSystemStatus`
3. Add code generation tooling and repository instructions for regenerating stubs.
4. Keep the proto set intentionally small until the first end-to-end flow works.

Deliverable:

- Generated RPC types and handlers are wired into the Go project.

## Phase 4: Implement the First End-to-End Flow

Goal: an agent can report to the controller and appear as online.

1. Implement controller RPC server startup.
2. Implement agent heartbeat loop.
3. Persist heartbeat data into `nodes`.
4. Track online status from recent heartbeat timestamps.
5. Return basic controller status from `GetSystemStatus`.

Deliverable:

- Controller and agent can run together, and the controller records live node state.

## Phase 5: Parse the Service Repository

Goal: let the controller understand declared services from Git-backed files.

1. Add a repo scanner rooted at `controller.repo_dir`.
2. Parse `composia-meta.yaml` files.
3. Validate the documented meta schema.
4. Refresh the `services` table from parsed declarations.
5. Add structured validation errors for malformed service definitions.

Deliverable:

- Controller can list declared services from the repo and persist the current service snapshot.

## Phase 6: Add the Task Model and Queue

Goal: move from direct actions to durable task execution.

1. Implement task creation in SQLite.
2. Add a single-worker persistent queue.
3. Add task status transitions:
   - `pending`
   - `running`
   - terminal states
4. Add step summaries in `task_steps`.
5. Add per-task log files under `controller.log_dir`.

Deliverable:

- Controller can create and execute queued tasks with persisted state.

## Phase 7: Ship the First Real Service Action

Goal: support a minimal `deploy` task.

1. Bind each task to a specific `repo_revision`.
2. Build a minimal service bundle from the repo.
3. Transfer the bundle to the target agent.
4. Run `docker compose up -d` on the agent.
5. Store task results and refresh service runtime state.

Deliverable:

- A declared service can be deployed to one configured agent.

## Phase 8: Add Read-Only Web UI Backed by Real Data

Goal: replace the placeholder frontend with real controller data.

1. Add service list and service detail pages.
2. Add node list and node detail pages.
3. Add task history and task detail pages.
4. Add status badges based on live controller data.
5. Keep the UI responsive on mobile from the start.

Deliverable:

- The frontend becomes an actual control-plane UI instead of a static landing page.

## Phase 9: Add Safe Repo Editing APIs

Goal: let the controller own desired state changes.

1. Add repo lock handling.
2. Add repo read APIs.
3. Add file update APIs with validation.
4. Create commits for desired-state changes.
5. Add optional remote sync behavior later.

Deliverable:

- Service-related files can be safely read and updated through the controller.

## Phase 10: Expand Runtime Features

Goal: implement the major v1 service operations after the platform foundation is stable.

1. `update`
2. `stop`
3. `restart`
4. `dns_update`
5. `caddy_reload`
6. `prune`

Deliverable:

- Core day-2 operations work through the task system.

## Phase 11: Add Secrets Support

Goal: support encrypted service secrets with the selected age-based model.

1. Add controller-side secret decryption.
2. Add controller-side secret re-encryption.
3. Add service secret read and write APIs.
4. Ensure plaintext secrets never persist in `controller.repo_dir`.
5. Include decrypted runtime files only in agent bundles.

Deliverable:

- `.secret.env.enc` is handled safely and consistently with the v1 design.

## Phase 12: Add Backups

Goal: implement the first complete data protection workflow.

1. Implement `data_protect` parsing and validation.
2. Add `backup` task execution.
3. Start with the documented v1 strategies only.
4. Persist backup records in SQLite.
5. Add backup list APIs and UI views.

Deliverable:

- Manual and scheduled backups work for the supported strategies.

## Phase 13: Add Migration

Goal: implement the most complex v1 workflow last.

1. Add `migrate` task parameters and validation.
2. Export selected data from the source node.
3. Transfer artifacts to the target node.
4. Restore data and start the target service.
5. Switch runtime ingress and DNS.
6. Persist the repo node change only after runtime cutover succeeds.
7. Use `awaiting_confirmation` exactly as defined in `plan.md`.

Deliverable:

- Service migration works with explicit operator confirmation and clear failure boundaries.

## Recommended Immediate Next Step

Start with Phases 1 through 4 only.

That sequence provides the smallest useful milestone:

- new CLI shape
- real config model
- SQLite initialization
- first controller-agent RPC
- live node heartbeat

Do not start backup, migration, DNS, or rich UI work before this milestone is complete.
