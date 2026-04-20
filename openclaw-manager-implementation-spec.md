# OpenClaw Manager — Implementation Instructions

This document defines the full implementation plan for a local control-plane webapp that manages multiple OpenClaw Docker agents from one UI. The target is a deterministic, low-footgun system that minimizes invalid states, surfaces drift clearly, and recovers safely after failures or reboots.[cite:1][cite:2][cite:3][cite:5][cite:13][cite:18]

## Product Goal

Build a single-host web application that runs on port `11436`, discovers and manages multiple Dockerized OpenClaw agents based on `ghcr.io/openclaw/openclaw`, and supports create, edit, clone, start, stop, restart, recreate, delete, backup, restore, and workspace download operations from one interface.[cite:1][cite:2]

The app must allow per-agent configuration of at least the following fields: agent name, Docker image/tag, provider, model, Telegram API key, workspace mount path, optional extra environment variables, and startup behavior.[cite:2][cite:10][cite:12]

The install path must be simple enough to bootstrap from a raw GitHub installer via `curl | bash`, and the manager itself must be automatically enabled on startup using `systemd` while managed agent containers use Docker restart policies.[cite:13][cite:18][cite:23][cite:27]

## Non-Negotiable Design Principles

Do not treat the UI as the source of truth. Persist desired state in SQLite and continuously reconcile desired state with actual Docker state so that manual edits, crashes, or daemon restarts do not silently corrupt the system.[cite:18][cite:27]

Do not execute ad hoc shell command strings for lifecycle management in the normal code path. Use the Docker Engine API or official SDK so container creation, label management, restart policies, logs, inspect data, networks, and volume bindings are structured and validated.[cite:13][cite:16][cite:19]

Do not promise “no bugs.” Engineer for safe failure: invalid configs should be blocked before apply, destructive actions must be explicit, every mutation must be auditable, and recovery paths must be deterministic.[cite:18][cite:19]

The default experience must be safe for a local admin who may be tired, distracted, or operating under pressure. Every critical action must be reversible, previewable, or clearly explained.[cite:18][cite:19]

## Recommended Stack

Use **Go** for the backend. It is the best fit here because it produces a single static binary, integrates well with `systemd`, has mature Docker SDK support, and keeps the install script simple.[cite:18][cite:27]

Use **SQLite** with WAL mode for local persistence. This keeps deployment single-node and dependency-light while still supporting transactional writes, migrations, and durable local state.[cite:18]

Use server-rendered templates with HTMX or a similarly thin client. The application is an admin control plane, not a consumer SaaS product, so reliability and debuggability matter more than a heavy SPA architecture.[cite:18]

## Host Layout

Use the following filesystem layout:

- `/opt/openclaw-manager/` — binary, static assets, versioned release payloads.
- `/etc/openclaw-manager/` — manager config file.
- `/var/lib/openclaw-manager/manager.db` — SQLite database.
- `/var/lib/openclaw-manager/agents/<agent-id>/` — manager-owned per-agent artifacts such as exported specs, backup metadata, restore staging, and audit traces.
- `/var/log/openclaw-manager/` — optional structured logs if not relying solely on journald.

Do not force workspaces under a manager-owned directory. The user explicitly needs selectable host workspace mount paths per agent, so the system must support arbitrary validated bind-mount paths.[cite:2]

## Network and Binding Rules

By default, bind the manager UI only to `127.0.0.1:11436`. Exposing a Docker-socket-capable admin app directly to the internet is an unnecessary risk.[cite:19][cite:22][cite:25]

If remote access is required, the supported model is “private network first,” for example via Tailscale or a reverse proxy with authentication and TLS. Do not make raw public exposure the default mode.[cite:19][cite:22][cite:25]

## Privilege Model

The manager will need access to `/var/run/docker.sock` or equivalent Docker Engine access in order to inspect and control containers. Treat this as effectively privileged host access and document it explicitly; a Docker socket exposure is security-sensitive and must be considered admin-only infrastructure.[cite:19][cite:22][cite:25]

If the manager is shipped as a native binary, run it under a dedicated system user such as `openclaw-manager`, but that user must still be able to access the Docker socket. If the manager is shipped as a container, avoid it for v1 unless there is a strong reason, because socket access plus arbitrary host workspace mounts make the containerized manager less clean operationally.[cite:19][cite:22]

## Core Domain Model

Create these main tables.

### agents

Required columns:

- `id` — UUID or stable slug-based ID.
- `name` — human-readable unique name.
- `slug` — filesystem-safe unique slug.
- `enabled` — boolean desired availability.
- `image_repo` — default `ghcr.io/openclaw/openclaw`.
- `image_tag` — e.g. `latest` or pinned tag.
- `provider_id` — selected provider key.
- `model_id` — selected model key.
- `telegram_api_key_encrypted` — encrypted or at minimum protected secret value.[cite:2]
- `workspace_host_path` — canonical absolute host path.
- `workspace_container_path` — default agent workspace path inside the container.
- `extra_env_json` — validated key-value map.
- `command_override_json` — optional advanced mode override.
- `restart_policy` — default `unless-stopped`.[cite:13][cite:23]
- `status_desired` — desired lifecycle state.
- `status_actual` — current observed lifecycle state.
- `drift_state` — enum such as `in_sync`, `missing_container`, `orphaned`, `invalid_mount`, `config_error`, `needs_recreate`.
- `last_error` — nullable text.
- `last_reconciled_at` — timestamp.
- `created_at`, `updated_at`, `deleted_at`.
- `spec_version` — schema version for migrations.
- `config_revision` — monotonic integer for optimistic locking.

### providers

This table should define known providers and metadata used for validation.

Columns:

- `id`
- `display_name`
- `base_url` (optional)
- `auth_type`
- `enabled`
- `supports_model_discovery`
- `created_at`, `updated_at`

Seed the system with sensible defaults, but keep provider definitions extensible because the user actively configures external providers and models for OpenClaw.[cite:2][cite:10][cite:12]

### provider_models

Columns:

- `id`
- `provider_id`
- `model_key`
- `display_name`
- `enabled`
- `sort_order`
- `metadata_json`

### backups

Columns:

- `id`
- `agent_id`
- `backup_type` (`config_only`, `workspace_only`, `full`)
- `archive_path`
- `sha256`
- `size_bytes`
- `includes_secrets`
- `created_at`
- `created_by`

### audit_log

Columns:

- `id`
- `actor`
- `action`
- `agent_id`
- `summary`
- `payload_json`
- `result`
- `created_at`

### reconciler_runs

Columns:

- `id`
- `started_at`
- `finished_at`
- `summary`
- `result`
- `details_json`

## Container Identity and Discovery

Every managed container must have these labels:

- `com.openclaw.manager=true`
- `com.openclaw.agent.id=<agent-id>`
- `com.openclaw.agent.name=<agent-name>`
- `com.openclaw.manager.spec-version=<spec-version>`

On startup, the manager must scan Docker for containers with `com.openclaw.manager=true` and reconcile them against DB state. This enables restart recovery, orphan detection, and safe import flows.[cite:18][cite:19]

Also support “import existing container” in the UI. If a container appears to be an OpenClaw instance but lacks manager labels, allow the user to inspect it and adopt it into managed state through a guided import wizard rather than forcing manual recreation.[cite:1][cite:2]

## Desired State and Reconciliation Engine

Implement a reconciler loop. The reconciler is the heart of the product and must exist before advanced UX features.

The reconciler should:

1. Load agent desired state from the database.
2. Inspect matching containers by label.
3. Compare effective config against the stored desired spec.
4. Detect drift, invalid mounts, missing images, missing containers, stopped containers, and label mismatches.
5. Decide whether the issue is fixable in place or requires a recreate.
6. Apply the smallest safe change.
7. Persist resulting observed state, errors, and timestamps.

Rules:

- Starting/stopping a container is not the same as updating config. Config changes that affect image, env, command, or mounts should mark the agent `needs_recreate` until reconciliation replaces the container safely.[cite:13][cite:16]
- A manual container deletion must not delete the agent record; it should become `missing_container` and offer “repair.”[cite:18]
- A manual container rename or relabel must not silently create duplicates; treat it as drift.[cite:18]
- Reconciliation must be idempotent. Running it twice with no real changes must do nothing.[cite:18]

## Lifecycle Semantics

Support these user actions:

- Create
- Edit
- Clone
- Start
- Stop
- Restart
- Recreate
- Delete container only
- Delete container + metadata
- Delete everything including workspace
- Backup
- Restore
- Download workspace
- View logs
- Repair drift

Lifecycle rules:

- **Create** writes the agent spec transactionally, validates it, then schedules reconciliation.
- **Edit** increments `config_revision`, writes a new desired spec, and marks the agent as `needs_recreate` or `needs_restart` depending on the diff.
- **Clone** copies an existing agent but forces a new unique name, slug, and optionally a new workspace path.
- **Delete** must be a three-path flow because users often expect data retention by default.
- **Restore** must use an explicit wizard that shows what will be overwritten.

## Validation Requirements

Do not rely on backend validation alone. Implement the same core checks in both UI and backend, with backend treated as authoritative.

### General validation

- Agent name must be unique, trimmed, length-limited, and slug-safe.
- Slug collisions must be blocked.
- Image repo defaults to `ghcr.io/openclaw/openclaw`, and changing it belongs in advanced mode only.[cite:1]
- Image tag must be non-empty and normalized.
- Restart policy defaults to `unless-stopped`.[cite:13][cite:23]

### Path validation

- Resolve `realpath` and store the canonical path.
- Path must be absolute.
- Path must exist or be explicitly created by the manager.
- Path must be writable by the container use case.
- Refuse dangerous roots by default, including `/`, `/etc`, `/var/run`, `/root`, and broad home directories unless an unsafe override is explicitly enabled.
- Warn if the same writable workspace path is already assigned to another agent.
- Warn if the target is on a network mount or low-space filesystem.

### Provider/model validation

- Selected provider must exist and be enabled.
- Selected model must belong to the selected provider unless custom mode is enabled.
- Custom model entry must still be syntax-checked.
- If a provider requires auth, enforce presence of the secret before save.

### Telegram key validation

- Non-empty format check on the client and server.
- Optional “Test token” action before save.
- Never display the full token after initial entry; show masked form.

### Docker validation

- Docker daemon must be reachable before any apply path runs.
- Preflight image pull optional in create flow.
- Port and mount conflicts must be detected before container creation.

### Disk validation

- Refuse backup or restore if free space is below a configurable threshold.
- Display archive size estimates when possible.

## UI Requirements

Keep the UI compact, readable, and operationally obvious. This is a control-plane, not a marketing page.

### Main dashboard

For each agent card or table row, show:

- Name
- Desired state
- Actual state
- Drift state
- Image tag
- Provider
- Model
- Workspace host path
- Restart policy
- Last backup time
- Last reconcile result
- Quick actions: Start, Stop, Restart, Edit, Backup, Logs

Provide filtering for:

- Running
- Stopped
- Drifted
- Needs recreate
- Error

### Create/edit wizard

Use a multi-step form:

1. Basics — name, image tag, enable on boot.
2. Workspace — host path, container path, ownership checks.
3. Provider — provider select, model select, advanced custom model.
4. Secrets — Telegram API key and other provider credentials.
5. Review — render the effective container spec before apply.

Do not expose raw JSON by default. Add an “advanced” accordion for extra env vars, command overrides, labels, and low-level settings.

### Repair UX

When drift is detected, show a dedicated repair panel explaining the problem in plain language and offering the smallest safe fix. Example: “Container missing, DB record still exists. Repair will recreate the container using the saved spec.”[cite:18]

### Deletion UX

Deletion must be explicit and data-aware:

- Delete container only
- Delete container + metadata
- Delete everything including workspace contents

Only the last option should require a typed confirmation phrase. The UI must clearly state what data will remain.[cite:18]

### Logs view

Support live tail and recent logs from the container through the Docker API. Default to the last 500 lines with search and copy support.[cite:16]

### First-run wizard

If the database is empty, show a startup wizard that:

- Verifies Docker reachability.
- Verifies manager permissions.
- Checks socket access.
- Confirms the UI bind address.
- Explains startup behavior.
- Creates the first agent.

## Backup and Restore Design

Backups must be implemented early, not as a late feature. This product edits live agent environments, so recovery is core functionality.

Support these backup types:

- **Config only** — exported agent spec, secret metadata markers, no workspace.
- **Workspace only** — tar.gz archive of the mounted workspace path.
- **Full** — config export + workspace archive + manifest.

Backup rules:

- Stream archives; do not buffer full tarballs in RAM.
- Compute SHA-256 for every archive.
- Store archive metadata in the database.
- Provide download from the UI.
- Allow optional secret exclusion so exported configs can be shared safely.
- Redact secrets in manifests by default.

Restore rules:

- Restore is always wizard-driven.
- Show detected agent ID, path, size, checksum, and overwrite targets.
- Allow restore into a new agent instead of only in-place restore.
- Refuse automatic restore over a running mismatched container without explicit confirmation.

Workspace download is separate from backup. The user explicitly wants workspace download of all agents, so support one-click streaming of the current live workspace as an archive without requiring a DB backup record.[cite:1]

## Secrets Handling

Secrets include Telegram API keys and provider keys. The user actively configures external providers and Telegram-based OpenClaw integrations, so secret handling is not optional.[cite:2][cite:10][cite:12]

Rules:

- Encrypt at rest if feasible; if not, lock down permissions and clearly document the tradeoff.
- Never write secrets to regular logs.
- Never echo secrets in the UI after initial submit.
- When exporting configs, omit secrets by default or replace with placeholders.
- If “include secrets” is requested in backup/export, force explicit confirmation.
- Use structured redaction for all error paths.

## Startup and Recovery Behavior

There are two separate startup concerns and both must be implemented.

### Manager startup

Install the manager as a `systemd` service and enable it on boot using `systemctl enable --now openclaw-manager.service`. Use restart settings such as `Restart=on-failure` and `RestartSec=5` so transient failures recover while repeated crash loops are controlled by `StartLimitIntervalSec` and `StartLimitBurst`.[cite:18][cite:27]

Recommended unit shape:

```ini
[Unit]
Description=OpenClaw Manager
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
User=openclaw-manager
Group=openclaw-manager
ExecStart=/opt/openclaw-manager/openclaw-manager --config /etc/openclaw-manager/config.yaml
WorkingDirectory=/opt/openclaw-manager
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/openclaw-manager /etc/openclaw-manager

[Install]
WantedBy=multi-user.target
```

Validate any hardening directives against actual Docker socket and bind-mount requirements during implementation. Over-hardening can break real operation.[cite:18][cite:27]

### Agent container startup

Managed agent containers must use Docker restart policy `unless-stopped` by default. This preserves the expectation that a manually stopped agent stays stopped across Docker daemon restarts better than `always`.[cite:13][cite:23]

On manager startup, immediately run reconciliation so the UI reflects reality after reboot, daemon restart, or crash.[cite:18]

## Docker Container Spec

Generate container specs from structured data, not string concatenation.

Minimum effective spec elements:

- Image: `ghcr.io/openclaw/openclaw:<tag>` by default.[cite:1]
- Restart policy: `unless-stopped`.[cite:13][cite:23]
- Bind mount: validated workspace host path to configured in-container workspace path.
- Environment variables: provider/model/auth/runtime config.
- Labels: manager ownership + agent identity.
- Optional command override only in advanced mode.

Before container creation:

- Pull image if needed.
- Validate mount source exists or create it if safe.
- Validate env set completeness.
- Detect conflicting existing managed container for the same agent ID.

When recreating a container:

1. Save desired spec revision.
2. Pull image if needed.
3. Create replacement container with correct labels and spec.
4. Stop/remove old container when safe.
5. Start replacement.
6. Verify running state and health if available.
7. Commit observed state in DB.

If creation succeeds but start fails, preserve enough metadata for recovery and surface the exact failure reason.[cite:13][cite:16][cite:18]

## API Surface

Implement an internal HTTP API even if the UI is server-rendered. This keeps the code organized and enables automation later.

Suggested routes:

- `GET /healthz`
- `GET /api/agents`
- `POST /api/agents`
- `GET /api/agents/:id`
- `PATCH /api/agents/:id`
- `POST /api/agents/:id/clone`
- `POST /api/agents/:id/start`
- `POST /api/agents/:id/stop`
- `POST /api/agents/:id/restart`
- `POST /api/agents/:id/recreate`
- `POST /api/agents/:id/backup`
- `POST /api/agents/:id/restore`
- `GET /api/agents/:id/workspace/download`
- `GET /api/agents/:id/logs`
- `POST /api/agents/:id/repair`
- `DELETE /api/agents/:id?mode=container|metadata|full`
- `POST /api/reconcile`
- `GET /api/providers`
- `GET /api/providers/:id/models`
- `POST /api/validate/path`
- `POST /api/validate/token`

Use clear response models with machine-readable error codes. Do not force the UI to parse human log messages.

## Installer Requirements

The project must support a simple bootstrap flow from a raw GitHub script via `curl | bash`. The installer should do the following:

1. Detect OS and architecture.
2. Verify Docker is installed; if not, instruct or optionally install depending on target distro policy.
3. Create system user and group `openclaw-manager` if absent.
4. Create `/opt/openclaw-manager`, `/etc/openclaw-manager`, and `/var/lib/openclaw-manager`.
5. Download the correct release artifact from GitHub Releases.
6. Install the binary.
7. Write default config.
8. Write the `systemd` unit.
9. Reload `systemd`.
10. Enable and start the service.
11. Wait for health check on `127.0.0.1:11436/healthz`.
12. Print the final access URL and next steps.

Also provide:

- `upgrade.sh`
- `uninstall.sh`
- checksum verification for release downloads

The installer must be idempotent. Re-running it should upgrade or repair rather than corrupt the install.[cite:18][cite:27]

## Example Default Config

Provide a config file such as `/etc/openclaw-manager/config.yaml`:

```yaml
server:
  host: 127.0.0.1
  port: 11436

database:
  path: /var/lib/openclaw-manager/manager.db

security:
  require_auth: false
  secret_key_file: /etc/openclaw-manager/secret.key

paths:
  data_dir: /var/lib/openclaw-manager
  backup_dir: /var/lib/openclaw-manager/backups

reconcile:
  interval_seconds: 30

agents:
  default_image_repo: ghcr.io/openclaw/openclaw
  default_restart_policy: unless-stopped
  default_workspace_container_path: /workspace
```

If authentication is added later, make it simple local-admin auth, not a heavyweight identity system for v1.[cite:19]

## Security Requirements

Because the manager controls Docker, security mistakes here have host-level implications.[cite:19][cite:22][cite:25]

Required controls:

- Default bind to localhost only.[cite:19][cite:22][cite:25]
- CSRF protection for state-changing UI actions.
- SameSite cookies if auth is enabled.
- Strict input validation for all filesystem paths, env keys, and command overrides.
- Redacted logs.
- Secret masking in UI and exports.
- Audit log for all destructive or secret-affecting actions.
- No public Docker TCP exposure unless the operator explicitly configures TLS and understands the risk.[cite:19][cite:22][cite:25]

Do not attempt to hide the risk of Docker socket access. Document it plainly in both README and UI setup screens.[cite:19][cite:22][cite:25]

## Observability

Implement structured logs and audit logs from day one.

Structured application logs should include:

- request ID
- agent ID
- action
- reconcile decision
- Docker API error code
- duration

Provide a “recent events” view in the UI using the audit log table. Operators need a timeline of what changed and why.[cite:18]

Expose at least:

- `/healthz` — process healthy.
- `/readyz` — database reachable, Docker reachable.
- optional metrics endpoint if easy.

## Concurrency and Consistency

Handle multi-tab and background race conditions explicitly.

Rules:

- Use optimistic locking with `config_revision` on edit/update forms.
- Reject stale form submissions with a clear “config changed since load” message.
- Serialize lifecycle mutations per agent using DB locks or in-memory keyed mutexes.
- Prevent concurrent backup and restore against the same agent.
- Do not let a reconcile loop and a manual recreate conflict; one owner at a time.

## Testing Strategy

Build the test matrix before feature completion.

### Unit tests

Cover:

- slug generation
- path validation
- provider/model validation
- secret redaction
- diff classification (`restart` vs `recreate`)
- backup manifest generation
- destructive action routing

### Integration tests

Use disposable Docker containers in CI where possible.

Cover:

- create agent
- edit agent requiring recreate
- start/stop/restart
- drift after manual container deletion
- drift after manual container stop
- backup/download flow
- restore into new agent
- installer idempotency
- reboot/startup reconciliation simulation

### Failure tests

Cover explicitly:

- Docker unavailable at app start
- workspace path missing after reboot
- disk full during backup
- invalid model/provider selection
- permission denied on workspace path
- corrupted DB record migration
- container start failure after successful create

### UI tests

At minimum test:

- first-run wizard
- create wizard validation states
- deletion confirmation states
- repair flow
- backup and workspace download buttons

## Release Engineering

For each release:

- Build static binaries for target architectures.
- Publish GitHub Releases with checksums.
- Keep the raw installer pinned to the latest stable release channel by default.
- Support rollback to the previous binary.
- Run DB migrations on startup with backup-before-migrate behavior.

Do not auto-migrate silently without a DB backup checkpoint.

## Proposed Implementation Order

Implement in this sequence.

### Phase 1 — skeleton

- Repository scaffold.
- Config loading.
- SQLite init + migrations.
- Docker connectivity test.
- `systemd` service files.
- `/healthz` and minimal UI shell.

### Phase 2 — domain and reconciliation

- Agent schema.
- Provider/model schema.
- Reconciler.
- Container discovery by labels.
- Drift states.
- Audit log.

### Phase 3 — CRUD and lifecycle

- Create/edit/clone flows.
- Start/stop/restart/recreate.
- Delete variants.
- Logs view.
- First-run wizard.

### Phase 4 — backup and restore

- Config export.
- Workspace tar streaming.
- Backup DB table.
- Restore wizard.
- Workspace download endpoint.

### Phase 5 — installer and startup polish

- Raw GitHub installer.
- upgrade/uninstall scripts.
- startup validation UI.
- health/readiness checks.
- release packaging.

### Phase 6 — hardening

- auth optionality.
- CSRF.
- secret encryption.
- race-condition cleanup.
- fault injection tests.

## Explicit Anti-Patterns

Do not do the following:

- Do not shell out to `docker` CLI for normal operations when SDK access is available.[cite:13][cite:16][cite:19]
- Do not store the only source of truth in container names or folder names.
- Do not let users edit raw container JSON in the main form.
- Do not delete workspaces implicitly when deleting agents.
- Do not log tokens, provider keys, or full env dumps.[cite:19][cite:25]
- Do not bind the manager to `0.0.0.0` by default.[cite:19][cite:22][cite:25]
- Do not use `restart=always` as the default for managed agents when the intended admin semantics are “stay down if manually stopped.”[cite:13][cite:23]
- Do not skip reconciliation on startup.[cite:18]

## Acceptance Criteria

The implementation is considered complete only when all of the following are true:

- A fresh host can install the manager from a raw GitHub `curl | bash` command and reach the UI on port `11436` after installation.[cite:18][cite:27]
- The manager starts automatically on reboot through `systemd`.[cite:18][cite:27]
- Managed OpenClaw agent containers restart according to Docker `unless-stopped` semantics.[cite:13][cite:23]
- The UI can create, edit, clone, delete, backup, restore, repair, and download workspaces for multiple agents from one place.[cite:1][cite:2]
- Invalid configs are blocked before apply.
- Manual Docker drift is detected and clearly repairable.
- Secrets are masked and excluded from exports by default.[cite:19][cite:25]
- Destructive actions are explicit and data-aware.
- Re-running the installer repairs or upgrades safely.

## Final Guidance to the Implementing Agent

Bias toward boring, inspectable infrastructure. A smaller, deterministic control plane is better than a feature-rich but ambiguous one.[cite:18][cite:19]

Prioritize the reconciler, backup/restore, validation, and startup behavior before polishing visuals. Those are the parts that decide whether this tool is genuinely useful on a real machine.[cite:13][cite:18][cite:27]

When tradeoffs appear, prefer: structured Docker API over shelling out, explicit drift over silent magic, localhost-only over public exposure, `unless-stopped` over `always`, and reversible operations over convenience shortcuts.[cite:13][cite:18][cite:19][cite:22][cite:23][cite:25]
