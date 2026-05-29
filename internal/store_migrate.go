package buzzhive

import "fmt"

func (s *Store) Migrate() error {
	stmts := s.migrationStatements()
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	idRefType := "INTEGER"
	if s.dialect == "postgres" {
		idRefType = "BIGINT"
	}
	for _, column := range []struct {
		name string
		def  string
	}{
		{"user_api_key_id", idRefType},
		{"user_api_key_name", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.addColumnIfMissing("usage_logs", column.name, column.def); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_usage_logs_user_api_key_id ON usage_logs(user_api_key_id)`); err != nil {
		return err
	}
	for _, column := range []struct {
		name string
		def  string
	}{
		{"disabled_status", "INTEGER NOT NULL DEFAULT 0"},
		{"disabled_error_code", "TEXT NOT NULL DEFAULT ''"},
		{"disabled_error_message", "TEXT NOT NULL DEFAULT ''"},
		{"disabled_error_body", "TEXT NOT NULL DEFAULT ''"},
		{"disabled_at", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.addColumnIfMissing("api_keys", column.name, column.def); err != nil {
			return err
		}
	}
	for _, column := range []struct {
		name string
		def  string
	}{
		{"request_id", "TEXT NOT NULL DEFAULT ''"},
		{"attempt", "INTEGER NOT NULL DEFAULT 0"},
		{"error_code", "TEXT NOT NULL DEFAULT ''"},
		{"error_message", "TEXT NOT NULL DEFAULT ''"},
		{"error_body", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.addColumnIfMissing("model_usage_logs", column.name, column.def); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_model_usage_logs_request_id ON model_usage_logs(request_id)`); err != nil {
		return err
	}
	if err := s.backfillDisabledAPIKeyReasons(); err != nil {
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
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS app_users (
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
			FOREIGN KEY (user_id) REFERENCES app_users(id)
		)`, idType, intType),
		`CREATE TABLE IF NOT EXISTS app_sessions (
			token_hash TEXT PRIMARY KEY,
			user_id ` + intType + ` NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES app_users(id)
		)`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS users (
			id %s,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`, idType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS google_accounts (
			id %s,
			email TEXT NOT NULL UNIQUE,
			prefix TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`, idType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS api_keys (
			id %s,
			google_account_id %s NOT NULL,
			name TEXT NOT NULL UNIQUE,
			key TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (google_account_id) REFERENCES google_accounts(id)
		)`, idType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_logs (
			id %s,
			user_id %s,
			user_name TEXT NOT NULL,
			api_key_id %s,
			api_key_name TEXT NOT NULL,
			google_account_id %s,
			google_account_email TEXT NOT NULL,
			model TEXT NOT NULL,
			status INTEGER NOT NULL,
			latency_ms INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`, idType, intType, intType, intType),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS model_usage_logs (
			id %s,
			user_id %s,
			user_name TEXT NOT NULL,
			user_api_key_id %s,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			api_key_id %s,
			api_key_name TEXT NOT NULL,
			google_account_id %s,
			google_account_email TEXT NOT NULL,
			request_id TEXT NOT NULL DEFAULT '',
			attempt INTEGER NOT NULL DEFAULT 0,
			model TEXT NOT NULL,
			status INTEGER NOT NULL,
			latency_ms INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			error_code TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			error_body TEXT NOT NULL DEFAULT ''
		)`, idType, intType, intType, intType, intType),
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_api_key_id ON usage_logs(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_usage_logs_created_at ON model_usage_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_model_usage_logs_api_key_id ON model_usage_logs(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_usage_logs_model ON model_usage_logs(model)`,
		`CREATE INDEX IF NOT EXISTS idx_app_sessions_user_id ON app_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_app_sessions_expires_at ON app_sessions(expires_at)`,
	}
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
