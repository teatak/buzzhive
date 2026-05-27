package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	db      *sql.DB
	dialect string
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
	URL    string `yaml:"url"`
}

type UsageRecord struct {
	UserID             int64
	UserName           string
	UserAPIKeyID       int64
	UserAPIKeyName     string
	APIKeyID           int64
	APIKeyName         string
	GoogleAccountID    int64
	GoogleAccountEmail string
	Model              string
	Status             int
	LatencyMS          int64
}

type UsageSummary struct {
	Requests     int64            `json:"requests"`
	Errors       int64            `json:"errors"`
	AvgLatencyMS float64          `json:"avg_latency_ms"`
	ByKey        map[string]int64 `json:"by_key"`
	Series       []UsagePoint     `json:"series"`
}

type UsagePoint struct {
	Date         string  `json:"date"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
}

type UsageQuery struct {
	UserID       int64
	UserAPIKeyID int64
	From         time.Time
	To           time.Time
}

type SessionUser struct {
	User      AppUser
	ExpiresAt time.Time
}

func OpenStore(cfg DatabaseConfig) (*Store, error) {
	driver := strings.ToLower(cfg.Driver)
	if driver == "" {
		driver = "sqlite"
	}
	var dsn string
	switch driver {
	case "sqlite", "sqlite3":
		driver = "sqlite"
		if cfg.Path == "" {
			cfg.Path = "data/buzzhive.db"
		}
		dsn = cfg.Path + "?_busy_timeout=5000&_journal_mode=WAL"
	case "postgres", "postgresql", "pg":
		driver = "postgres"
		dsn = cfg.URL
		if dsn == "" {
			return nil, errors.New("database.url is required for postgres")
		}
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}

	sqlDriver := driver
	if driver == "sqlite" {
		sqlDriver = "sqlite3"
	}
	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, dialect: driver}
	if err := store.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

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
	return nil
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.rebind(query), args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(s.rebind(query), args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(s.rebind(query), args...)
}

func (s *Store) prepareTx(tx *sql.Tx, query string) (*sql.Stmt, error) {
	return tx.Prepare(s.rebind(query))
}

func (s *Store) insertReturningID(query string, args ...any) (int64, error) {
	if s.dialect == "postgres" {
		var id int64
		err := s.queryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := s.exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) rebind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	arg := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
			arg++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
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
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_logs_api_key_id ON usage_logs(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_app_sessions_user_id ON app_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_app_sessions_expires_at ON app_sessions(expires_at)`,
	}
}

func (s *Store) Seed(cfg Config) error {
	now := time.Now().Format(time.RFC3339)
	accountIDs := make(map[string]int64)
	for email, prefix := range cfg.KeyAccounts {
		if email == "" || prefix == "" {
			continue
		}
		if _, err := s.exec(
			`INSERT INTO google_accounts (email, prefix, enabled, created_at, updated_at)
			 VALUES (?, ?, 1, ?, ?)
			 ON CONFLICT(email) DO UPDATE SET prefix = excluded.prefix, enabled = 1, updated_at = excluded.updated_at`,
			email, prefix, now, now,
		); err != nil {
			return err
		}
		id, err := s.accountIDByPrefix(prefix)
		if err != nil {
			return err
		}
		accountIDs[prefix] = id
	}

	for _, key := range cfg.GeminiAPIKeys {
		prefix := keyPrefix(key.Name)
		accountID := accountIDs[prefix]
		if accountID == 0 {
			continue
		}
		if _, err := s.exec(
			`INSERT INTO api_keys (google_account_id, name, key, enabled, created_at, updated_at)
			 VALUES (?, ?, ?, 1, ?, ?)
			 ON CONFLICT(name) DO UPDATE SET google_account_id = excluded.google_account_id, key = excluded.key, enabled = 1, updated_at = excluded.updated_at`,
			accountID, key.Name, key.Key, now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AuthTokens() (map[string]AuthToken, error) {
	rows, err := s.query(`
		SELECT k.id, k.user_id, u.username, k.name, k.token, k.valid
		FROM user_api_keys k
		JOIN app_users u ON u.id = k.user_id
		WHERE k.valid = 1 AND u.valid = 1
		ORDER BY k.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]AuthToken)
	for rows.Next() {
		var user AuthToken
		var valid int
		if err := rows.Scan(&user.ID, &user.UserID, &user.UserName, &user.Name, &user.Token, &valid); err != nil {
			return nil, err
		}
		user.Valid = valid != 0
		out[user.Token] = user
	}
	return out, rows.Err()
}

func (s *Store) Users() ([]AppUser, error) {
	rows, err := s.query(`SELECT id, username, role, valid FROM app_users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AppUser
	for rows.Next() {
		var user AppUser
		var valid int
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &valid); err != nil {
			return nil, err
		}
		user.Valid = valid != 0
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *Store) UserAPIKeys() ([]AuthToken, error) {
	rows, err := s.query(`SELECT id, user_id, name, token, valid FROM user_api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuthToken
	for rows.Next() {
		var key AuthToken
		var valid int
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.Token, &valid); err != nil {
			return nil, err
		}
		key.Valid = valid != 0
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) GoogleAccounts() ([]GoogleAccount, error) {
	rows, err := s.query(`SELECT id, email, prefix, enabled FROM google_accounts ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GoogleAccount
	for rows.Next() {
		var account GoogleAccount
		var enabled int
		if err := rows.Scan(&account.ID, &account.Email, &account.Prefix, &enabled); err != nil {
			return nil, err
		}
		account.Enabled = enabled != 0
		out = append(out, account)
	}
	return out, rows.Err()
}

func (s *Store) APIKeys() ([]APIKey, error) {
	rows, err := s.query(`
		SELECT k.id, k.google_account_id, k.name, k.key, k.enabled, a.email, a.prefix
		FROM api_keys k
		JOIN google_accounts a ON a.id = k.google_account_id
		WHERE k.enabled = 1 AND a.enabled = 1
		ORDER BY k.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAPIKeys(rows)
}

func (s *Store) AllAPIKeys() ([]APIKey, error) {
	rows, err := s.query(`
		SELECT k.id, k.google_account_id, k.name, k.key, k.enabled, a.email, a.prefix
		FROM api_keys k
		JOIN google_accounts a ON a.id = k.google_account_id
		ORDER BY k.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAPIKeys(rows)
}

func scanAPIKeys(rows *sql.Rows) ([]APIKey, error) {
	var out []APIKey
	for rows.Next() {
		var key APIKey
		var enabled int
		if err := rows.Scan(&key.ID, &key.AccountID, &key.Name, &key.Key, &enabled, &key.AccountEmail, &key.AccountPrefix); err != nil {
			return nil, err
		}
		key.Enabled = enabled != 0
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) CreateAppUser(username, password, role string) (AppUser, error) {
	if username == "" || password == "" {
		return AppUser{}, errors.New("username and password are required")
	}
	if role == "" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AppUser{}, err
	}
	now := time.Now().Format(time.RFC3339)
	id, err := s.insertReturningID(`INSERT INTO app_users (username, password_hash, role, valid, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`, username, string(hash), role, now, now)
	if err != nil {
		return AppUser{}, err
	}
	return AppUser{ID: id, Username: username, Role: role, Valid: true}, nil
}

func (s *Store) CreateInitialAdmin(username, password string) (AppUser, error) {
	if required, err := s.SetupRequired(); err != nil {
		return AppUser{}, err
	} else if !required {
		return AppUser{}, errors.New("setup already completed")
	}
	return s.CreateAppUser(username, password, "admin")
}

func (s *Store) CreateUserAPIKey(key AuthToken) (AuthToken, error) {
	if key.UserID == 0 || key.Token == "" {
		return AuthToken{}, errors.New("user_id and token are required")
	}
	if key.Name == "" {
		key.Name = "user-key-" + randomToken(5)
	}
	now := time.Now().Format(time.RFC3339)
	id, err := s.insertReturningID(`INSERT INTO user_api_keys (user_id, name, token, valid, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, key.UserID, key.Name, key.Token, boolInt(key.Valid), now, now)
	if err != nil {
		return AuthToken{}, err
	}
	key.ID = id
	return key, nil
}

func (s *Store) VerifyPassword(username, password string) (AppUser, error) {
	var user AppUser
	var hash string
	var valid int
	err := s.queryRow(`SELECT id, username, password_hash, role, valid FROM app_users WHERE username = ?`, username).Scan(&user.ID, &user.Username, &hash, &user.Role, &valid)
	if err != nil {
		return AppUser{}, err
	}
	if valid == 0 {
		return AppUser{}, errors.New("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return AppUser{}, err
	}
	user.Valid = true
	return user, nil
}

func (s *Store) SetupRequired() (bool, error) {
	var count int
	if err := s.queryRow(`SELECT COUNT(1) FROM app_users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Store) CreateSession(token string, userID int64, expiresAt time.Time) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.exec(
		`INSERT INTO app_sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		sessionHash(token), userID, expiresAt.Format(time.RFC3339), now,
	)
	return err
}

func (s *Store) UserBySession(token string) (SessionUser, error) {
	var user AppUser
	var valid int
	var expiresAtText string
	err := s.queryRow(`
		SELECT u.id, u.username, u.role, u.valid, s.expires_at
		FROM app_sessions s
		JOIN app_users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`,
		sessionHash(token), time.Now().Format(time.RFC3339),
	).Scan(&user.ID, &user.Username, &user.Role, &valid, &expiresAtText)
	if err != nil {
		return SessionUser{}, err
	}
	if valid == 0 {
		return SessionUser{}, errors.New("user disabled")
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtText)
	if err != nil {
		return SessionUser{}, err
	}
	user.Valid = true
	return SessionUser{User: user, ExpiresAt: expiresAt}, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.exec(`DELETE FROM app_sessions WHERE token_hash = ?`, sessionHash(token))
	return err
}

func (s *Store) RenewSession(token string, expiresAt time.Time) error {
	_, err := s.exec(`UPDATE app_sessions SET expires_at = ? WHERE token_hash = ?`, expiresAt.Format(time.RFC3339), sessionHash(token))
	return err
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.exec(`DELETE FROM app_sessions WHERE expires_at <= ?`, time.Now().Format(time.RFC3339))
	return err
}

func (s *Store) CreateGoogleAccount(account GoogleAccount) error {
	if account.Email == "" {
		return errors.New("email is required")
	}
	if account.Prefix == "" {
		prefix, err := s.uniqueGoogleAccountPrefix()
		if err != nil {
			return err
		}
		account.Prefix = prefix
	}
	now := time.Now().Format(time.RFC3339)
	_, err := s.exec(`INSERT INTO google_accounts (email, prefix, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, account.Email, account.Prefix, boolInt(account.Enabled), now, now)
	return err
}

func (s *Store) UpdateGoogleAccount(account GoogleAccount) error {
	if account.ID == 0 || account.Email == "" || account.Prefix == "" {
		return errors.New("id, email and prefix are required")
	}
	_, err := s.exec(`UPDATE google_accounts SET email = ?, prefix = ?, enabled = ?, updated_at = ? WHERE id = ?`, account.Email, account.Prefix, boolInt(account.Enabled), time.Now().Format(time.RFC3339), account.ID)
	return err
}

func (s *Store) CreateAPIKey(key APIKey) error {
	if key.AccountID == 0 || key.Key == "" {
		return errors.New("account_id and key are required")
	}
	if key.Name == "" {
		name, err := s.uniqueAPIKeyName(key.AccountID)
		if err != nil {
			return err
		}
		key.Name = name
	}
	now := time.Now().Format(time.RFC3339)
	_, err := s.exec(`INSERT INTO api_keys (google_account_id, name, key, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, key.AccountID, key.Name, key.Key, boolInt(key.Enabled), now, now)
	return err
}

func (s *Store) UpdateAPIKey(key APIKey) error {
	if key.ID == 0 || key.AccountID == 0 || key.Name == "" || key.Key == "" {
		return errors.New("id, account_id, name and key are required")
	}
	_, err := s.exec(`UPDATE api_keys SET google_account_id = ?, name = ?, key = ?, enabled = ?, updated_at = ? WHERE id = ?`, key.AccountID, key.Name, key.Key, boolInt(key.Enabled), time.Now().Format(time.RFC3339), key.ID)
	return err
}

func (s *Store) InsertUsage(record UsageRecord) error {
	return s.InsertUsageBatch([]UsageRecord{record})
}

func (s *Store) InsertUsageBatch(records []UsageRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := s.prepareTx(tx,
		`INSERT INTO usage_logs (user_id, user_name, user_api_key_id, user_api_key_name, api_key_id, api_key_name, google_account_id, google_account_email, model, status, latency_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().Format(time.RFC3339)
	for _, record := range records {
		if _, err := stmt.Exec(
			record.UserID, record.UserName, record.UserAPIKeyID, record.UserAPIKeyName, record.APIKeyID, record.APIKeyName, record.GoogleAccountID, record.GoogleAccountEmail, record.Model, record.Status, record.LatencyMS, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UsageSummary(query UsageQuery) (UsageSummary, error) {
	summary := UsageSummary{ByKey: make(map[string]int64)}
	where, args := usageWhere(query)

	var avg sql.NullFloat64
	if err := s.queryRow(
		`SELECT COUNT(1), COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0), AVG(latency_ms) FROM usage_logs `+where,
		args...,
	).Scan(&summary.Requests, &summary.Errors, &avg); err != nil {
		return summary, err
	}
	if avg.Valid {
		summary.AvgLatencyMS = avg.Float64
	}

	rows, err := s.query(`SELECT COALESCE(NULLIF(user_api_key_name, ''), user_name), COUNT(1) FROM usage_logs `+where+` GROUP BY COALESCE(NULLIF(user_api_key_name, ''), user_name) ORDER BY 2 DESC`, args...)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			rows.Close()
			return summary, err
		}
		summary.ByKey[name] = count
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}

	rows, err = s.query(
		`SELECT substr(created_at, 1, 16), COUNT(1), SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), AVG(latency_ms)
		 FROM usage_logs `+where+` GROUP BY substr(created_at, 1, 16) ORDER BY substr(created_at, 1, 16)`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var point UsagePoint
		var avg sql.NullFloat64
		if err := rows.Scan(&point.Date, &point.Requests, &point.Errors, &avg); err != nil {
			return summary, err
		}
		if avg.Valid {
			point.AvgLatencyMS = avg.Float64
		}
		summary.Series = append(summary.Series, point)
	}
	return summary, rows.Err()
}

func usageWhere(query UsageQuery) (string, []any) {
	clauses := []string{"WHERE user_id = ?", "created_at >= ?", "created_at < ?"}
	args := []any{query.UserID, query.From.Format(time.RFC3339), query.To.Add(time.Minute).Format(time.RFC3339)}
	if query.UserAPIKeyID > 0 {
		clauses = append(clauses, "user_api_key_id = ?")
		args = append(args, query.UserAPIKeyID)
	}
	return strings.Join(clauses, " AND "), args
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

func (s *Store) accountIDByPrefix(prefix string) (int64, error) {
	var id int64
	err := s.queryRow(`SELECT id FROM google_accounts WHERE prefix = ?`, prefix).Scan(&id)
	return id, err
}

func (s *Store) uniqueGoogleAccountPrefix() (string, error) {
	for i := 0; i < 20; i++ {
		prefix := "ga_" + randomToken(6)
		var exists int
		err := s.queryRow(`SELECT COUNT(1) FROM google_accounts WHERE prefix = ?`, prefix).Scan(&exists)
		if err != nil {
			return "", err
		}
		if exists == 0 {
			return prefix, nil
		}
	}
	return "", errors.New("failed to generate unique account id")
}

func (s *Store) uniqueAPIKeyName(accountID int64) (string, error) {
	var prefix string
	if err := s.queryRow(`SELECT prefix FROM google_accounts WHERE id = ?`, accountID).Scan(&prefix); err != nil {
		return "", err
	}
	for i := 0; i < 20; i++ {
		name := prefix + "-" + randomToken(5)
		var exists int
		err := s.queryRow(`SELECT COUNT(1) FROM api_keys WHERE name = ?`, name).Scan(&exists)
		if err != nil {
			return "", err
		}
		if exists == 0 {
			return name, nil
		}
	}
	return "", errors.New("failed to generate unique api key id")
}

func randomToken(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b strings.Builder
	b.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b.WriteByte(alphabet[time.Now().UnixNano()%int64(len(alphabet))])
			continue
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String()
}

func keyPrefix(name string) string {
	if idx := strings.IndexByte(name, '-'); idx > 0 {
		return name[:idx]
	}
	return name
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ReloadRuntime() (map[string]AuthToken, []APIKey, error) {
	tokens, err := s.AuthTokens()
	if err != nil {
		return nil, nil, err
	}
	keys, err := s.APIKeys()
	if err != nil {
		return nil, nil, err
	}
	if len(keys) == 0 {
		return nil, nil, fmt.Errorf("no enabled api keys")
	}
	return tokens, keys, nil
}
