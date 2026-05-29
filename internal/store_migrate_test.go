package buzzhive

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateAppUserTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "buzzhive.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO users (name, token, valid, created_at, updated_at) VALUES ('legacy', 'old-token', 1, 'now', 'now')`,
		`CREATE TABLE app_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO app_users (username, password_hash, role, valid, created_at, updated_at) VALUES ('admin', 'hash', 'admin', 1, 'now', 'now')`,
		`CREATE TABLE app_sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`INSERT INTO app_sessions (token_hash, user_id, expires_at, created_at) VALUES ('session-hash', 1, '2099-01-01T00:00:00Z', 'now')`,
		`CREATE TABLE user_api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO user_api_keys (user_id, name, token, valid, created_at, updated_at) VALUES (1, 'default', 'bh_test', 1, 'now', 'now')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(DatabaseConfig{Path: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, table := range []string{"users", "sessions", "legacy_users"} {
		exists, err := store.tableExists(table)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", table)
		}
	}
	for _, table := range []string{"app_users", "app_sessions"} {
		exists, err := store.tableExists(table)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("expected table %s to be migrated away", table)
		}
	}

	var username string
	if err := store.queryRow(`SELECT username FROM users WHERE id = 1`).Scan(&username); err != nil {
		t.Fatal(err)
	}
	if username != "admin" {
		t.Fatalf("username = %q, want admin", username)
	}

	var sessionHash string
	if err := store.queryRow(`SELECT token_hash FROM sessions WHERE user_id = 1`).Scan(&sessionHash); err != nil {
		t.Fatal(err)
	}
	if sessionHash != "session-hash" {
		t.Fatalf("session hash = %q, want session-hash", sessionHash)
	}
}
