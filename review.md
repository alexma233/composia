# Comprehensive Security / Code Review Report

## Executive Summary

This final version incorporates the explicit product and deployment assumptions clarified after the initial review:

1. TLS may be terminated by a reverse proxy such as Caddy.
2. The project currently uses a single-admin trust model and does not implement multi-user authorization or RBAC.
3. The admin UI intentionally exposes decrypted secrets, raw logs, Docker inspect output, internal paths, and detailed errors to the administrator.
4. The checked-in development defaults are for local non-secret development only.

After applying those assumptions, the remaining actionable findings are:

- **2 High**
- **3 Medium**
- **4 Low**

Most important remaining issues:

- The container exec WebSocket is effectively protected only by a leaked `session_id`, with no handshake authentication and no origin check.
- Backup and restore metadata can drive arbitrary host path reads and destructive writes on agents.
- `ReportServiceInstanceStatus` does not bind `node_id` to the authenticated agent token.
- Backup and restore runtime payloads are built from live HEAD instead of the queued task revision.
- Task serialization remains broader than necessary in one place.

## Methodology

This review used a parallel subagent approach with **7 security-focused subagents**, each auditing the full tracked repository while concentrating on a different risk domain:

1. Authentication / authorization / sessions
2. Secret handling / storage / privacy
3. Network / RPC / WebSocket / transport security
4. Input validation / injection / path safety
5. Dependency / CI/CD / supply-chain risk
6. Logging / error handling / information disclosure
7. Business logic / concurrency / state integrity

The review covered the tracked repository surfaced by `git ls-files`, with emphasis on:

- Go backend:
  - `cmd/composia/main.go`
  - `internal/controller/*.go`
  - `internal/agent/*.go`
  - `internal/repo/*.go`
  - `internal/store/*.go`
  - `internal/config/config.go`
  - `internal/secret/age.go`
  - `internal/rpcutil/auth.go`
- Web app:
  - `web/src/hooks.server.ts`
  - `web/src/lib/server/*.ts`
  - `web/src/routes/**/*.ts`
  - relevant `.svelte` files that render admin-visible data
- Supply chain / deployment:
  - `web/package.json`
  - `go.mod`, `go.sum`
  - `Dockerfile`, `web/Dockerfile`
  - `.forgejo/workflows/*.yaml`
  - `docker-compose.yaml`
  - `dev/docker-compose.dev.yaml`
  - `scripts/docs/generate-proto-docs.sh`
- Documentation and examples that materially affect security posture:
  - `docs/content/en-us/**`
  - `dev/.env.example`

This report is intentionally narrower than the first draft. Findings that depend on a multi-user permission model, or on hiding admin-facing operational data from the admin, were removed and are now treated as accepted design assumptions.

## Accepted Design Assumptions

The following are treated as current design choices rather than security defects:

1. **TLS is expected at the reverse-proxy or trusted network boundary.**
   The controller may still run plain HTTP/h2c internally, but deployments should make that boundary explicit.

2. **All logged-in users and all enabled access tokens are full administrators.**
   There is currently no RBAC, no read-only role, and no privilege separation between admin actions.

3. **The admin console intentionally shows sensitive operational detail.**
   This includes decrypted secrets, raw task logs, Docker inspect output, controller paths, and detailed upstream errors.

4. **Development defaults are non-secret samples.**
   The checked-in dev credentials and session values are not treated as production secrets.

## Detailed Findings

## High

### 1. Container exec WebSocket attach is unauthenticated and origin-blind

- Source: Subagents 1, 3, 4
- Severity: High
- Confidence: High
- Locations:
  - `internal/controller/run.go:124-125`
  - `internal/controller/exec_tunnel.go:77-84`
  - `internal/controller/exec_tunnel.go:190-205`
  - `internal/controller/exec_tunnel.go:229-246`
  - `internal/controller/exec_tunnel.go:394-412`
  - `web/src/routes/nodes/[id]/docker/containers/[cid]/exec/+server.ts:19-30`
  - `web/src/routes/nodes/[id]/docker/containers/[cid]/+page.svelte:217-255`

**Issue**

The browser attaches to `/ws/container-exec/{sessionId}`. That endpoint:

- is not wrapped by the bearer auth interceptor
- does not validate a web session or bearer token during the WebSocket handshake
- uses `CheckOrigin: func(*http.Request) bool { return true }`
- effectively treats possession of `session_id` as sufficient authority

**Why it matters**

Even in a single-admin system, an interactive container shell is one of the highest-value capabilities in the product. A leaked `session_id` is enough to hijack the session.

**Attack path / failure mode**

1. An admin opens a terminal session.
2. `OpenContainerExec` returns `sessionId` and `websocketPath`.
3. If the `sessionId` is exposed through network sniffing, browser tooling, proxy logs, extensions, or any future UI/XSS bug, an attacker can attach directly.
4. Because `browserTaken` is first-come-first-served, the attacker can preempt the real operator.

**Recommendation**

- Require authentication on the WebSocket handshake.
- Bind exec sessions to the authenticated principal that created them.
- Replace bare `session_id` URLs with a short-lived, signed, one-time attach token.
- Restrict allowed origins.
- Add idle timeout and expiry semantics.

## 2. Backup/restore path handling allows arbitrary host filesystem reads and destructive writes

- Source: Subagents 2, 7
- Severity: High
- Confidence: High
- Locations:
  - `internal/repo/services.go:597-626`
  - `internal/agent/run.go:1225-1257`
  - `internal/agent/run.go:1311-1324`

**Issue**

`data_protect.data[].backup.include` and `.restore.include` are validated only for presence and strategy type. At execution time, absolute paths and `../`-style paths are accepted.

Key behavior:

- `stageInclude()` accepts absolute paths, `./...`, and `../...`
- `restoreInclude()` resolves the same forms back to host paths
- `replacePath()` calls `os.RemoveAll(targetPath)` before restoring

**Why it matters**

Repository metadata becomes a host filesystem capability. A malicious repo change or compromised synced commit can cause:

- exfiltration of arbitrary host files into backup artifacts
- overwrite or removal of arbitrary host paths during restore
- compromise far beyond the intended service workspace boundary

**Attack path / failure mode**

- A service definition includes `/etc`, `/root/.ssh`, or `../other-service`.
- Backup copies host data into staging.
- Restore removes and rewrites host paths outside the service root.

**Recommendation**

- Reject absolute paths and `..` segments in backup and restore includes.
- Limit includes to service-root-relative paths or explicitly declared volume identifiers.
- Enforce these constraints both:
  - at repo validation time
  - at agent runtime before copy and remove operations
- Never call `RemoveAll` on paths outside approved roots.

## Medium

### 3. `ReportServiceInstanceStatus` did not bind `node_id` to the authenticated agent token

- Source: Subagent 1
- Severity: Medium
- Confidence: High
- Status: Fixed in working tree
- Locations:
  - `internal/controller/run.go:652-672`
  - comparison references:
    - `internal/controller/run.go:453-464`
    - `internal/controller/run.go:494-497`

**Issue**

Unlike `Heartbeat` and `ReportDockerStats`, `ReportServiceInstanceStatus` validated fields but did **not** verify that the bearer-authenticated node matches `req.Msg.GetNodeId()`.

**Fix status**

The controller now applies the same bearer-subject and `node_id` equality check used by `Heartbeat` and `ReportDockerStats`, and includes a regression test that rejects mismatched node reporting.

**Why it matters**

Any valid agent token can report runtime status on behalf of another node.

**Attack path / failure mode**

- A compromised node token is used to call `ReportServiceInstanceStatus`.
- The caller submits status for a different `node_id`.
- The controller records false runtime state, misleading operators and automation.

**Recommendation**

- Apply the same bearer-subject and node-ID equality check used in heartbeat and Docker stats reporting.
- Audit all agent-facing report methods for consistent identity binding.

### 4. Backup and restore runtime payloads are built from live HEAD, not the queued task revision

- Source: Subagent 7
- Severity: Medium
- Confidence: High
- Locations:
  - `internal/controller/run.go:836-876`
  - `internal/controller/run.go:879-921`
  - related correct pattern:
    - `internal/controller/migrate.go:132-135`
    - `internal/controller/run.go:572-579`

**Issue**

`buildBackupRuntimePayload()` and `buildRestoreRuntimePayload()` accept a `revision` parameter but load service definitions from live repo state via `repo.FindService(...)`, not the pinned task revision.

**Why it matters**

This creates a time-of-check/time-of-use gap. A queued task can execute with runtime instructions that differ from the revision recorded on the task.

**Attack path / failure mode**

1. A task is created at revision A.
2. The repo changes to revision B before execution.
3. Backup or restore metadata from B drives the runtime behavior, even though the task claims revision A.

This is especially dangerous combined with unrestricted include paths.

**Recommendation**

- Build all backup and restore runtime payloads from the task's immutable revision.
- Use revision-aware service lookup consistently.
- Add a revision-aware helper for the rustic infra service, analogous to `FindServiceAtRevision(...)`.
- Build the runtime payload from the task's saved `service_dir` and `repo_revision`, not from live HEAD.
- Add a regression test that queues a task, mutates HEAD, and verifies the task still uses the original revision.

### 5. Destructive service-task admission checks are non-atomic

- Source: Subagent 7
- Severity: Medium
- Confidence: High
- Status: Fixed in working tree
- Locations:
  - `internal/controller/run.go:1668-1739`
  - `internal/controller/migrate.go:290-313`
  - `internal/store/tasks.go:31-90`
  - `internal/store/tasks.go:392-408`

**Issue**

The code checks `HasActiveServiceInstanceTask(...)` and then inserts a task with `CreateTask(...)`, but there is no transaction or uniqueness constraint enforcing exclusivity.

**Fix status**

Destructive task admission now goes through store-level transactional create helpers that:

- check service or service-instance activity inside the same database transaction used to insert the task
- keep multi-node service actions all-or-nothing instead of partially enqueuing per-node tasks
- preserve migrate-time service and source/target instance exclusivity in one atomic admission path
- add concurrency regression tests that race task creation and verify only one task is admitted

**Why it matters**

Concurrent requests can race and both enqueue destructive tasks for the same service instance.

**Attack path / failure mode**

- Two clients issue stop, deploy, restore, backup, or migrate-related operations at nearly the same time.
- Both see "no active task".
- Both insert pending tasks.
- Service state, restore flow, or migration correctness can be corrupted.

**Recommendation**

- Enforce lock and exclusivity at the database layer.
- Use a transaction or a lock table keyed by `(service_name, node_id)`.
- Add concurrency tests that intentionally race task creation.

### 6. Global single-running-task serialization is broader than necessary

- Source: Subagents 3, 7
- Severity: Medium
- Confidence: High
- Locations:
  - `internal/store/tasks.go:113-150`
  - `internal/store/tasks.go:199-207`
  - related tests:
    - `internal/store/tasks_test.go:44-76`

**Issue**

`ensureNoRunningTask()` blocks claiming new pending tasks whenever **any** task is already running.

**Why it matters**

This appears to be an intentional safety choice, but the scope is broader than necessary. One long-running task on one node can block unrelated work on other nodes and controller-side tasks.

**Failure mode**

- A slow task remains `running`.
- New restore, DNS, maintenance, or operational tasks are not claimable even when they target different nodes or resources.

**Recommendation**

If you want to reduce the lock scope without losing safety, two practical options are:

1. **Minimal change**: remove `ensureNoRunningTask()` from `ClaimNextPendingTaskForNode()` and `ClaimNextPendingTaskOfType()`, then rely on:
   - one agent execution loop per node
   - existing `HasActiveServiceInstanceTask(...)` checks
   - the single controller worker

2. **Explicit scoped locking**: introduce lock keys such as:
   - `service:<service>:<node>`
   - `service-migrate:<service>`
   - `controller:dns`
   - `node:<node>:docker`

The second approach is cleaner long-term, but the first is a valid small step.

## Low

### 7. Cleartext controller URLs should surface an explicit operator warning

- Source: Subagents 1, 2, 3
- Severity: Low
- Confidence: High
- Locations:
  - `internal/controller/run.go:127-145`
  - `internal/agent/run.go:2010-2020`
  - `docs/content/en-us/guide/configuration/controller.md:66-82`
  - `docs/content/en-us/guide/configuration/agent.md:24-33`
  - `dev/.env.example:12-18`

**Issue**

The project can intentionally run behind a reverse proxy such as Caddy, so plain internal HTTP is not automatically a defect. However, the current product does not surface a strong warning when `http://` controller links are used.

**Why it matters**

Operators may incorrectly assume the link is already protected, even when bearer tokens are still crossing an untrusted segment in cleartext.

**Recommendation**

- Emit a startup warning when controller URLs use `http://`.
- Surface the same warning in the web settings page.
- Document more explicitly that plain HTTP is acceptable only behind a trusted boundary or local reverse proxy.

### 8. The `readonly` access token example is misleading documentation

- Source: Subagent 1
- Severity: Low
- Confidence: High
- Status: Fixed in docs
- Locations:
  - `docs/content/en-us/guide/configuration/controller.md:18-25`
  - `internal/config/config.go:309-317`
  - `internal/controller/run.go:116-122`
  - `internal/controller/run.go:172-250`

**Issue**

The documentation shows a token named `readonly`, but token names do not affect permissions. All enabled access tokens are currently full-admin tokens.

**Why it matters**

This is not an implementation bug, but it can mislead operators into thinking they have a low-privilege token when they do not.

**Recommendation**

- Remove `readonly` from the example.
- Replace it with a neutral label such as `web-ui` or `automation`.
- Add a sentence stating that Composia currently has no RBAC and all enabled access tokens grant full controller access.

### 9. Duplicate token values silently override earlier identities

- Source: Subagent 3
- Severity: Low
- Confidence: High
- Status: Fixed in working tree
- Locations:
  - `internal/config/config.go:174-185`
  - `internal/config/config.go:218-225`
  - `internal/config/config.go:301-317`

**Issue**

Config validation enforces unique node IDs, but not unique token values. `NodeTokenMap()` and `EnabledAccessTokenMap()` simply assign into a map, so later duplicates overwrite earlier ones.

**Fix status**

The config loader now rejects duplicate node tokens, duplicate access tokens, and node-token/access-token collisions during controller config validation. Regression tests cover all three cases.

**Why it matters**

This can cause identity confusion, misattribution, and accidental token sharing.

**Recommendation**

- Reject duplicate node tokens.
- Reject duplicate access tokens.
- Reject node-token and access-token collisions.

### 10. Supply-chain pinning can be tightened

- Source: Subagent 5
- Severity: Low
- Confidence: High
- Locations:
  - `web/package.json:18-36`
  - `.forgejo/workflows/release-build-docker.yaml:21-25`
  - `.forgejo/workflows/release-build-docker.yaml:28-43`
  - `.forgejo/workflows/release-build-docker.yaml:61-119`
  - `scripts/docs/generate-proto-docs.sh:16-19`

**Issue**

The web toolchain declares many dependencies as `latest`, CI actions are referenced by mutable tags, and the docs generator installs `protoc-gen-doc@latest`.

**Why it matters**

This is primarily a reproducibility and supply-chain hygiene issue rather than an immediate exploit path in the application itself.

**Recommendation**

- Pin exact versions or bounded semver ranges in `web/package.json`.
- Pin external CI actions by immutable commit SHA.
- Pin generator tools to known-good versions.

## Recommendations

### Priority 1

1. Redesign exec WebSocket attach authentication and origin validation.
2. Constrain backup and restore includes to safe roots.
3. Bind `ReportServiceInstanceStatus` to the authenticated node.
4. Build backup and restore runtime payloads strictly from the queued task revision.

### Priority 2

1. Reduce task serialization scope, either by removing the global running gate or replacing it with scoped lock keys.
2. Add clear operator warnings for `http://` controller links.

### Priority 3

1. Fix the misleading `readonly` token example in the docs.
2. Reject duplicate token values at config load.
3. Tighten dependency, workflow, and generator pinning.

## Overall Risk Assessment & Best Practices

**Overall risk: Medium**

Within the stated single-admin trust model, most of the earlier "sensitive data exposure" findings are accepted design choices rather than defects. The remaining risk is concentrated in:

- cross-boundary trust for container exec
- overly powerful backup and restore path handling
- agent identity binding
- task revision integrity
- task orchestration correctness

The repository still shows several positive engineering properties:

- path traversal protections in normal repo file APIs
- no obvious shell-string command execution pattern
- parameterized SQL usage
- signed web session cookies
- archive extraction containment checks
- multiple agent RPCs already bind bearer subject to node or task correctly

## Conclusion

After aligning the report with the current product boundary, the most important issues to address first are:

1. **Unauthenticated exec WebSocket attachment**
2. **Arbitrary host path backup and restore behavior**
3. **Missing node binding in `ReportServiceInstanceStatus`**
4. **Backup and restore runtime payload revision drift**
5. **Overly broad task serialization**

The rest of the removed findings are best understood as product assumptions or documentation and operational clarity issues, not core code vulnerabilities under the current single-admin model.
