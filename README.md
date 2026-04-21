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
curl -sSL https://raw.githubusercontent.com/PetrouilFan/clawdock/refs/heads/main/scripts/install.sh | bash
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
git clone https://github.com/PetrouilFan/clawdock.git
cd clawdock
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
  host: 0.0.0.0
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
| `GET /api/providers` | List all providers (built-in and custom) |
| `POST /api/providers` | Create custom provider |
| `GET /api/providers/:id` | Get provider details |
| `PATCH /api/providers/:id` | Update provider (name, URL, auth, discovery flag, enabled) |
| `DELETE /api/providers/:id` | Delete custom provider (built-in cannot be deleted) |
| `POST /api/providers/:id/refresh-models` | Discover and sync models from provider API |
| `GET /api/providers/:id/models` | List models for a provider |
| `PATCH /api/provider-models/:id` | Enable/disable a specific model |
| `GET /api/models/status` | Get health status for all models |

### Custom Model Aliases

Create friendly names that map to provider models.

| Endpoint | Description |
|----------|-------------|
| `GET /api/custom-models` | List all custom model aliases |
| `POST /api/custom-models/:alias` | Create alias (e.g., `my-gpt-4` → `openai/gpt-4o`) |
| `PATCH /api/custom-models/:alias` | Update alias target or enabled state |
| `DELETE /api/custom-models/:alias` | Delete alias |

### Settings

| Endpoint | Description |
|----------|-------------|
| `GET /api/settings/default_model` | Get global default model key |
| `PUT /api/settings/default_model` | Set default model (must be valid and enabled) |
| `GET /api/settings/chat_proxy_enabled` | Check if chat proxy is enabled |
| `PUT /api/settings/chat_proxy_enabled` | Enable/disable chat proxy |

### Chat Proxy (OpenAI-Compatible)

Clawdock can act as an OpenAI-compatible gateway, routing requests to configured providers.

| Endpoint | Description |
|----------|-------------|
| `GET /v1/models` | List available models (provider models + custom aliases) in OpenAI format |
| `POST /v1/chat/completions` | Create chat completion; routed to appropriate provider |

Supported providers: OpenAI, Anthropic, Google, Ollama, OpenRouter, and any OpenAI-compatible endpoint.

Request format matches OpenAI's API. The response is translated from the provider's native format to OpenAI's format when necessary.

**Example:**

```bash
curl -X POST http://localhost:11436/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": false
  }'
```

### Agent Validation

When creating or updating an agent, the system validates:
- Provider exists and is enabled
- Model exists and is enabled for that provider
- If a custom model alias is used, it resolves to an existing enabled model

If the provider requires authentication (API key or bearer token), you may optionally provide a per-agent API key override. If omitted, the provider's stored key is used.

## Model Discovery

For providers with `supports_model_discovery` enabled, you can fetch the list of available models:

```bash
curl -X POST http://localhost:11436/api/providers/{provider-id}/refresh-models
```

Models are **added** and **stale models are disabled** (sync exactly). This keeps your model list up-to-date with the remote provider.

Some providers (like Ollama) may require no authentication; others (OpenAI, Anthropic, OpenRouter) require an API key stored in the provider configuration.

## Provider Management

### Built-in Providers

Seeded providers (OpenAI, Anthropic, Google, Ollama, OpenRouter, Custom) are marked as `is_builtin`. They can be edited (e.g., change base URL or API key) but cannot be deleted.

### Custom Providers

Create a provider pointing to any endpoint:

```json
{
  "display_name": "My Ollama",
  "base_url": "https://mainframeollama.petrouil.com",
  "auth_type": "none",
  "supports_model_discovery": true,
  "enabled": true
}
```

Authentication types:
- `none` - No authentication header (Ollama, local proxies)
- `api_key` - Adds `Authorization: Bearer <key>` header
- `bearer` - Same as `api_key` (explicit)

## Model Proxy Architecture

The chat proxy resolves the requested model in this order:
1. Look up custom model alias (`custom_models` table). If found and enabled, use its target.
2. Look up provider model by `model_key`. Must be enabled.
3. If model not found or disabled → error response.

The proxy then:
- Retrieves the provider's configuration (base URL, auth type, encrypted API key)
- Decrypts the API key in-memory
- Translates the request body to the provider's expected format if needed
- Forwards the request, streaming or buffered
- Translates the response back to OpenAI format for clients

Translation adapters:
- Anthropic: converts `/v1/messages` response to OpenAI's chat completion format
- Ollama: converts NDJSON streaming to OpenAI SSE format

## Demo: Connect to Remote Ollama

Use the included demo script to quickly configure a remote Ollama instance:

```bash
./scripts/demo-ollama.sh https://mainframeollama.petrouil.com
```

This will:
1. Create a provider for the given URL
2. Refresh models (discovering `llama3`, `mistral`, etc.)
3. Set `llama3` as the default model

You can then test the chat proxy:

```bash
curl -X POST http://localhost:11436/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3","messages":[{"role":"user","content":"Hello"}]}'
```

## Security Considerations

- Provider API keys are encrypted at rest using AES-256-GCM with the server's `secret_key`.
- Keys are never exposed via APIs (except masked hints in UI).
- All management actions are audit-logged.
- Rate limiting is applied to all endpoints (default 100 req/min per IP).
- Inputs are sanitized to prevent XSS and command injection.

## Development

### Running Locally

```bash
go run ./cmd/server --config=config.local.yaml
```

The server listens on `0.0.0.0:11436` by default.

### Building

```bash
make build
```

### Testing

```bash
make test
```

## License

MIT

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

- Bind address `0.0.0.0` for network access (Tailscale, etc.)
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
