package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func Init(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	if err := seedData(db); err != nil {
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		slug TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL DEFAULT 1,
		image_repo TEXT NOT NULL DEFAULT 'ghcr.io/openclaw/openclaw',
		image_tag TEXT NOT NULL DEFAULT 'latest',
		provider_id TEXT NOT NULL,
		model_id TEXT NOT NULL,
		telegram_api_key_encrypted TEXT,
		workspace_host_path TEXT NOT NULL,
		workspace_container_path TEXT NOT NULL DEFAULT '/workspace',
		extra_env_json TEXT,
		command_override_json TEXT,
		restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
		status_desired TEXT NOT NULL DEFAULT 'stopped',
		status_actual TEXT NOT NULL DEFAULT 'unknown',
		drift_state TEXT NOT NULL DEFAULT 'unknown',
		last_error TEXT,
		last_reconciled_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		deleted_at TEXT,
		spec_version INTEGER NOT NULL DEFAULT 1,
		config_revision INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS providers (
		id TEXT PRIMARY KEY,
		display_name TEXT NOT NULL,
		base_url TEXT,
		auth_type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		supports_model_discovery INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS provider_models (
		id TEXT PRIMARY KEY,
		provider_id TEXT NOT NULL,
		model_key TEXT NOT NULL,
		display_name TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		sort_order INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT,
		FOREIGN KEY (provider_id) REFERENCES providers(id)
	);

	CREATE TABLE IF NOT EXISTS backups (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		backup_type TEXT NOT NULL,
		archive_path TEXT,
		sha256 TEXT,
		size_bytes INTEGER,
		includes_secrets INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		created_by TEXT,
		FOREIGN KEY (agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		actor TEXT NOT NULL,
		action TEXT NOT NULL,
		agent_id TEXT,
		summary TEXT NOT NULL,
		payload_json TEXT,
		result TEXT NOT NULL,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS reconciler_runs (
		id TEXT PRIMARY KEY,
		started_at TEXT NOT NULL,
		finished_at TEXT,
		summary TEXT,
		result TEXT NOT NULL,
		details_json TEXT
	);

	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}

	return nil
}

func seedData(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)

	providers := []struct {
		id        string
		name     string
		baseURL  string
		authType string
		supports bool
	}{
		{"openai", "OpenAI", "https://api.openai.com", "api_key", false},
		{"anthropic", "Anthropic", "https://api.anthropic.com", "api_key", false},
		{"google", "Google AI", "https://generativelanguage.googleapis.com", "api_key", false},
		{"ollama", "Ollama", "http://localhost:11434", "none", true},
		{"openrouter", "OpenRouter", "https://openrouter.ai", "api_key", false},
		{"custom", "Custom Provider", "", "api_key", false},
	}

	for _, p := range providers {
		_, err := db.Exec(`INSERT OR IGNORE INTO providers (id, display_name, base_url, auth_type, enabled, supports_model_discovery, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?, ?)`,
			p.id, p.name, p.baseURL, p.authType, p.supports, now, now)
		if err != nil {
			return fmt.Errorf("seeding provider %s: %w", p.id, err)
		}
	}

	models := []struct {
		id          string
		providerID  string
		modelKey    string
		displayName string
		sortOrder   int
	}{
		// OpenAI models
		{"openai-gpt-4o", "openai", "gpt-4o", "GPT-4o", 1},
		{"openai-gpt-4o-mini", "openai", "gpt-4o-mini", "GPT-4o Mini", 2},
		{"openai-gpt-4-turbo", "openai", "gpt-4-turbo", "GPT-4 Turbo", 3},
		{"openai-gpt-3.5-turbo", "openai", "gpt-3.5-turbo", "GPT-3.5 Turbo", 4},
		// Anthropic models
		{"anthropic-claude-opus-4", "anthropic", "claude-opus-4", "Claude Opus 4", 1},
		{"anthropic-claude-sonnet-4", "anthropic", "claude-sonnet-4-20250514", "Claude Sonnet 4", 2},
		{"anthropic-claude-3-5-sonnet", "anthropic", "claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet", 3},
		{"anthropic-claude-3-5-haiku", "anthropic", "claude-3-5-haiku-20241022", "Claude 3.5 Haiku", 4},
		// Google models
		{"google-gemini-2-pro", "google", "gemini-2.0-pro-exp", "Gemini 2.0 Pro", 1},
		{"google-gemini-15-pro", "google", "gemini-1.5-pro", "Gemini 1.5 Pro", 2},
		{"google-gemini-15-flash", "google", "gemini-1.5-flash", "Gemini 1.5 Flash", 3},
		// Ollama models (defaults - user can discover more)
		{"ollama-llama3", "ollama", "llama3", "Llama 3", 1},
		{"ollama-llama3.1", "ollama", "llama3.1", "Llama 3.1", 2},
		{"ollama-mistral", "ollama", "mistral", "Mistral", 3},
		{"ollama-codellama", "ollama", "codellama", "Code Llama", 4},
		// OpenRouter models
		{"openrouter-anthropic-claude", "openrouter", "anthropic/claude-3.5-sonnet", "Claude 3.5 Sonnet (OpenRouter)", 1},
		{"openrouter-openai-gpt", "openrouter", "openai/gpt-4o", "GPT-4o (OpenRouter)", 2},
	}

	for _, m := range models {
		_, err := db.Exec(`INSERT OR IGNORE INTO provider_models (id, provider_id, model_key, display_name, enabled, sort_order)
			VALUES (?, ?, ?, ?, 1, ?)`,
			m.id, m.providerID, m.modelKey, m.displayName, m.sortOrder)
		if err != nil {
			return fmt.Errorf("seeding model %s: %w", m.id, err)
		}
	}

	return nil
}
