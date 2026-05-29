package buzzhive

import (
	"database/sql"
	"time"
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
	RequestID          string
	Attempt            int
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
	CreatedAt          time.Time
	ErrorCode          string
	ErrorMessage       string
	ErrorBody          string
}

type UsageSummary struct {
	Requests     int64            `json:"requests"`
	Errors       int64            `json:"errors"`
	AvgLatencyMS float64          `json:"avg_latency_ms"`
	ByKey        map[string]int64 `json:"by_key"`
	Series       []UsagePoint     `json:"series"`
}

type ModelUsageSummary struct {
	TotalByModel  []ModelUsageTotal    `json:"total_by_model"`
	Series        []ModelUsagePoint    `json:"series"`
	AccountTotals []AccountModelUsage  `json:"account_totals"`
	QuotaSignals  []AccountQuotaSignal `json:"quota_signals"`
	RecentErrors  []ModelUsageError    `json:"recent_errors"`
}

type ModelUsageTotal struct {
	Model    string `json:"model"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

type ModelUsagePoint struct {
	Date     string `json:"date"`
	Model    string `json:"model"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

type AccountModelUsage struct {
	AccountEmail string `json:"account_email"`
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	Quota429     int64  `json:"quota_429"`
	DistinctKeys int64  `json:"distinct_keys"`
}

type AccountQuotaSignal struct {
	Date         string `json:"date"`
	AccountEmail string `json:"account_email"`
	Model        string `json:"model"`
	Quota429     int64  `json:"quota_429"`
	DistinctKeys int64  `json:"distinct_keys"`
}

type ModelUsageError struct {
	Date         string `json:"date"`
	RequestID    string `json:"request_id"`
	Attempt      int    `json:"attempt"`
	AccountEmail string `json:"account_email"`
	KeyName      string `json:"key_name"`
	Model        string `json:"model"`
	Status       int    `json:"status"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	ErrorBody    string `json:"error_body"`
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

type ModelUsageQuery struct {
	APIKeyID int64
	From     time.Time
	To       time.Time
}

type SessionUser struct {
	User      AppUser
	ExpiresAt time.Time
}
