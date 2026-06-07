package buzzhive

import (
	"database/sql"
	"time"
)

type Store struct {
	db *sql.DB
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	URL    string `yaml:"url"`
}

type RedisConfig struct {
	URL      string `yaml:"url"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type UsageRecord struct {
	UserID           int64
	UserName         string
	UserAPIKeyID     int64
	UserAPIKeyName   string
	ProviderID       int64
	ProviderName     string
	ProviderKeyID    int64
	ProviderKeyName  string
	Model            string
	UpstreamModel    string
	Status           int
	LatencyMS        int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
	ReasoningTokens  int64
	RawUsage         string
	CreatedAt        time.Time
}

type UsageSummary struct {
	Requests         int64            `json:"requests"`
	Errors           int64            `json:"errors"`
	AvgLatencyMS     float64          `json:"avg_latency_ms"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	TotalTokens      int64            `json:"total_tokens"`
	CachedTokens     int64            `json:"cached_tokens"`
	ReasoningTokens  int64            `json:"reasoning_tokens"`
	ByKey            map[string]int64 `json:"by_key"`
	Series           []UsagePoint     `json:"series"`
	BucketMinutes    int              `json:"bucket_minutes"`
	TimeZone         string           `json:"time_zone"`
}

type UsagePoint struct {
	Date             string  `json:"date"`
	Label            string  `json:"label"`
	Tooltip          string  `json:"tooltip"`
	Requests         int64   `json:"requests"`
	Errors           int64   `json:"errors"`
	AvgLatencyMS     float64 `json:"avg_latency_ms"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CachedTokens     int64   `json:"cached_tokens"`
	ReasoningTokens  int64   `json:"reasoning_tokens"`
}

type UsageQuery struct {
	UserID       int64
	UserAPIKeyID int64
	Model        string
	From         time.Time
	To           time.Time
	Location     *time.Location
}

type ProviderRecord struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	PresetID  string `json:"preset_id"`
	BaseURL   string `json:"base_url"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ProviderKey struct {
	ID                int64  `json:"id"`
	ProviderID        int64  `json:"provider_id"`
	Name              string `json:"name"`
	Secret            string `json:"secret,omitempty"`
	SecretHint        string `json:"secret_hint"`
	Enabled           bool   `json:"enabled"`
	Priority          int    `json:"priority"`
	Weight            int    `json:"weight"`
	Labels            string `json:"labels,omitempty"`
	DisabledStatus    int    `json:"disabled_status,omitempty"`
	DisabledErrorCode string `json:"disabled_error_code,omitempty"`
	DisabledMessage   string `json:"disabled_error_message,omitempty"`
	DisabledBody      string `json:"disabled_error_body,omitempty"`
	DisabledAt        string `json:"disabled_at,omitempty"`
	ProviderName      string `json:"provider_name,omitempty"`
}

type Model struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	ContextWindow   int64  `json:"context_window"`
	MaxInputTokens  int64  `json:"max_input_tokens"`
	MaxOutputTokens int64  `json:"max_output_tokens"`
	Capabilities    string `json:"capabilities"`
	SelectionPolicy string `json:"selection_policy"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type ModelRoute struct {
	ID            int64  `json:"id"`
	ModelID       int64  `json:"model_id"`
	ProviderID    int64  `json:"provider_id"`
	UpstreamModel string `json:"upstream_model"`
	QuotaFamily   string `json:"quota_family"`
	Enabled       bool   `json:"enabled"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	ProviderName  string `json:"provider_name,omitempty"`
	ProviderType  string `json:"provider_type,omitempty"`
}

type RouteTarget struct {
	ID              int64
	ModelID         int64
	ModelName       string
	SelectionPolicy string
	ProviderID      int64
	ProviderName    string
	ProviderType    string
	UpstreamModel   string
	QuotaFamily     string
	Priority        int
	Weight          int
}

type RouteSession struct {
	ModelRouteID int64
	ExpiresAt    time.Time
}

func (t RouteTarget) CooldownModel() string {
	model := t.UpstreamModel
	if t.QuotaFamily != "" {
		model = t.QuotaFamily
	}
	if t.ProviderName == "" {
		return model
	}
	return t.ProviderName + ":" + model
}

type SessionUser struct {
	User      AppUser
	ExpiresAt time.Time
}
