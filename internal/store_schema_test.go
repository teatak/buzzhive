package buzzhive

import (
	"database/sql"
	"testing"
)

func TestEnsureSchemaCreatesCoreTables(t *testing.T) {
	store := openTestStore(t)

	for _, table := range []string{
		"users",
		"sessions",
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
		{"sessions", "expires_at", false, false, "timestamp with time zone", ""},
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
