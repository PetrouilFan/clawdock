package providers

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"clawdock/internal/database"
)

func TestDiscovery(t *testing.T) {
	if os.Getenv("RUN_NETWORK_TESTS") != "1" {
		t.Skip("set RUN_NETWORK_TESTS=1")
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	secret := []byte("testtesttesttesttesttesttesttest")
	reg := NewRegistry(db, secret)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO providers (id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 1, 0, ?, ?)`,
		"test-ollama", "Test Ollama", "http://localhost:11434", "none", now, now)
	if err != nil {
		t.Fatalf("insert provider: %v", err)
	}

	result, err := reg.DiscoverAndUpsertModels("test-ollama")
	if err != nil {
		t.Logf("Discovery failed (no Ollama?): %v", err)
		return
	}

	t.Logf("Discovery: %+v", result)
}
