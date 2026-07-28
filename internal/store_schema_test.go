package buzzhive

import (
	"database/sql"
	"strings"
	"testing"
)

func TestEnsureSchemaCreatesCoreTables(t *testing.T) {
	store := openTestStore(t)

	for _, table := range []string{
		"users",
		"user_api_keys",
		"providers",
		"provider_keys",
		"models",
		"model_routes",
		"usage_logs",
		"usage_stats_hourly",
		"usage_stats_daily",
	} {
		if !postgresTableExists(t, store.db, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}

func TestEnsureSchemaTimestampColumns(t *testing.T) {
	store := openTestStore(t)

	for _, column := range []struct {
		table       string
		name        string
		hasDefault  bool
		isNullable  bool
		dataType    string
		defaultExpr string
	}{
		{"users", "created_at", true, false, "timestamp with time zone", "now()"},
		{"users", "updated_at", true, false, "timestamp with time zone", "now()"},
		{"provider_keys", "disabled_at", false, true, "timestamp with time zone", ""},
		{"usage_logs", "created_at", true, false, "timestamp with time zone", "now()"},
		{"usage_stats_hourly", "bucket_start", false, false, "timestamp with time zone", ""},
		{"usage_stats_daily", "bucket_start", false, false, "timestamp with time zone", ""},
	} {
		dataType, nullable, defaultExpr := postgresColumnInfo(t, store.db, column.table, column.name)
		if dataType != column.dataType {
			t.Fatalf("%s.%s data_type = %q, want %q", column.table, column.name, dataType, column.dataType)
		}
		if (nullable == "YES") != column.isNullable {
			t.Fatalf("%s.%s nullable = %q", column.table, column.name, nullable)
		}
		if column.hasDefault && defaultExpr != column.defaultExpr {
			t.Fatalf("%s.%s default = %q, want %q", column.table, column.name, defaultExpr, column.defaultExpr)
		}
		if !column.hasDefault && defaultExpr != "" {
			t.Fatalf("%s.%s default = %q, want empty", column.table, column.name, defaultExpr)
		}
	}
}

func TestEnsureSchemaDropsUnusedProviderColumns(t *testing.T) {
	store := openTestStore(t)

	for _, column := range []string{"supports_responses", "base_url", "protocols"} {
		if postgresColumnExists(t, store.db, "providers", column) {
			t.Fatalf("providers.%s should not exist", column)
		}
	}
}

func TestModelRoutesStoreProviderProtocolPolicy(t *testing.T) {
	store := openTestStore(t)

	if !postgresColumnExists(t, store.db, "model_routes", "provider_id") {
		t.Fatal("model_routes.provider_id should exist")
	}
	if !postgresColumnExists(t, store.db, "model_routes", "upstream_protocol") {
		t.Fatal("model_routes.upstream_protocol should exist")
	}
	if postgresColumnExists(t, store.db, "model_routes", "provider_endpoint_id") {
		t.Fatal("model_routes.provider_endpoint_id should not exist")
	}
	dataType, nullable, defaultExpr := postgresColumnInfo(t, store.db, "model_routes", "upstream_protocol")
	if dataType != "text" || nullable != "NO" || !strings.Contains(defaultExpr, "auto") {
		t.Fatalf("model_routes.upstream_protocol = type %q nullable %q default %q", dataType, nullable, defaultExpr)
	}
}

func TestEnsureSchemaUpgradesPublishedRoutesWithoutLosingAccounts(t *testing.T) {
	store := openTestStoreWithSetup(t, func(db *sql.DB) {
		_, err := db.Exec(`
			CREATE TABLE users (
				id BIGSERIAL PRIMARY KEY,
				username TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				role TEXT NOT NULL DEFAULT 'user',
				valid INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE TABLE providers (
				id BIGSERIAL PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				preset_id TEXT NOT NULL DEFAULT '',
				enabled INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE TABLE models (
				id BIGSERIAL PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				display_name TEXT NOT NULL DEFAULT '',
				description TEXT NOT NULL DEFAULT '',
				context_window BIGINT NOT NULL DEFAULT 0,
				max_input_tokens BIGINT NOT NULL DEFAULT 0,
				max_output_tokens BIGINT NOT NULL DEFAULT 0,
				capabilities TEXT NOT NULL DEFAULT '{}',
				selection_policy TEXT NOT NULL DEFAULT 'round_robin',
				enabled INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE TABLE model_routes (
				id BIGSERIAL PRIMARY KEY,
				model_id BIGINT NOT NULL REFERENCES models(id),
				provider_id BIGINT NOT NULL REFERENCES providers(id),
				upstream_model TEXT NOT NULL,
				enabled INTEGER NOT NULL DEFAULT 1,
				priority INTEGER NOT NULL DEFAULT 0,
				weight INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			INSERT INTO users (username, password_hash, role) VALUES ('admin', 'hash-kept', 'admin');
			INSERT INTO providers (name) VALUES ('published-provider');
			INSERT INTO models (name) VALUES ('published-model');
			INSERT INTO model_routes (model_id, provider_id, upstream_model)
			SELECT m.id, p.id, 'published-upstream'
			FROM models m, providers p
			WHERE m.name = 'published-model' AND p.name = 'published-provider';
		`)
		if err != nil {
			t.Fatal(err)
		}
	})

	var passwordHash string
	if err := store.db.QueryRow(`SELECT password_hash FROM users WHERE username = 'admin'`).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if passwordHash != "hash-kept" {
		t.Fatalf("password hash = %q", passwordHash)
	}
	var protocol string
	if err := store.db.QueryRow(`SELECT upstream_protocol FROM model_routes WHERE upstream_model = 'published-upstream'`).Scan(&protocol); err != nil {
		t.Fatal(err)
	}
	if protocol != providerAuto {
		t.Fatalf("upstream protocol = %q, want %q", protocol, providerAuto)
	}
	if err := store.EnsureSchema(); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
}

func postgresTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func postgresColumnInfo(t *testing.T, db *sql.DB, table, column string) (string, string, string) {
	t.Helper()
	var dataType, nullable string
	var defaultExpr sql.NullString
	err := db.QueryRow(`
		SELECT data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema()
			AND table_name = $1
			AND column_name = $2`,
		table, column,
	).Scan(&dataType, &nullable, &defaultExpr)
	if err != nil {
		t.Fatal(err)
	}
	return dataType, nullable, defaultExpr.String
}

func postgresColumnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = $1
				AND column_name = $2
		)`,
		table, column,
	).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}
