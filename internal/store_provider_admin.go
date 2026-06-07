package buzzhive

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) Providers() ([]ProviderRecord, error) {
	rows, err := s.query(`
		SELECT id, name, type, preset_id, base_url, enabled, created_at, updated_at
		FROM providers
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProviderRecord, 0)
	for rows.Next() {
		var item ProviderRecord
		var enabled int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.PresetID, &item.BaseURL, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.CreatedAt = formatStoreTime(createdAt)
		item.UpdatedAt = formatStoreTime(updatedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Provider(id int64) (ProviderRecord, error) {
	var item ProviderRecord
	var enabled int
	var createdAt, updatedAt time.Time
	err := s.queryRow(`
		SELECT id, name, type, preset_id, base_url, enabled, created_at, updated_at
		FROM providers
		WHERE id = ?`,
		id,
	).Scan(&item.ID, &item.Name, &item.Type, &item.PresetID, &item.BaseURL, &enabled, &createdAt, &updatedAt)
	item.Enabled = enabled != 0
	item.CreatedAt = formatStoreTime(createdAt)
	item.UpdatedAt = formatStoreTime(updatedAt)
	return item, err
}

func (s *Store) CreateProvider(provider ProviderRecord) (ProviderRecord, error) {
	if provider.Name == "" || provider.Type == "" || provider.BaseURL == "" {
		return ProviderRecord{}, errors.New("name, type and base_url are required")
	}
	if provider.PresetID == "" {
		provider.PresetID = provider.Type
	}
	now := storeNow()
	id, err := s.insertReturningID(
		`INSERT INTO providers (name, type, preset_id, base_url, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		provider.Name, provider.Type, provider.PresetID, provider.BaseURL, boolInt(provider.Enabled), now, now,
	)
	if err != nil {
		return ProviderRecord{}, err
	}
	return s.Provider(id)
}

func (s *Store) UpdateProvider(provider ProviderRecord) (ProviderRecord, error) {
	if provider.ID == 0 || provider.Name == "" || provider.Type == "" || provider.BaseURL == "" {
		return ProviderRecord{}, errors.New("id, name, type and base_url are required")
	}
	if provider.PresetID == "" {
		provider.PresetID = provider.Type
	}
	_, err := s.exec(
		`UPDATE providers SET name = ?, type = ?, preset_id = ?, base_url = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		provider.Name, provider.Type, provider.PresetID, provider.BaseURL, boolInt(provider.Enabled), storeNow(), provider.ID,
	)
	if err != nil {
		return ProviderRecord{}, err
	}
	return s.Provider(provider.ID)
}

func (s *Store) DeleteProvider(id int64) error {
	if id == 0 {
		return errors.New("id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM model_routes WHERE provider_id = ?`,
		`DELETE FROM provider_keys WHERE provider_id = ?`,
		`DELETE FROM providers WHERE id = ?`,
	} {
		if _, err := tx.Exec(s.rebind(stmt), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ProviderKey(id int64, reveal bool) (ProviderKey, error) {
	keys, err := s.providerKeysByFilter(id, 0, reveal)
	if err != nil {
		return ProviderKey{}, err
	}
	if len(keys) == 0 {
		return ProviderKey{}, sql.ErrNoRows
	}
	return keys[0], nil
}

func (s *Store) ProviderKeys(providerID int64, reveal bool) ([]ProviderKey, error) {
	return s.providerKeysByFilter(0, providerID, reveal)
}

func (s *Store) providerKeysByFilter(id, providerID int64, reveal bool) ([]ProviderKey, error) {
	rows, err := s.query(`
		SELECT
			pk.id,
			pk.provider_id,
			p.name,
			pk.name,
			pk.secret,
			pk.secret_hint,
			pk.enabled,
			pk.priority,
			pk.weight,
			pk.labels,
			pk.disabled_status,
			pk.disabled_error_code,
			pk.disabled_error_message,
			pk.disabled_error_body,
			pk.disabled_at
		FROM provider_keys pk
		JOIN providers p ON p.id = pk.provider_id
		WHERE (? = 0 OR pk.id = ?)
			AND (? = 0 OR pk.provider_id = ?)
		ORDER BY p.name, pk.name`,
		id, id, providerID, providerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProviderKey, 0)
	for rows.Next() {
		var item ProviderKey
		var enabled int
		var disabledAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.ProviderID,
			&item.ProviderName,
			&item.Name,
			&item.Secret,
			&item.SecretHint,
			&enabled,
			&item.Priority,
			&item.Weight,
			&item.Labels,
			&item.DisabledStatus,
			&item.DisabledErrorCode,
			&item.DisabledMessage,
			&item.DisabledBody,
			&disabledAt,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.DisabledAt = formatNullStoreTime(disabledAt)
		if !reveal {
			item.Secret = maskSecret(item.Secret)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateProviderKey(key ProviderKey) (ProviderKey, error) {
	if key.ProviderID == 0 || key.Secret == "" {
		return ProviderKey{}, errors.New("provider_id and secret are required")
	}
	if _, err := s.Provider(key.ProviderID); err != nil {
		return ProviderKey{}, err
	}
	if key.Name == "" {
		name, err := s.uniqueProviderKeyName(key.ProviderID)
		if err != nil {
			return ProviderKey{}, err
		}
		key.Name = name
	}
	if key.Weight == 0 {
		key.Weight = 1
	}
	now := storeNow()
	id, err := s.insertReturningID(
		`INSERT INTO provider_keys (provider_id, name, secret, secret_hint, enabled, priority, weight, labels, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ProviderID, key.Name, key.Secret, secretHint(key.Secret), boolInt(key.Enabled), key.Priority, key.Weight, key.Labels, now, now,
	)
	if err != nil {
		return ProviderKey{}, err
	}
	return s.ProviderKey(id, false)
}

func (s *Store) UpdateProviderKey(key ProviderKey) (ProviderKey, error) {
	if key.ID == 0 || key.ProviderID == 0 || key.Name == "" {
		return ProviderKey{}, errors.New("id, provider_id and name are required")
	}
	if _, err := s.Provider(key.ProviderID); err != nil {
		return ProviderKey{}, err
	}
	if key.Weight == 0 {
		key.Weight = 1
	}
	now := storeNow()
	var err error
	if key.Secret == "" {
		_, err = s.exec(
			`UPDATE provider_keys SET provider_id = ?, name = ?, enabled = ?, priority = ?, weight = ?, labels = ?, disabled_status = CASE WHEN ? = 1 THEN 0 ELSE disabled_status END, disabled_error_code = CASE WHEN ? = 1 THEN '' ELSE disabled_error_code END, disabled_error_message = CASE WHEN ? = 1 THEN '' ELSE disabled_error_message END, disabled_error_body = CASE WHEN ? = 1 THEN '' ELSE disabled_error_body END, disabled_at = CASE WHEN ? = 1 THEN NULL ELSE disabled_at END, updated_at = ? WHERE id = ?`,
			key.ProviderID, key.Name, boolInt(key.Enabled), key.Priority, key.Weight, key.Labels, boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), now, key.ID,
		)
	} else {
		_, err = s.exec(
			`UPDATE provider_keys SET provider_id = ?, name = ?, secret = ?, secret_hint = ?, enabled = ?, priority = ?, weight = ?, labels = ?, disabled_status = CASE WHEN ? = 1 THEN 0 ELSE disabled_status END, disabled_error_code = CASE WHEN ? = 1 THEN '' ELSE disabled_error_code END, disabled_error_message = CASE WHEN ? = 1 THEN '' ELSE disabled_error_message END, disabled_error_body = CASE WHEN ? = 1 THEN '' ELSE disabled_error_body END, disabled_at = CASE WHEN ? = 1 THEN NULL ELSE disabled_at END, updated_at = ? WHERE id = ?`,
			key.ProviderID, key.Name, key.Secret, secretHint(key.Secret), boolInt(key.Enabled), key.Priority, key.Weight, key.Labels, boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), boolInt(key.Enabled), now, key.ID,
		)
	}
	if err != nil {
		return ProviderKey{}, err
	}
	return s.ProviderKey(key.ID, false)
}

func (s *Store) DeleteProviderKeys(ids []int64) error {
	if len(ids) == 0 {
		return errors.New("id is required")
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := s.exec(`DELETE FROM provider_keys WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	return err
}

func (s *Store) Models() ([]Model, error) {
	rows, err := s.query(`
		SELECT id, name, display_name, description, context_window, max_input_tokens, max_output_tokens, capabilities, selection_policy, enabled, created_at, updated_at
		FROM models
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Model, 0)
	for rows.Next() {
		var item Model
		var enabled int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.DisplayName, &item.Description, &item.ContextWindow, &item.MaxInputTokens, &item.MaxOutputTokens, &item.Capabilities, &item.SelectionPolicy, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.CreatedAt = formatStoreTime(createdAt)
		item.UpdatedAt = formatStoreTime(updatedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Model(id int64) (Model, error) {
	var item Model
	var enabled int
	var createdAt, updatedAt time.Time
	err := s.queryRow(`
		SELECT id, name, display_name, description, context_window, max_input_tokens, max_output_tokens, capabilities, selection_policy, enabled, created_at, updated_at
		FROM models
		WHERE id = ?`,
		id,
	).Scan(&item.ID, &item.Name, &item.DisplayName, &item.Description, &item.ContextWindow, &item.MaxInputTokens, &item.MaxOutputTokens, &item.Capabilities, &item.SelectionPolicy, &enabled, &createdAt, &updatedAt)
	item.Enabled = enabled != 0
	item.CreatedAt = formatStoreTime(createdAt)
	item.UpdatedAt = formatStoreTime(updatedAt)
	return item, err
}

func (s *Store) CreateModel(model Model) (Model, error) {
	if model.Name == "" {
		return Model{}, errors.New("name is required")
	}
	if model.DisplayName == "" {
		model.DisplayName = model.Name
	}
	if model.Capabilities == "" {
		model.Capabilities = "{}"
	}
	if model.SelectionPolicy == "" {
		model.SelectionPolicy = "round_robin"
	}
	now := storeNow()
	id, err := s.insertReturningID(
		`INSERT INTO models (name, display_name, description, context_window, max_input_tokens, max_output_tokens, capabilities, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		model.Name, model.DisplayName, model.Description, model.ContextWindow, model.MaxInputTokens, model.MaxOutputTokens, model.Capabilities, model.SelectionPolicy, boolInt(model.Enabled), now, now,
	)
	if err != nil {
		return Model{}, err
	}
	return s.Model(id)
}

func (s *Store) UpdateModel(model Model) (Model, error) {
	if model.ID == 0 || model.Name == "" {
		return Model{}, errors.New("id and name are required")
	}
	if model.Capabilities == "" {
		model.Capabilities = "{}"
	}
	if model.SelectionPolicy == "" {
		model.SelectionPolicy = "round_robin"
	}
	_, err := s.exec(
		`UPDATE models SET name = ?, display_name = ?, description = ?, context_window = ?, max_input_tokens = ?, max_output_tokens = ?, capabilities = ?, selection_policy = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		model.Name, model.DisplayName, model.Description, model.ContextWindow, model.MaxInputTokens, model.MaxOutputTokens, model.Capabilities, model.SelectionPolicy, boolInt(model.Enabled), storeNow(), model.ID,
	)
	if err != nil {
		return Model{}, err
	}
	return s.Model(model.ID)
}

func (s *Store) DeleteModel(id int64) error {
	if id == 0 {
		return errors.New("id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(s.rebind(`DELETE FROM model_routes WHERE model_id = ?`), id); err != nil {
		return err
	}
	if _, err := tx.Exec(s.rebind(`DELETE FROM models WHERE id = ?`), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ModelRoutes(modelID int64) ([]ModelRoute, error) {
	rows, err := s.query(`
		SELECT
			mr.id,
			mr.model_id,
			mr.provider_id,
			p.name,
			p.type,
			mr.upstream_model,
			mr.quota_family,
			mr.enabled,
			mr.priority,
			mr.weight
		FROM model_routes mr
		JOIN providers p ON p.id = mr.provider_id
		WHERE ? = 0 OR mr.model_id = ?
		ORDER BY mr.model_id, mr.priority DESC, mr.weight DESC, mr.id`,
		modelID, modelID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ModelRoute, 0)
	for rows.Next() {
		var item ModelRoute
		var enabled int
		if err := rows.Scan(
			&item.ID,
			&item.ModelID,
			&item.ProviderID,
			&item.ProviderName,
			&item.ProviderType,
			&item.UpstreamModel,
			&item.QuotaFamily,
			&enabled,
			&item.Priority,
			&item.Weight,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ModelRoute(id int64) (ModelRoute, error) {
	routes, err := s.ModelRoutes(0)
	if err != nil {
		return ModelRoute{}, err
	}
	for _, route := range routes {
		if route.ID == id {
			return route, nil
		}
	}
	return ModelRoute{}, sql.ErrNoRows
}

func (s *Store) CreateModelRoute(route ModelRoute) (ModelRoute, error) {
	if route.ModelID == 0 || route.ProviderID == 0 || route.UpstreamModel == "" {
		return ModelRoute{}, errors.New("model_id, provider_id and upstream_model are required")
	}
	if _, err := s.Model(route.ModelID); err != nil {
		return ModelRoute{}, err
	}
	if _, err := s.Provider(route.ProviderID); err != nil {
		return ModelRoute{}, err
	}
	if route.Weight == 0 {
		route.Weight = 1
	}
	now := storeNow()
	id, err := s.insertReturningID(
		`INSERT INTO model_routes (model_id, provider_id, upstream_model, quota_family, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		route.ModelID, route.ProviderID, route.UpstreamModel, route.QuotaFamily, boolInt(route.Enabled), route.Priority, route.Weight, now, now,
	)
	if err != nil {
		return ModelRoute{}, err
	}
	return s.ModelRoute(id)
}

func (s *Store) UpdateModelRoute(route ModelRoute) (ModelRoute, error) {
	if route.ID == 0 || route.ModelID == 0 || route.ProviderID == 0 || route.UpstreamModel == "" {
		return ModelRoute{}, errors.New("id, model_id, provider_id and upstream_model are required")
	}
	if _, err := s.Model(route.ModelID); err != nil {
		return ModelRoute{}, err
	}
	if _, err := s.Provider(route.ProviderID); err != nil {
		return ModelRoute{}, err
	}
	if route.Weight == 0 {
		route.Weight = 1
	}
	_, err := s.exec(
		`UPDATE model_routes SET model_id = ?, provider_id = ?, upstream_model = ?, quota_family = ?, enabled = ?, priority = ?, weight = ?, updated_at = ? WHERE id = ?`,
		route.ModelID, route.ProviderID, route.UpstreamModel, route.QuotaFamily, boolInt(route.Enabled), route.Priority, route.Weight, storeNow(), route.ID,
	)
	if err != nil {
		return ModelRoute{}, err
	}
	return s.ModelRoute(route.ID)
}

func (s *Store) DeleteModelRoute(id int64) error {
	if id == 0 {
		return errors.New("id is required")
	}
	_, err := s.exec(`DELETE FROM model_routes WHERE id = ?`, id)
	return err
}

func (s *Store) uniqueProviderKeyName(providerID int64) (string, error) {
	var providerName string
	if err := s.queryRow(`SELECT name FROM providers WHERE id = ?`, providerID).Scan(&providerName); err != nil {
		return "", err
	}
	prefix := providerKeyNamePrefix(providerName)
	for i := 0; i < 20; i++ {
		name := prefix + "-" + randomToken(5)
		var exists int
		err := s.queryRow(`SELECT COUNT(1) FROM provider_keys WHERE provider_id = ? AND name = ?`, providerID, name).Scan(&exists)
		if err != nil {
			return "", err
		}
		if exists == 0 {
			return name, nil
		}
	}
	return "", errors.New("failed to generate unique provider key name")
}

func providerKeyNamePrefix(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "pk"
	}
	if len(out) > 16 {
		return out[:16]
	}
	return out
}
