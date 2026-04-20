# OpenClaw Manager

A local control-plane webapp for managing multiple Dockerized OpenClaw agents from one UI.

## Features

- **Multi-Agent Management** - Create, edit, clone, start, stop, restart, recreate, and delete OpenClaw agents
- **Reconciliation Engine** - Automatically detects and repairs Docker drift
- **WebSocket Terminal** - Interactive bash access to each agent container
- **Backup & Restore** - Config-only, workspace-only, and full backup types with streaming archives
- **Audit Logging** - All actions logged with timestamps and actor information
- **Security Hardened** - Rate limiting, security headers, input sanitization, secret masking
- **Simple Installation** - `curl | bash` installer with systemd integration

## Requirements

- Linux (amd64/arm64)
- Docker running and accessible
- Root or sudo for installation

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/openclaw/manager/main/install.sh | bash
```

The installer will:
1. Create the `openclaw-manager` system user and group
2. Install the binary to `/opt/openclaw-manager/`
3. Create configuration at `/etc/openclaw-manager/config.yaml`
4. Install the systemd service
5. Start and verify the service

After installation, access the UI at **http://127.0.0.1:11436**

## Manual Installation

### Build from Source

```bash
git clone https://github.com/openclaw/manager.git
cd manager
make build
sudo make install
```

### Directory Structure

```
/opt/openclaw-manager/          # Binary and scripts
/etc/openclaw-manager/            # Configuration
/var/lib/openclaw-manager/        # Database and data
│   ├── manager.db               # SQLite database
│   ├── backups/                 # Backup archives
│   ├── workspaces/              # Downloaded workspaces
│   └── agents/                  # Per-agent artifacts
```

## Configuration

Edit `/etc/openclaw-manager/config.yaml`:

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

## Usage

### Service Management

```bash
# Check status
systemctl status openclaw-manager

# View logs
journalctl -u openclaw-manager -f

# Restart
systemctl restart openclaw-manager

# Stop
systemctl stop openclaw-manager
```

### Upgrading

```bash
# Upgrade to latest
sudo /opt/openclaw-manager/upgrade.sh

# Upgrade to specific version
sudo /opt/openclaw-manager/upgrade.sh v1.2.3
```

### Uninstalling

```bash
# Remove everything
sudo /opt/openclaw-manager/uninstall.sh

# Keep data (config and database)
sudo /opt/openclaw-manager/uninstall.sh --keep-data
```

## API Reference

### Health & Status

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Simple health check |
| `GET /readyz` | Readiness check (DB + Docker) |
| `GET /version` | Version information |

### Agents

| Endpoint | Description |
|----------|-------------|
| `GET /api/agents` | List all agents |
| `POST /api/agents` | Create agent |
| `GET /api/agents/:id` | Get agent details |
| `PATCH /api/agents/:id` | Update agent |
| `DELETE /api/agents/:id?mode=container\|metadata\|full` | Delete agent |

### Agent Lifecycle

| Endpoint | Description |
|----------|-------------|
| `POST /api/agents/:id/start` | Start container |
| `POST /api/agents/:id/stop` | Stop container |
| `POST /api/agents/:id/restart` | Restart container |
| `POST /api/agents/:id/recreate` | Recreate container |
| `POST /api/agents/:id/clone` | Clone agent |
| `POST /api/agents/:id/repair` | Repair drift |
| `POST /api/agents/:id/backup` | Create backup |
| `POST /api/agents/:id/restore` | Restore from backup |
| `GET /api/agents/:id/logs` | Get container logs |
| `GET /api/agents/:id/workspace/download` | Download workspace |
| `WS /api/agents/:id/terminal` | WebSocket terminal |

### Providers & Models

| Endpoint | Description |
|----------|-------------|
| `GET /api/providers` | List providers |
| `GET /api/providers/:id/models` | List models for provider |

### Validation

| Endpoint | Description |
|----------|-------------|
| `POST /api/validate/path` | Validate workspace path |
| `POST /api/validate/token` | Validate API token |

### Audit

| Endpoint | Description |
|----------|-------------|
| `GET /api/audit` | Recent audit log entries |
| `POST /api/reconcile` | Trigger reconciliation |

## Agent Configuration

When creating an agent, specify:

- **Name** - Unique display name
- **Image Tag** - Docker image tag (default: `latest`)
- **Provider** - AI provider (OpenAI, Anthropic, Google, Ollama, OpenRouter, Custom)
- **Model** - Model ID for the provider
- **Workspace Path** - Host path for agent workspace
- **Restart Policy** - `always`, `unless-stopped`, or `no`

## WebSocket Terminal

Connect to an agent's terminal via WebSocket:

```javascript
const ws = new WebSocket('ws://localhost:11436/api/agents/{id}/terminal');
ws.onmessage = (e) => console.log(e.data);
ws.send('ls\n');
```

## Backup Types

- **config_only** - Agent spec without workspace
- **workspace_only** - Only the workspace directory
- **full** - Both config and workspace

## Deletion Modes

| Mode | Behavior |
|------|----------|
| `container` | Remove container only, keep DB record |
| `metadata` | Remove DB record, keep workspace |
| `full` | Remove everything including workspace |

## Security

- Default bind to localhost only
- Rate limiting (100 req/min per IP)
- Security headers on all responses
- Input sanitization
- Secret hashing for storage
- Secret masking for display
- Audit logging for all mutations

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     OpenClaw Manager                     │
├─────────────────────────────────────────────────────────┤
│  Handlers        │  Reconciler  │  Terminal  │  Audit    │
│  - API routes   │  - 30s loop │  - WS exec │  - Logs   │
│  - Security     │  - Drift    │  - PTY     │           │
├─────────────────────────────────────────────────────────┤
│              Docker Client (fsouza/go-dockerclient)     │
├─────────────────────────────────────────────────────────┤
│         SQLite WAL Mode (persistent state)              │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                 Docker Containers                        │
│  com.openclaw.manager=true + com.openclaw.agent.id     │
└─────────────────────────────────────────────────────────┘
```

## License

MIT
