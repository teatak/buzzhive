package buzzhive

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) ProviderAPIKeys(providerName string) ([]APIKey, error) {
	rows, err := s.query(`
		SELECT
			pk.id,
			pk.provider_id,
			p.name,
			pk.id,
			pk.name,
			pk.secret,
			pk.enabled,
			pk.disabled_status,
			pk.disabled_error_code,
			pk.disabled_error_message,
			pk.disabled_error_body,
			pk.disabled_at
		FROM provider_keys pk
		JOIN providers p ON p.id = pk.provider_id
		WHERE p.name = ? AND p.enabled = 1 AND pk.enabled = 1
		ORDER BY pk.name`,
		providerName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		var key APIKey
		var enabled int
		var disabledAt sql.NullTime
		if err := rows.Scan(
			&key.ProviderKeyID,
			&key.ProviderID,
			&key.ProviderName,
			&key.ID,
			&key.Name,
			&key.Key,
			&enabled,
			&key.DisabledStatus,
			&key.DisabledErrorCode,
			&key.DisabledErrorMessage,
			&key.DisabledErrorBody,
			&disabledAt,
		); err != nil {
			return nil, err
		}

		key.Enabled = enabled != 0
		key.DisabledAt = formatNullStoreTime(disabledAt)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) EnabledProviders() ([]ProviderRecord, error) {
	rows, err := s.query(`
		SELECT id, name, preset_id, enabled, created_at, updated_at
		FROM providers
		WHERE enabled = 1
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProviderRecord
	for rows.Next() {
		var item ProviderRecord
		var enabled int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.PresetID, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.CreatedAt = formatStoreTime(createdAt)
		item.UpdatedAt = formatStoreTime(updatedAt)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.hydrateProviderEndpoints(out)
}

func normalizeProviderEndpoints(provider ProviderRecord) ([]ProviderEndpoint, error) {
	out := make([]ProviderEndpoint, 0, len(provider.Endpoints))
	seen := make(map[string]bool)
	for _, endpoint := range provider.Endpoints {
		endpoint.Protocol = strings.ToLower(strings.TrimSpace(endpoint.Protocol))
		endpoint.BaseURL = strings.TrimSpace(endpoint.BaseURL)
		if endpoint.Protocol == "" || endpoint.BaseURL == "" {
			continue
		}
		if seen[endpoint.Protocol] {
			return nil, errors.New("duplicate provider endpoint protocol")
		}
		seen[endpoint.Protocol] = true
		out = append(out, endpoint)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one provider endpoint is required")
	}
	return out, nil
}

func (s *Store) hydrateProviderEndpoints(providers []ProviderRecord) ([]ProviderRecord, error) {
	if len(providers) == 0 {
		return providers, nil
	}
	ids := make([]string, len(providers))
	args := make([]any, len(providers))
	indexByID := make(map[int64]int, len(providers))
	for i, provider := range providers {
		ids[i] = "?"
		args[i] = provider.ID
		indexByID[provider.ID] = i
	}
	rows, err := s.query(`
		SELECT id, provider_id, protocol, base_url, enabled
		FROM provider_endpoints
		WHERE provider_id IN (`+strings.Join(ids, ",")+`)
		ORDER BY provider_id, id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var endpoint ProviderEndpoint
		var enabled int
		if err := rows.Scan(&endpoint.ID, &endpoint.ProviderID, &endpoint.Protocol, &endpoint.BaseURL, &enabled); err != nil {
			return nil, err
		}
		endpoint.Enabled = enabled != 0
		if index, ok := indexByID[endpoint.ProviderID]; ok {
			providers[index].Endpoints = append(providers[index].Endpoints, endpoint)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *Store) replaceProviderEndpoints(providerID int64, endpoints []ProviderEndpoint) error {
	if _, err := s.exec(`DELETE FROM provider_endpoints WHERE provider_id = ?`, providerID); err != nil {
		return err
	}
	now := storeNow()
	for _, endpoint := range endpoints {
		if _, err := s.exec(
			`INSERT INTO provider_endpoints (provider_id, protocol, base_url, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			providerID, endpoint.Protocol, endpoint.BaseURL, boolInt(endpoint.Enabled), now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RuntimeProviderAPIKeys() ([]APIKey, error) {
	rows, err := s.query(`
		SELECT
			pk.id,
			pk.provider_id,
			p.name,
			pk.id,
			pk.name,
			pk.secret,
			pk.enabled,
			pk.disabled_status,
			pk.disabled_error_code,
			pk.disabled_error_message,
			pk.disabled_error_body,
			pk.disabled_at
		FROM provider_keys pk
		JOIN providers p ON p.id = pk.provider_id
		WHERE p.enabled = 1 AND pk.enabled = 1
		ORDER BY p.name, pk.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		var key APIKey
		var enabled int
		var disabledAt sql.NullTime
		if err := rows.Scan(
			&key.ProviderKeyID,
			&key.ProviderID,
			&key.ProviderName,
			&key.ID,
			&key.Name,
			&key.Key,
			&enabled,
			&key.DisabledStatus,
			&key.DisabledErrorCode,
			&key.DisabledErrorMessage,
			&key.DisabledErrorBody,
			&disabledAt,
		); err != nil {
			return nil, err
		}

		key.Enabled = enabled != 0
		key.DisabledAt = formatNullStoreTime(disabledAt)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) ResolveModelRoute(publicModel string) (RouteTarget, bool, error) {
	targets, ok, err := s.ResolveModelRoutes(publicModel)
	if err != nil || !ok {
		return RouteTarget{}, ok, err
	}
	return targets[0], true, nil
}

func (s *Store) ResolveModelRoutes(publicModel string) ([]RouteTarget, bool, error) {
	var target RouteTarget
	rows, err := s.query(`
		SELECT
			mr.id,
			m.id,
			m.name,
			m.selection_policy,
			p.id,
			p.name,
			pe.id,
			pe.protocol,
			mr.upstream_model,
			mr.priority,
			mr.weight
		FROM models m
		JOIN model_routes mr ON mr.model_id = m.id
		JOIN providers p ON p.id = mr.provider_id
		JOIN provider_endpoints pe ON pe.provider_id = p.id
		WHERE m.name = ?
			AND m.enabled = 1
			AND mr.enabled = 1
			AND p.enabled = 1
			AND pe.enabled = 1
		ORDER BY mr.priority DESC, mr.weight DESC, mr.id
		`,
		publicModel,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var out []RouteTarget
	for rows.Next() {
		if err := rows.Scan(
			&target.ID,
			&target.ModelID,
			&target.ModelName,
			&target.SelectionPolicy,
			&target.ProviderID,
			&target.ProviderName,
			&target.ProviderEndpointID,
			&target.ProviderType,
			&target.UpstreamModel,
			&target.Priority,
			&target.Weight,
		); err != nil {
			return nil, false, err
		}

		out = append(out, target)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) == 0 {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *Store) providerIDByName(name string) (int64, error) {
	var id int64
	err := s.queryRow(`SELECT id FROM providers WHERE name = ?`, name).Scan(&id)
	return id, err
}

func secretHint(secret string) string {
	if len(secret) <= 4 {
		return secret
	}
	return secret[len(secret)-4:]
}

func (s *Store) DisableProviderKey(id int64, status int, errorCode, errorMessage, errorBody string) error {
	if id == 0 {
		return errors.New("id is required")
	}
	if len(errorBody) > 4096 {
		errorBody = errorBody[:4096]
	}
	now := storeNow()
	_, err := s.exec(
		`UPDATE provider_keys SET enabled = 0, disabled_status = ?, disabled_error_code = ?, disabled_error_message = ?, disabled_error_body = ?, disabled_at = ?, updated_at = ? WHERE id = ?`,
		status, errorCode, errorMessage, errorBody, now, now, id,
	)
	return err
}
