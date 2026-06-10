package buzzhive

func (s *Store) EnsureSchema() error {
	stmts := s.schemaStatements()
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) schemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS user_api_keys (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			valid INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			preset_id TEXT NOT NULL DEFAULT '',
			base_url TEXT NOT NULL,
			supports_responses INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`ALTER TABLE providers ADD COLUMN IF NOT EXISTS supports_responses INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS provider_keys (
			id BIGSERIAL PRIMARY KEY,
			provider_id BIGINT NOT NULL,
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
			disabled_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(provider_id, name),
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS models (
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
		)`,
		`CREATE TABLE IF NOT EXISTS model_routes (
			id BIGSERIAL PRIMARY KEY,
			model_id BIGINT NOT NULL,
			provider_id BIGINT NOT NULL,
			upstream_model TEXT NOT NULL,
			quota_family TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			FOREIGN KEY (model_id) REFERENCES models(id),
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS usage_logs (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id BIGINT NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			provider_id BIGINT NOT NULL DEFAULT 0,
			provider_name TEXT NOT NULL DEFAULT '',
			provider_key_id BIGINT NOT NULL DEFAULT 0,
			provider_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			upstream_model TEXT NOT NULL DEFAULT '',
			status INTEGER NOT NULL DEFAULT 0,
			latency_ms BIGINT NOT NULL DEFAULT 0,
			prompt_tokens BIGINT NOT NULL DEFAULT 0,
			completion_tokens BIGINT NOT NULL DEFAULT 0,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			cached_tokens BIGINT NOT NULL DEFAULT 0,
			reasoning_tokens BIGINT NOT NULL DEFAULT 0,
			raw_usage TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS usage_stats_hourly (
			bucket_start TIMESTAMPTZ NOT NULL,
			user_id BIGINT NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id BIGINT NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			requests BIGINT NOT NULL DEFAULT 0,
			errors BIGINT NOT NULL DEFAULT 0,
			latency_ms_sum BIGINT NOT NULL DEFAULT 0,
			prompt_tokens_sum BIGINT NOT NULL DEFAULT 0,
			completion_tokens_sum BIGINT NOT NULL DEFAULT 0,
			total_tokens_sum BIGINT NOT NULL DEFAULT 0,
			cached_tokens_sum BIGINT NOT NULL DEFAULT 0,
			reasoning_tokens_sum BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, user_id, user_api_key_id, model)
		)`,
		`CREATE TABLE IF NOT EXISTS usage_stats_daily (
			bucket_start TIMESTAMPTZ NOT NULL,
			user_id BIGINT NOT NULL DEFAULT 0,
			user_name TEXT NOT NULL DEFAULT '',
			user_api_key_id BIGINT NOT NULL DEFAULT 0,
			user_api_key_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			requests BIGINT NOT NULL DEFAULT 0,
			errors BIGINT NOT NULL DEFAULT 0,
			latency_ms_sum BIGINT NOT NULL DEFAULT 0,
			prompt_tokens_sum BIGINT NOT NULL DEFAULT 0,
			completion_tokens_sum BIGINT NOT NULL DEFAULT 0,
			total_tokens_sum BIGINT NOT NULL DEFAULT 0,
			cached_tokens_sum BIGINT NOT NULL DEFAULT 0,
			reasoning_tokens_sum BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, user_id, user_api_key_id, model)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_provider_keys_provider_id ON provider_keys(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_routes_model_id ON model_routes(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_model_routes_provider_id ON model_routes(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_user_created ON usage_logs(user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_key_created ON usage_logs(user_api_key_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_hourly_user_bucket ON usage_stats_hourly(user_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_hourly_key_bucket ON usage_stats_hourly(user_api_key_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_daily_user_bucket ON usage_stats_daily(user_id, bucket_start)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_stats_daily_key_bucket ON usage_stats_daily(user_api_key_id, bucket_start)`,
		`UPDATE providers SET type = 'openai' WHERE type = 'openai-compatible'`,
	}
}
