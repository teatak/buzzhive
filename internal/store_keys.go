package buzzhive

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

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
		SELECT k.id, k.google_account_id, k.name, k.key, k.enabled, a.email, a.prefix,
			k.disabled_status, k.disabled_error_code, k.disabled_error_message, k.disabled_error_body, k.disabled_at
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
		SELECT k.id, k.google_account_id, k.name, k.key, k.enabled, a.email, a.prefix,
			k.disabled_status, k.disabled_error_code, k.disabled_error_message, k.disabled_error_body, k.disabled_at
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
		if err := rows.Scan(
			&key.ID,
			&key.AccountID,
			&key.Name,
			&key.Key,
			&enabled,
			&key.AccountEmail,
			&key.AccountPrefix,
			&key.DisabledStatus,
			&key.DisabledErrorCode,
			&key.DisabledErrorMessage,
			&key.DisabledErrorBody,
			&key.DisabledAt,
		); err != nil {
			return nil, err
		}
		key.Enabled = enabled != 0
		out = append(out, key)
	}
	return out, rows.Err()
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

func (s *Store) DeleteGoogleAccount(id int64) error {
	if id == 0 {
		return errors.New("id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	deleteKeys, err := s.prepareTx(tx, `DELETE FROM api_keys WHERE google_account_id = ?`)
	if err != nil {
		return err
	}
	defer deleteKeys.Close()
	if _, err := deleteKeys.Exec(id); err != nil {
		return err
	}
	deleteAccount, err := s.prepareTx(tx, `DELETE FROM google_accounts WHERE id = ?`)
	if err != nil {
		return err
	}
	defer deleteAccount.Close()
	if _, err := deleteAccount.Exec(id); err != nil {
		return err
	}
	return tx.Commit()
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
	if key.Enabled {
		_, err := s.exec(
			`UPDATE api_keys SET google_account_id = ?, name = ?, key = ?, enabled = 1, disabled_status = 0, disabled_error_code = '', disabled_error_message = '', disabled_error_body = '', disabled_at = '', updated_at = ? WHERE id = ?`,
			key.AccountID, key.Name, key.Key, time.Now().Format(time.RFC3339), key.ID,
		)
		return err
	}
	_, err := s.exec(`UPDATE api_keys SET google_account_id = ?, name = ?, key = ?, enabled = ?, updated_at = ? WHERE id = ?`, key.AccountID, key.Name, key.Key, boolInt(key.Enabled), time.Now().Format(time.RFC3339), key.ID)
	return err
}

func (s *Store) SetAPIKeyEnabled(id int64, enabled bool) error {
	if id == 0 {
		return errors.New("id is required")
	}
	if enabled {
		_, err := s.exec(
			`UPDATE api_keys SET enabled = 1, disabled_status = 0, disabled_error_code = '', disabled_error_message = '', disabled_error_body = '', disabled_at = '', updated_at = ? WHERE id = ?`,
			time.Now().Format(time.RFC3339), id,
		)
		return err
	}
	_, err := s.exec(`UPDATE api_keys SET enabled = 0, updated_at = ? WHERE id = ?`, time.Now().Format(time.RFC3339), id)
	return err
}

func (s *Store) DisableAPIKey(id int64, status int, errorCode, errorMessage, errorBody string) error {
	if id == 0 {
		return errors.New("id is required")
	}
	if len(errorBody) > 4096 {
		errorBody = errorBody[:4096]
	}
	now := time.Now().Format(time.RFC3339)
	_, err := s.exec(
		`UPDATE api_keys SET enabled = 0, disabled_status = ?, disabled_error_code = ?, disabled_error_message = ?, disabled_error_body = ?, disabled_at = ?, updated_at = ? WHERE id = ?`,
		status, errorCode, errorMessage, errorBody, now, now, id,
	)
	return err
}

func (s *Store) backfillDisabledAPIKeyReasons() error {
	rows, err := s.query(`
		SELECT k.id, l.status, l.error_code, l.error_message, l.error_body, l.created_at
		FROM api_keys k
		JOIN model_usage_logs l ON l.api_key_id = k.id
		WHERE k.enabled = 0 AND k.disabled_status = 0 AND l.status IN (400, 401, 403)
		ORDER BY k.id, l.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := make(map[int64]bool)
	for rows.Next() {
		var id int64
		var status int
		var errorCode, errorMessage, errorBody, createdAt string
		if err := rows.Scan(&id, &status, &errorCode, &errorMessage, &errorBody, &createdAt); err != nil {
			return err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		if len(errorBody) > 4096 {
			errorBody = errorBody[:4096]
		}
		if _, err := s.exec(
			`UPDATE api_keys SET disabled_status = ?, disabled_error_code = ?, disabled_error_message = ?, disabled_error_body = ?, disabled_at = ? WHERE id = ?`,
			status, errorCode, errorMessage, errorBody, createdAt, id,
		); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) DeleteAPIKeys(ids []int64) error {
	if len(ids) == 0 {
		return errors.New("id is required")
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := s.exec(`DELETE FROM api_keys WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
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
