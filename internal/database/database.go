package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	if err := Migrate(db); err != nil {
		return nil, err
	}

	if err := seedData(db); err != nil {
		return nil, err
	}

	return db, nil
}

// Migrate applies all schema changes and migrations to the database.
// It creates tables if missing and adds new columns for upgrades.
func Migrate(db *sql.DB) error {
	// Create tables if they don't exist (fresh install)
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
		api_key_encrypted TEXT,
		is_builtin BOOLEAN DEFAULT 0 NOT NULL,
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
		last_health_check TEXT,
		health_status TEXT DEFAULT 'unknown',
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

	CREATE TABLE IF NOT EXISTS custom_models (
		id TEXT PRIMARY KEY,
		target_provider_id TEXT NOT NULL,
		target_model_key TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY (target_provider_id) REFERENCES providers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}

	// --- Upgrades for existing installations ---
	// Add missing columns to providers
	if _, err := db.Exec("ALTER TABLE providers ADD COLUMN base_url TEXT"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding base_url column: %w", err)
		}
	}
	if _, err := db.Exec("ALTER TABLE providers ADD COLUMN api_key_encrypted TEXT"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding api_key_encrypted column: %w", err)
		}
	}
	if _, err := db.Exec("ALTER TABLE providers ADD COLUMN is_builtin BOOLEAN DEFAULT 0 NOT NULL"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding is_builtin column: %w", err)
		}
	}
	if _, err := db.Exec("ALTER TABLE providers ADD COLUMN supports_model_discovery INTEGER DEFAULT 0 NOT NULL"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding supports_model_discovery column: %w", err)
		}
	}

	// Add missing columns to provider_models
	if _, err := db.Exec("ALTER TABLE provider_models ADD COLUMN health_status TEXT DEFAULT 'unknown'"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding health_status column: %w", err)
		}
	}
	if _, err := db.Exec("ALTER TABLE provider_models ADD COLUMN last_health_check TEXT"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("adding last_health_check column: %w", err)
		}
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_providers_enabled ON providers(enabled)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_model_key ON provider_models(provider_id, model_key)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_model_target ON custom_models(target_provider_id, target_model_key)",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}

	return nil
}

func seedData(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)

	providers := []struct {
		id          string
		displayName string
		baseURL     string
		authType    string
		supports    bool
	}{
		{"openai", "OpenAI", "https://api.openai.com", "api_key", false},
		{"anthropic", "Anthropic", "https://api.anthropic.com", "api_key", false},
		{"google", "Google AI", "https://generativelanguage.googleapis.com", "api_key", false},
		{"ollama", "Ollama", "http://localhost:11434", "none", true},
		{"openrouter", "OpenRouter", "https://openrouter.ai", "bearer", false},
		{"custom", "Custom Provider", "", "api_key", false},
	}
	for _, p := range providers {
		_, err := db.Exec(`INSERT INTO providers (id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, 1, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				display_name = excluded.display_name,
				base_url = excluded.base_url,
				auth_type = excluded.auth_type,
				supports_model_discovery = excluded.supports_model_discovery,
				is_builtin = 1,
				updated_at = excluded.updated_at`,
			p.id, p.displayName, p.baseURL, p.authType, p.supports, now, now)
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
		{"openai-gpt-4o", "openai", "gpt-4o", "GPT-4o", 1},
		{"openai-gpt-4o-mini", "openai", "gpt-4o-mini", "GPT-4o Mini", 2},
		{"openai-gpt-4-turbo", "openai", "gpt-4-turbo", "GPT-4 Turbo", 3},
		{"openai-gpt-3.5-turbo", "openai", "gpt-3.5-turbo", "GPT-3.5 Turbo", 4},
		{"anthropic-claude-opus-4", "anthropic", "claude-opus-4", "Claude Opus 4", 1},
		{"anthropic-claude-sonnet-4", "anthropic", "claude-sonnet-4-20250514", "Claude Sonnet 4", 2},
		{"anthropic-claude-3-5-sonnet", "anthropic", "claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet", 3},
		{"anthropic-claude-3-5-haiku", "anthropic", "claude-3-5-haiku-20241022", "Claude 3.5 Haiku", 4},
		{"google-gemini-2-pro", "google", "gemini-2.0-pro-exp", "Gemini 2.0 Pro", 1},
		{"google-gemini-15-pro", "google", "gemini-1.5-pro", "Gemini 1.5 Pro", 2},
		{"google-gemini-15-flash", "google", "gemini-1.5-flash", "Gemini 1.5 Flash", 3},
		{"ollama-llama3", "ollama", "llama3", "Llama 3", 1},
		{"ollama-llama3.1", "ollama", "llama3.1", "Llama 3.1", 2},
		{"ollama-mistral", "ollama", "mistral", "Mistral", 3},
		{"ollama-codellama", "ollama", "codellama", "Code Llama", 4},
		{"openrouter-anthropic-claude", "openrouter", "anthropic/claude-3.5-sonnet", "Claude 3.5 Sonnet (OpenRouter)", 1},
		{"openrouter-openai-gpt", "openrouter", "openai/gpt-4o", "GPT-4o (OpenRouter)", 2},
	}
	for _, m := range models {
		_, err := db.Exec(`INSERT OR IGNORE INTO provider_models (id, provider_id, model_key, display_name, enabled, sort_order, health_status)
			VALUES (?, ?, ?, ?, 1, ?, 'unknown')`,
			m.id, m.providerID, m.modelKey, m.displayName, m.sortOrder)
		if err != nil {
			return fmt.Errorf("seeding model %s: %w", m.id, err)
		}
	}

	settings := []struct {
		key   string
		value string
	}{
		{"default_model", ""},
		{"chat_proxy_enabled", "true"},
	}
	for _, s := range settings {
		_, err := db.Exec(`INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES (?, ?, ?)`,
			s.key, s.value, now)
		if err != nil {
			return fmt.Errorf("seeding setting %s: %w", s.key, err)
		}
	}

	return nil
}

// GetSetting fetches a setting value by key; returns empty string if not found.
func GetSetting(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

// SetSetting updates or inserts a setting.
func SetSetting(db *sql.DB, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, now)
	return err
}
