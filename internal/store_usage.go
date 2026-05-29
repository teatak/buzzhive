package buzzhive

import (
	"database/sql"
	"strings"
	"time"
)

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

func (s *Store) InsertModelUsage(record UsageRecord) error {
	return s.InsertModelUsageBatch([]UsageRecord{record})
}

func (s *Store) InsertModelUsageBatch(records []UsageRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := s.prepareTx(tx,
		`INSERT INTO model_usage_logs (user_id, user_name, user_api_key_id, user_api_key_name, api_key_id, api_key_name, google_account_id, google_account_email, request_id, attempt, model, status, latency_ms, created_at, error_code, error_message, error_body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, record := range records {
		createdAt := record.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		if _, err := stmt.Exec(
			record.UserID, record.UserName, record.UserAPIKeyID, record.UserAPIKeyName, record.APIKeyID, record.APIKeyName, record.GoogleAccountID, record.GoogleAccountEmail, record.RequestID, record.Attempt, record.Model, record.Status, record.LatencyMS, createdAt.Format(time.RFC3339Nano), record.ErrorCode, record.ErrorMessage, record.ErrorBody,
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

func (s *Store) ModelUsageSummary(query ModelUsageQuery) (ModelUsageSummary, error) {
	var summary ModelUsageSummary
	where, args := modelUsageWhere(query)

	rows, err := s.query(
		`SELECT model, COUNT(1), COALESCE(SUM(CASE WHEN status >= 400 OR status = 0 THEN 1 ELSE 0 END), 0)
		 FROM model_usage_logs `+where+` GROUP BY model ORDER BY 2 DESC`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var item ModelUsageTotal
		if err := rows.Scan(&item.Model, &item.Requests, &item.Errors); err != nil {
			rows.Close()
			return summary, err
		}
		summary.TotalByModel = append(summary.TotalByModel, item)
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}

	rows, err = s.query(
		`SELECT google_account_email, model, COUNT(1), COALESCE(SUM(CASE WHEN status = 429 THEN 1 ELSE 0 END), 0), COUNT(DISTINCT api_key_name)
		 FROM model_usage_logs `+where+` GROUP BY google_account_email, model ORDER BY 4 DESC, 3 DESC`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var item AccountModelUsage
		if err := rows.Scan(&item.AccountEmail, &item.Model, &item.Requests, &item.Quota429, &item.DistinctKeys); err != nil {
			rows.Close()
			return summary, err
		}
		summary.AccountTotals = append(summary.AccountTotals, item)
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}

	rows, err = s.query(
		`SELECT substr(created_at, 1, 16), google_account_email, model, COUNT(1), COUNT(DISTINCT api_key_name)
		 FROM model_usage_logs `+where+` AND status = 429
		 GROUP BY substr(created_at, 1, 16), google_account_email, model
		 HAVING COUNT(DISTINCT api_key_name) > 1
		 ORDER BY substr(created_at, 1, 16) DESC, 4 DESC
		 LIMIT 50`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var item AccountQuotaSignal
		if err := rows.Scan(&item.Date, &item.AccountEmail, &item.Model, &item.Quota429, &item.DistinctKeys); err != nil {
			rows.Close()
			return summary, err
		}
		summary.QuotaSignals = append(summary.QuotaSignals, item)
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}

	rows, err = s.query(
		`SELECT created_at, request_id, attempt, google_account_email, api_key_name, model, status, error_code, error_message, error_body
		 FROM model_usage_logs `+where+` AND (status >= 400 OR status = 0)
		 ORDER BY created_at DESC
		 LIMIT 30`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var item ModelUsageError
		if err := rows.Scan(&item.Date, &item.RequestID, &item.Attempt, &item.AccountEmail, &item.KeyName, &item.Model, &item.Status, &item.ErrorCode, &item.ErrorMessage, &item.ErrorBody); err != nil {
			rows.Close()
			return summary, err
		}
		summary.RecentErrors = append(summary.RecentErrors, item)
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}

	rows, err = s.query(
		`SELECT substr(created_at, 1, 16), model, COUNT(1), COALESCE(SUM(CASE WHEN status >= 400 OR status = 0 THEN 1 ELSE 0 END), 0)
		 FROM model_usage_logs `+where+` GROUP BY substr(created_at, 1, 16), model ORDER BY substr(created_at, 1, 16), model`,
		args...,
	)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var point ModelUsagePoint
		if err := rows.Scan(&point.Date, &point.Model, &point.Requests, &point.Errors); err != nil {
			return summary, err
		}
		summary.Series = append(summary.Series, point)
	}
	return summary, rows.Err()
}

func usageWhere(query UsageQuery) (string, []any) {
	clauses := []string{"WHERE user_id = ?", "created_at >= ?", "created_at < ?"}
	args := []any{query.UserID, query.From.Format(time.RFC3339), query.To.Format(time.RFC3339)}
	if query.UserAPIKeyID > 0 {
		clauses = append(clauses, "user_api_key_id = ?")
		args = append(args, query.UserAPIKeyID)
	}
	return strings.Join(clauses, " AND "), args
}

func modelUsageWhere(query ModelUsageQuery) (string, []any) {
	clauses := []string{"WHERE created_at >= ?", "created_at < ?"}
	args := []any{query.From.Format(time.RFC3339), query.To.Format(time.RFC3339)}
	if query.APIKeyID > 0 {
		clauses = append(clauses, "api_key_id = ?")
		args = append(args, query.APIKeyID)
	}
	return strings.Join(clauses, " AND "), args
}
