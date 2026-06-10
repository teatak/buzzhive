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
			p.protocols,
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
		var protocolsStr string
		if err := rows.Scan(
			&key.ProviderKeyID,
			&key.ProviderID,
			&key.ProviderName,
			&protocolsStr,
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
		
		protos := splitProtocols(protocolsStr)
		hasOpenAI := false
		for _, proto := range protos {
			if proto == providerOpenAI || proto == providerOpenAIResponses {
				hasOpenAI = true
				key.ProviderType = proto
				break
			}
		}
		if !hasOpenAI && len(protos) > 0 {
			key.ProviderType = protos[0]
		} else if len(protos) == 0 {
			key.ProviderType = ""
		}
		
		key.Enabled = enabled != 0
		key.DisabledAt = formatNullStoreTime(disabledAt)
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) EnabledProviders() ([]ProviderRecord, error) {
	rows, err := s.query(`
		SELECT id, name, preset_id, base_url, protocols, enabled, created_at, updated_at
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
		var protocolsStr string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.PresetID, &item.BaseURL, &protocolsStr, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.Protocols = splitProtocols(protocolsStr)
		item.CreatedAt = formatStoreTime(createdAt)
		item.UpdatedAt = formatStoreTime(updatedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func splitProtocols(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) RuntimeProviderAPIKeys() ([]APIKey, error) {
	rows, err := s.query(`
		SELECT
			pk.id,
			pk.provider_id,
			p.name,
			p.protocols,
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
		var protocolsStr string
		if err := rows.Scan(
			&key.ProviderKeyID,
			&key.ProviderID,
			&key.ProviderName,
			&protocolsStr,
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
		
		protos := splitProtocols(protocolsStr)
		hasOpenAI := false
		for _, proto := range protos {
			if proto == providerOpenAI || proto == providerOpenAIResponses {
				hasOpenAI = true
				key.ProviderType = proto
				break
			}
		}
		if !hasOpenAI && len(protos) > 0 {
			key.ProviderType = protos[0]
		} else if len(protos) == 0 {
			key.ProviderType = ""
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
			p.protocols,
			mr.upstream_model,
			mr.priority,
			mr.weight
		FROM models m
		JOIN model_routes mr ON mr.model_id = m.id
		JOIN providers p ON p.id = mr.provider_id
		WHERE m.name = ?
			AND m.enabled = 1
			AND mr.enabled = 1
			AND p.enabled = 1
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
		var protocolsStr string
		if err := rows.Scan(
			&target.ID,
			&target.ModelID,
			&target.ModelName,
			&target.SelectionPolicy,
			&target.ProviderID,
			&target.ProviderName,
			&protocolsStr,
			&target.UpstreamModel,
			&target.Priority,
			&target.Weight,
		); err != nil {
			return nil, false, err
		}
		
		protos := splitProtocols(protocolsStr)
		hasOpenAI := false
		for _, proto := range protos {
			if proto == providerOpenAI || proto == providerOpenAIResponses {
				hasOpenAI = true
				target.ProviderType = proto
				break
			}
		}
		if !hasOpenAI && len(protos) > 0 {
			target.ProviderType = protos[0]
		} else if len(protos) == 0 {
			target.ProviderType = ""
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
