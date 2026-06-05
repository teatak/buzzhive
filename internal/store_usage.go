package buzzhive

import (
	"database/sql"
	"sort"
	"strings"
	"time"
)

func (s *Store) InsertUsageBatch(records []UsageRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := s.prepareTx(tx, `
		INSERT INTO usage_logs (
			user_id,
			user_name,
			user_api_key_id,
			user_api_key_name,
			provider_id,
			provider_name,
			provider_key_id,
			provider_key_name,
			model,
			upstream_model,
			status,
			latency_ms,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	hourlyStmt, err := s.prepareTx(tx, usageStatsUpsertSQL("usage_stats_hourly"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer hourlyStmt.Close()
	dailyStmt, err := s.prepareTx(tx, usageStatsUpsertSQL("usage_stats_daily"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer dailyStmt.Close()

	for _, record := range records {
		createdAt := record.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		if _, err := stmt.Exec(
			record.UserID,
			record.UserName,
			record.UserAPIKeyID,
			record.UserAPIKeyName,
			record.ProviderID,
			record.ProviderName,
			record.ProviderKeyID,
			record.ProviderKeyName,
			record.Model,
			record.UpstreamModel,
			record.Status,
			record.LatencyMS,
			createdAt.Format(time.RFC3339Nano),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := insertUsageStats(hourlyStmt, record, usageHourBucket(createdAt)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := insertUsageStats(dailyStmt, record, usageDayBucket(createdAt)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func usageStatsUpsertSQL(table string) string {
	return `
		INSERT INTO ` + table + ` (
			bucket_start,
			user_id,
			user_name,
			user_api_key_id,
			user_api_key_name,
			model,
			requests,
			errors,
			latency_ms_sum
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_start, user_id, user_api_key_id, model) DO UPDATE SET
			user_name = excluded.user_name,
			user_api_key_name = excluded.user_api_key_name,
			requests = ` + table + `.requests + excluded.requests,
			errors = ` + table + `.errors + excluded.errors,
			latency_ms_sum = ` + table + `.latency_ms_sum + excluded.latency_ms_sum`
}

func insertUsageStats(stmt *sql.Stmt, record UsageRecord, bucketStart string) error {
	errors := 0
	if record.Status >= 400 {
		errors = 1
	}
	_, err := stmt.Exec(
		bucketStart,
		record.UserID,
		record.UserName,
		record.UserAPIKeyID,
		record.UserAPIKeyName,
		record.Model,
		1,
		errors,
		record.LatencyMS,
	)
	return err
}

func (s *Store) backfillUsageStatsIfEmpty() error {
	var hourlyCount, dailyCount int64
	if err := s.queryRow(`SELECT COUNT(1) FROM usage_stats_hourly`).Scan(&hourlyCount); err != nil {
		return err
	}
	if err := s.queryRow(`SELECT COUNT(1) FROM usage_stats_daily`).Scan(&dailyCount); err != nil {
		return err
	}
	if hourlyCount != 0 || dailyCount != 0 {
		return nil
	}
	rows, err := s.query(`
		SELECT
			user_id,
			user_name,
			user_api_key_id,
			user_api_key_name,
			model,
			status,
			latency_ms,
			created_at
		FROM usage_logs
		ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type usageBackfillRecord struct {
		record  UsageRecord
		created time.Time
	}
	var records []usageBackfillRecord
	for rows.Next() {
		var record UsageRecord
		var createdAt string
		if err := rows.Scan(
			&record.UserID,
			&record.UserName,
			&record.UserAPIKeyID,
			&record.UserAPIKeyName,
			&record.Model,
			&record.Status,
			&record.LatencyMS,
			&createdAt,
		); err != nil {
			return err
		}
		created, err := parseStoredUsageTime(createdAt)
		if err != nil {
			continue
		}
		records = append(records, usageBackfillRecord{record: record, created: created})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	hourlyStmt, err := s.prepareTx(tx, usageStatsUpsertSQL("usage_stats_hourly"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer hourlyStmt.Close()
	dailyStmt, err := s.prepareTx(tx, usageStatsUpsertSQL("usage_stats_daily"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer dailyStmt.Close()

	for _, item := range records {
		if err := insertUsageStats(hourlyStmt, item.record, usageHourBucket(item.created)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := insertUsageStats(dailyStmt, item.record, usageDayBucket(item.created)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UsageSummary(query UsageQuery) (UsageSummary, error) {
	if table, bucketMinutes, ok := usageStatsSource(query); ok {
		return s.usageSummaryFromStats(query, table, bucketMinutes)
	}
	where, args := usageWhere(query)
	var summary UsageSummary
	var errors sql.NullInt64
	var avg sql.NullFloat64
	if err := s.queryRow(`
		SELECT
			COUNT(1),
			COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(latency_ms), 0)
		FROM usage_logs
		WHERE `+where,
		args...,
	).Scan(&summary.Requests, &errors, &avg); err != nil {
		return UsageSummary{}, err
	}
	summary.Errors = errors.Int64
	summary.AvgLatencyMS = avg.Float64

	byKey, err := s.usageByKey(where, args)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByKey = byKey

	summary.BucketMinutes = usageBucketMinutes(query.From, query.To)
	series, err := s.usageSeries(where, args, summary.BucketMinutes)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.Series = series
	return summary, nil
}

func (s *Store) usageSummaryFromStats(query UsageQuery, table string, bucketMinutes int) (UsageSummary, error) {
	where, args := usageStatsWhere(query, bucketMinutes)
	var summary UsageSummary
	var requests sql.NullInt64
	var errors sql.NullInt64
	var latencySum sql.NullInt64
	if err := s.queryRow(`
		SELECT
			COALESCE(SUM(requests), 0),
			COALESCE(SUM(errors), 0),
			COALESCE(SUM(latency_ms_sum), 0)
		FROM `+table+`
		WHERE `+where,
		args...,
	).Scan(&requests, &errors, &latencySum); err != nil {
		return UsageSummary{}, err
	}
	summary.Requests = requests.Int64
	summary.Errors = errors.Int64
	if summary.Requests > 0 {
		summary.AvgLatencyMS = float64(latencySum.Int64) / float64(summary.Requests)
	}
	byKey, err := s.usageByKeyFromStats(table, where, args)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByKey = byKey
	series, err := s.usageSeriesFromStats(table, where, args)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.Series = series
	summary.BucketMinutes = bucketMinutes
	return summary, nil
}

func (s *Store) usageByKey(where string, args []any) (map[string]int64, error) {
	rows, err := s.query(`
		SELECT user_api_key_name, COUNT(1)
		FROM usage_logs
		WHERE `+where+`
		GROUP BY user_api_key_name
		ORDER BY COUNT(1) DESC, user_api_key_name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "unknown"
		}
		out[name] = count
	}
	return out, rows.Err()
}

func (s *Store) usageByKeyFromStats(table, where string, args []any) (map[string]int64, error) {
	rows, err := s.query(`
		SELECT user_api_key_name, COALESCE(SUM(requests), 0)
		FROM `+table+`
		WHERE `+where+`
		GROUP BY user_api_key_name
		ORDER BY COALESCE(SUM(requests), 0) DESC, user_api_key_name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "unknown"
		}
		out[name] = count
	}
	return out, rows.Err()
}

func (s *Store) usageSeriesFromStats(table, where string, args []any) ([]UsagePoint, error) {
	rows, err := s.query(`
		SELECT
			bucket_start,
			COALESCE(SUM(requests), 0),
			COALESCE(SUM(errors), 0),
			COALESCE(SUM(latency_ms_sum), 0)
		FROM `+table+`
		WHERE `+where+`
		GROUP BY bucket_start
		ORDER BY bucket_start`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsagePoint
	for rows.Next() {
		var point UsagePoint
		var latencySum int64
		if err := rows.Scan(&point.Date, &point.Requests, &point.Errors, &latencySum); err != nil {
			return nil, err
		}
		if point.Requests > 0 {
			point.AvgLatencyMS = float64(latencySum) / float64(point.Requests)
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (s *Store) usageSeries(where string, args []any, bucketMinutes int) ([]UsagePoint, error) {
	rows, err := s.query(`
		SELECT
			created_at,
			status,
			latency_ms
		FROM usage_logs
		WHERE `+where+`
		ORDER BY created_at`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bucketAgg struct {
		requests   int64
		errors     int64
		latencySum int64
	}
	buckets := make(map[string]bucketAgg)
	for rows.Next() {
		var createdAt string
		var status int
		var latency int64
		if err := rows.Scan(&createdAt, &status, &latency); err != nil {
			return nil, err
		}
		created, err := parseStoredUsageTime(createdAt)
		if err != nil {
			continue
		}
		key := formatUsageBucket(floorUsageBucket(created, bucketMinutes))
		agg := buckets[key]
		agg.requests++
		if status >= 400 {
			agg.errors++
		}
		agg.latencySum += latency
		buckets[key] = agg
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]UsagePoint, 0, len(keys))
	for _, key := range keys {
		agg := buckets[key]
		point := UsagePoint{
			Date:     key,
			Requests: agg.requests,
			Errors:   agg.errors,
		}
		if agg.requests > 0 {
			point.AvgLatencyMS = float64(agg.latencySum) / float64(agg.requests)
		}
		out = append(out, point)
	}
	return out, nil
}

func usageBucketMinutes(from, to time.Time) int {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return 1
	}
	span := to.Sub(from)
	switch {
	case span <= time.Hour:
		return 1
	default:
		return 5
	}
}

func usageStatsSource(query UsageQuery) (string, int, bool) {
	if query.From.IsZero() || query.To.IsZero() || !query.From.Before(query.To) {
		return "", 0, false
	}
	span := query.To.Sub(query.From)
	if span <= 5*time.Hour {
		return "", 0, false
	}
	if span <= 4*24*time.Hour {
		return "usage_stats_hourly", 60, true
	}
	return "usage_stats_daily", 1440, true
}

func usageStatsWhere(query UsageQuery, bucketMinutes int) (string, []any) {
	clauses := []string{"user_id = ?"}
	args := []any{query.UserID}
	if query.UserAPIKeyID != 0 {
		clauses = append(clauses, "user_api_key_id = ?")
		args = append(args, query.UserAPIKeyID)
	}
	if strings.TrimSpace(query.Model) != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, strings.TrimSpace(query.Model))
	}
	if !query.From.IsZero() {
		clauses = append(clauses, "bucket_start >= ?")
		args = append(args, formatUsageBucket(floorUsageBucket(query.From, bucketMinutes)))
	}
	if !query.To.IsZero() {
		clauses = append(clauses, "bucket_start < ?")
		args = append(args, formatUsageBucket(query.To))
	}
	return strings.Join(clauses, " AND "), args
}

func floorUsageBucket(value time.Time, bucketMinutes int) time.Time {
	if bucketMinutes <= 1 {
		return value.Truncate(time.Minute)
	}
	bucket := time.Duration(bucketMinutes) * time.Minute
	return value.Truncate(bucket)
}

func formatUsageBucket(value time.Time) string {
	return value.Format("2006-01-02T15:04")
}

func usageHourBucket(value time.Time) string {
	return formatUsageBucket(floorUsageBucket(value, 60))
}

func usageDayBucket(value time.Time) string {
	return formatUsageBucket(time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location()))
}

func parseStoredUsageTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func usageWhere(query UsageQuery) (string, []any) {
	clauses := []string{"user_id = ?"}
	args := []any{query.UserID}
	if query.UserAPIKeyID != 0 {
		clauses = append(clauses, "user_api_key_id = ?")
		args = append(args, query.UserAPIKeyID)
	}
	if strings.TrimSpace(query.Model) != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, strings.TrimSpace(query.Model))
	}
	if !query.From.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, query.From.Format(time.RFC3339Nano))
	}
	if !query.To.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, query.To.Format(time.RFC3339Nano))
	}
	return strings.Join(clauses, " AND "), args
}
