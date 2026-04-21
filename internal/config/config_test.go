package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Create a minimal config
	content := `
server:
  port: "11436"
database:
  path: "/tmp/test.db"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults applied
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != "11436" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "11436")
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/tmp/test.db")
	}
	if cfg.Agents.DefaultImageRepo != "ghcr.io/openclaw/openclaw" {
		t.Errorf("DefaultImageRepo = %q, want %q", cfg.Agents.DefaultImageRepo, "ghcr.io/openclaw/openclaw")
	}
	if cfg.Agents.DefaultRestartPolicy != "unless-stopped" {
		t.Errorf("DefaultRestartPolicy = %q, want %q", cfg.Agents.DefaultRestartPolicy, "unless-stopped")
	}
}

func TestLoadSecretKey(t *testing.T) {
	content := `
server:
  port: "11436"
database:
  path: "/tmp/test.db"
security:
  secret_key_file: "/tmp/secret.key"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	secretKey := "test-secret-key-12345"
	if err := os.WriteFile("/tmp/secret.key", []byte(secretKey), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("/tmp/secret.key")

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Security.SecretKey != secretKey {
		t.Errorf("SecretKey = %q, want %q", cfg.Security.SecretKey, secretKey)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte("")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should get defaults even with empty file
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default", cfg.Server.Host)
	}
}
