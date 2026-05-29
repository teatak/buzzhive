package buzzhive

import (
	"fmt"
	"time"
)

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
