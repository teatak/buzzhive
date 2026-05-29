package buzzhive

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

var corsHeaders = map[string]string{
	"Access-Control-Allow-Origin":  "*",
	"Access-Control-Allow-Methods": "GET, HEAD, POST, PUT, DELETE, OPTIONS",
	"Access-Control-Allow-Headers": "Content-Type, Authorization, x-goog-api-key",
	"X-Proxy-Version":              "local-go-v1",
}

const (
	adminSessionTTL         = 7 * 24 * time.Hour
	adminSessionRenewBefore = 3 * 24 * time.Hour
)

type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Upstream struct {
		BaseURL string `yaml:"base_url"`
		Timeout string `yaml:"timeout"`
	} `yaml:"upstream"`
	Database DatabaseConfig `yaml:"database"`
	Auth     struct {
		Tokens []AuthToken `yaml:"tokens"`
	} `yaml:"auth"`
	KeyAccounts map[string]string `yaml:"key_accounts"`
	Models      struct {
		Auto []string `yaml:"auto"`
	} `yaml:"models"`
	Retry struct {
		MaxAttempts     int `yaml:"max_attempts"`
		CooldownSeconds int `yaml:"cooldown_seconds"`
	} `yaml:"retry"`
	GeminiAPIKeys []APIKey `yaml:"gemini_api_keys"`
}

type AuthToken struct {
	ID       int64  `yaml:"-" json:"id"`
	UserID   int64  `yaml:"-" json:"user_id"`
	UserName string `yaml:"-" json:"user_name,omitempty"`
	Name     string `yaml:"name" json:"name"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Valid    bool   `yaml:"valid" json:"valid"`
}

type APIKey struct {
	ID                   int64  `yaml:"-" json:"id"`
	AccountID            int64  `yaml:"-" json:"account_id"`
	Name                 string `yaml:"name" json:"name"`
	Key                  string `yaml:"key" json:"key,omitempty"`
	Enabled              bool   `yaml:"-" json:"enabled"`
	AccountEmail         string `yaml:"-" json:"account_email,omitempty"`
	AccountPrefix        string `yaml:"-" json:"account_prefix,omitempty"`
	DisabledStatus       int    `yaml:"-" json:"disabled_status,omitempty"`
	DisabledErrorCode    string `yaml:"-" json:"disabled_error_code,omitempty"`
	DisabledErrorMessage string `yaml:"-" json:"disabled_error_message,omitempty"`
	DisabledErrorBody    string `yaml:"-" json:"disabled_error_body,omitempty"`
	DisabledAt           string `yaml:"-" json:"disabled_at,omitempty"`
}

type KeyError struct {
	Key       string `json:"key"`
	Model     string `json:"model"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updated_at"`
}

type Stats struct {
	StartedAt   time.Time           `json:"started_at"`
	Requests    int64               `json:"requests"`
	ByUser      map[string]int64    `json:"by_user"`
	ByKey       map[string]int64    `json:"by_key"`
	Exhausted   map[string]string   `json:"exhausted"`
	KeyErrors   map[string]KeyError `json:"key_errors"`
	LastUpdated time.Time           `json:"last_updated"`
}

type KeyState struct {
	keys      []APIKey
	next      int
	cooldown  time.Duration
	exhausted map[string]time.Time
	errors    map[string]KeyError
	mu        sync.Mutex
}

type Server struct {
	cfg          Config
	adminDir     string
	store        *Store
	upstream     *url.URL
	client       *http.Client
	authTokens   map[string]AuthToken
	sessions     map[string]SessionUser
	keyState     *KeyState
	usageCh      chan UsageRecord
	modelUsageCh chan UsageRecord
	stats        Stats
	statsMu      sync.Mutex
	runtimeMu    sync.Mutex
}

type AdminConfig struct {
	Addr            string     `json:"addr"`
	UpstreamBaseURL string     `json:"upstream_base_url"`
	Timeout         string     `json:"timeout"`
	MaxAttempts     int        `json:"max_attempts"`
	CooldownSeconds int        `json:"cooldown_seconds"`
	Models          []string   `json:"models"`
	Keys            []AdminKey `json:"keys"`
	Accounts        []Account  `json:"accounts"`
	Tokens          []string   `json:"tokens"`
}

type AdminData struct {
	Config      AdminConfig     `json:"config"`
	Users       []AppUser       `json:"users"`
	UserAPIKeys []AuthToken     `json:"user_api_keys"`
	Accounts    []GoogleAccount `json:"accounts"`
	Keys        []APIKey        `json:"keys"`
}

type AppUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Valid    bool   `json:"valid"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Account struct {
	Email  string `json:"email"`
	Prefix string `json:"prefix"`
}

type GoogleAccount struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Prefix  string `json:"prefix"`
	Enabled bool   `json:"enabled"`
}

type AdminKey struct {
	Name          string `json:"name"`
	Key           string `json:"key"`
	AccountEmail  string `json:"account_email"`
	AccountPrefix string `json:"account_prefix"`
}
