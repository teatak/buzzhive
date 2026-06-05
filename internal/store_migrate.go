package buzzhive

import "fmt"

func (s *Store) Migrate() error {
	stmts := s.migrationStatements()
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.dropUnusedTables(); err != nil {
		return err
	}
	if err := s.migrateUsageLogColumns(); err != nil {
		return err
	}
	if err := s.createUsageLogIndexes(); err != nil {
		return err
	}
	if err := s.backfillUsageStatsIfEmpty(); err != nil {
		return err
	}
	if err := s.backfillGeminiModelContextWindow(); err != nil {
		return err
	}
	return nil
}

func (s *Store) migrationStatements() []string {
	idType := "INTEGER PRIMARY KEY AUTOINCREMENT"
	intType := "INTEGER"
	if s.dialect == "postgres" {
		idType = "BIGSERIAL PRIMARY KEY"
		intType = "BIGINT"
	}
	return []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS users (
			id %s,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`, idType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS user_api_keys (
			id %s,
			user_id %s NOT NULL,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`, idType, intType),
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id ` + intType + ` NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS providers (
			id %s,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			preset_id TEXT NOT NULL DEFAULT '',
			base_url TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`, idType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS provider_keys (
			id %s,
			provider_id %s NOT NULL,
			name TEXT NOT NULL,
			secret TEXT NOT NULL,
			secret_hint TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 1,
			labels TEXT NOT NULL DEFAULT '',
			disabled_status INTEGER NOT NULL DEFAULT 0,
			disabled_error_code TEXT NOT NULL DEFAULT '',
			disabled_error_message TEXT NOT NULL DEFAULT '',
			disabled_error_body TEXT NOT NULL DEFAULT '',
			disabled_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(provider_id, name),
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		)`, idType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS models (
			id %s,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			context_window %s NOT NULL DEFAULT 0,
			max_input_tokens %s NOT NULL DEFAULT 0,
			max_output_tokens %s NOT NULL DEFAULT 0,
			capabilities TEXT NOT NULL DEFAULT '{}',
			selection_policy TEXT NOT NULL DEFAULT 'round_robin',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`, idType, intType, intType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS model_routes (
			id %s,
			model_id %s NOT NULL,
			provider_id %s NOT NULL,
			upstream_model TEXT NOT NULL,
			quota_family TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (model_id) REFERENCES models(id),
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		)`, idType, intType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_logs (
			id %s,
			user_id %s NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id %s NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			provider_id %s NOT NULL DEFAULT 0,
			provider_name TEXT NOT NULL DEFAULT '',
			provider_key_id %s NOT NULL DEFAULT 0,
			provider_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			upstream_model TEXT NOT NULL DEFAULT '',
			status INTEGER NOT NULL DEFAULT 0,
			latency_ms %s NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`, idType, intType, intType, intType, intType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_stats_hourly (
			bucket_start TEXT NOT NULL,
			user_id %s NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id %s NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			requests %s NOT NULL DEFAULT 0,
			errors %s NOT NULL DEFAULT 0,
			latency_ms_sum %s NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, user_id, user_api_key_id, model)
		)`, intType, intType, intType, intType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_stats_daily (
			bucket_start TEXT NOT NULL,
			user_id %s NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id %s NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			requests %s NOT NULL DEFAULT 0,
			errors %s NOT NULL DEFAULT 0,
			latency_ms_sum %s NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, user_id, user_api_key_id, model)
		)`, intType, intType, intType, intType, intType),
		`CREATE INDEX IF NOT EXISTS idx_provider_keys_provider_id ON provider_keys(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_routes_model_id ON model_routes(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_routes_provider_id ON model_routes(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_hourly_user_bucket ON usage_stats_hourly(user_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_hourly_key_bucket ON usage_stats_hourly(user_api_key_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_daily_user_bucket ON usage_stats_daily(user_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_daily_key_bucket ON usage_stats_daily(user_api_key_id, bucket_start)`,
	}
}

func (s *Store) dropUnusedTables() error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS model_usage_logs`,
		`DROP TABLE IF EXISTS provider_accounts`,
		`DROP TABLE IF EXISTS api_keys`,
		`DROP TABLE IF EXISTS google_accounts`,
		`DROP TABLE IF EXISTS app_sessions`,
		`DROP TABLE IF EXISTS app_users`,
		`DROP TABLE IF EXISTS legacy_users`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateUsageLogColumns() error {
	intType := "INTEGER NOT NULL DEFAULT 0"
	if s.dialect == "postgres" {
		intType = "BIGINT NOT NULL DEFAULT 0"
	}
	columns := []struct {
		name string
		def  string
	}{
		{"user_id", intType},
		{"user_name", "TEXT NOT NULL DEFAULT ''"},
		{"user_api_key_id", intType},
		{"user_api_key_name", "TEXT NOT NULL DEFAULT ''"},
		{"provider_id", intType},
		{"provider_name", "TEXT NOT NULL DEFAULT ''"},
		{"provider_key_id", intType},
		{"provider_key_name", "TEXT NOT NULL DEFAULT ''"},
		{"model", "TEXT NOT NULL DEFAULT ''"},
		{"upstream_model", "TEXT NOT NULL DEFAULT ''"},
		{"status", "INTEGER NOT NULL DEFAULT 0"},
		{"latency_ms", intType},
		{"created_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		if err := s.addColumnIfMissing("usage_logs", column.name, column.def); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) createUsageLogIndexes() error {
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_user_created ON usage_logs(user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_key_created ON usage_logs(user_api_key_id, created_at)`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillGeminiModelContextWindow() error {
	_, err := s.exec(`
		UPDATE models
		SET
			context_window = CASE WHEN context_window = 1000000 THEN 1050000 ELSE context_window END,
			max_input_tokens = CASE WHEN max_input_tokens = 1000000 THEN 1050000 ELSE max_input_tokens END
		WHERE name LIKE 'gemini-%'
			AND (context_window = 1000000 OR max_input_tokens = 1000000)`)
	return err
}

func (s *Store) addColumnIfMissing(table, name, def string) error {
	if s.dialect == "postgres" {
		_, err := s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN IF NOT EXISTS ` + name + ` ` + def)
		return err
	}
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + def)
	return err
}
