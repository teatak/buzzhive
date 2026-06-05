package buzzhive

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateDropsUnusedTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "buzzhive.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE google_accounts (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE api_keys (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE app_users (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE app_sessions (token_hash TEXT PRIMARY KEY)`,
		`CREATE TABLE legacy_users (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE usage_logs (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE model_usage_logs (id INTEGER PRIMARY KEY)`,
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

	for _, table := range []string{"users", "sessions", "providers", "provider_keys", "models", "model_routes", "usage_logs", "usage_stats_hourly", "usage_stats_daily"} {
		exists := sqliteTableExists(t, store.db, table)
		if !exists {
			t.Fatalf("expected table %s to exist", table)
		}
	}
	for _, table := range []string{"google_accounts", "api_keys", "app_users", "app_sessions", "legacy_users", "model_usage_logs"} {
		exists := sqliteTableExists(t, store.db, table)
		if exists {
			t.Fatalf("expected table %s to be dropped", table)
		}
	}
	for _, column := range []string{"user_api_key_id", "provider_id", "provider_key_id", "model", "upstream_model", "status", "latency_ms", "created_at"} {
		exists := sqliteColumnExists(t, store.db, "usage_logs", column)
		if !exists {
			t.Fatalf("expected usage_logs column %s to exist", column)
		}
	}
}

func sqliteTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func sqliteColumnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}
