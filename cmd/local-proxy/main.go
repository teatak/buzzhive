package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
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

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	adminDir := flag.String("admin-dir", "admin/dist", "built admin frontend directory")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	srv, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	srv.adminDir = *adminDir

	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv,
		ReadHeaderTimeout: 15 * time.Second,
	}

	log.Printf("local Gemini proxy listening on http://%s", cfg.Server.Addr)
	log.Fatal(httpServer.ListenAndServe())
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = "127.0.0.1:9622"
	}
	if cfg.Upstream.BaseURL == "" {
		cfg.Upstream.BaseURL = "https://generativelanguage.googleapis.com"
	}
	if cfg.Upstream.Timeout == "" {
		cfg.Upstream.Timeout = "10m"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.Driver == "sqlite" && cfg.Database.Path == "" {
		cfg.Database.Path = "data/buzzhive.db"
	}
	if envURL := os.Getenv("BUZZHIVE_DATABASE_URL"); envURL != "" {
		cfg.Database.Driver = "postgres"
		cfg.Database.URL = envURL
	}
	if cfg.Retry.MaxAttempts <= 0 {
		cfg.Retry.MaxAttempts = 8
	}
	if cfg.Retry.CooldownSeconds <= 0 {
		cfg.Retry.CooldownSeconds = 60
	}
	if len(cfg.Models.Auto) == 0 {
		cfg.Models.Auto = []string{"gemini-3.5-flash", "gemini-3-flash-preview", "gemini-3.1-flash-lite"}
	}
	return cfg, nil
}

func newServer(cfg Config) (*Server, error) {
	upstream, err := url.Parse(cfg.Upstream.BaseURL)
	if err != nil {
		return nil, err
	}
	timeout, err := time.ParseDuration(cfg.Upstream.Timeout)
	if err != nil {
		return nil, err
	}

	store, err := OpenStore(cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := store.Seed(cfg); err != nil {
		return nil, err
	}
	authTokens, err := store.AuthTokens()
	if err != nil {
		return nil, err
	}
	apiKeys, err := store.APIKeys()
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 200
	transport.MaxIdleConnsPerHost = 100
	transport.IdleConnTimeout = 90 * time.Second
	transport.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	srv := &Server{
		cfg:          cfg,
		store:        store,
		upstream:     upstream,
		client:       &http.Client{Timeout: timeout, Transport: transport},
		authTokens:   authTokens,
		sessions:     make(map[string]SessionUser),
		usageCh:      make(chan UsageRecord, 4096),
		modelUsageCh: make(chan UsageRecord, 4096),
		keyState: &KeyState{
			keys:      apiKeys,
			cooldown:  time.Duration(cfg.Retry.CooldownSeconds) * time.Second,
			exhausted: make(map[string]time.Time),
			errors:    make(map[string]KeyError),
		},
		stats: Stats{
			StartedAt: time.Now(),
			ByUser:    make(map[string]int64),
			ByKey:     make(map[string]int64),
			Exhausted: make(map[string]string),
			KeyErrors: make(map[string]KeyError),
		},
	}
	go srv.usageWriter()
	go srv.modelUsageWriter()
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w.Header())
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.URL.Path == "/" {
		http.Redirect(w, r, "/admin/", http.StatusFound)
		return
	}

	if r.URL.Path == "/admin" || r.URL.Path == "/admin/" || strings.HasPrefix(r.URL.Path, "/admin/assets/") {
		if s.serveAdmin(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, adminHTML)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/admin/api/") {
		s.handleAdminAPI(w, r)
		return
	}

	switch r.URL.Path {
	case "/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	case "/stats":
		s.writeStats(w)
		return
	case "/flush-exhausted":
		s.keyState.Flush()
		s.refreshKeyStateStats()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	user, ok := s.authenticate(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	model := parseModel(r.URL.Path, r.URL.Query().Get("model"))
	if model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing model in path"})
		return
	}

	var body []byte
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	s.proxy(w, r, body, user, model)
}

func (s *Server) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/admin/api/setup-state":
		required, err := s.store.SetupRequired()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"setup_required": required})
		return
	case "/admin/api/setup":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req LoginRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		required, err := s.store.SetupRequired()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !required {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "setup already completed"})
			return
		}
		user, err := s.store.CreateInitialAdmin(req.Username, req.Password)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntimeNoResponse()
		token, err := s.createSession(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
		return
	case "/admin/api/login":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req LoginRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, err := s.store.VerifyPassword(req.Username, req.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		token, err := s.createSession(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
		return
	}

	user, ok := s.authenticateAdmin(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	switch r.URL.Path {
	case "/admin/api/session":
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	case "/admin/api/logout":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		token := adminTokenFromRequest(r)
		_ = s.store.DeleteSession(token)
		s.runtimeMu.Lock()
		delete(s.sessions, token)
		s.runtimeMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/admin/api/password":
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := s.store.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/admin/api/stats":
		s.writeStats(w)
	case "/admin/api/usage":
		s.writeUsage(w, r, user)
	case "/admin/api/model-usage":
		s.writeModelUsage(w, r)
	case "/admin/api/config":
		writeJSON(w, http.StatusOK, s.adminConfig())
	case "/admin/api/data":
		s.writeAdminData(w, user)
	case "/admin/api/users":
		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		s.handleUsers(w, r)
	case "/admin/api/user-api-keys":
		s.handleUserAPIKeys(w, r, user)
	case "/admin/api/google-accounts":
		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		s.handleGoogleAccounts(w, r)
	case "/admin/api/api-keys":
		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		s.handleAPIKeys(w, r)
	case "/admin/api/flush-exhausted":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		s.keyState.Flush()
		s.refreshKeyStateStats()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) authenticate(r *http.Request) (AuthToken, bool) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	if len(s.authTokens) == 0 {
		return AuthToken{Name: "local", UserName: "local", Valid: true}, true
	}

	token := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	} else if key := r.URL.Query().Get("key"); key != "" {
		token = key
	} else if key := r.Header.Get("x-goog-api-key"); key != "" {
		token = strings.TrimSpace(key)
	}

	user, ok := s.authTokens[token]
	return user, ok && user.Valid
}

func (s *Server) authenticateAdmin(r *http.Request) (AppUser, bool) {
	token := adminTokenFromRequest(r)
	if token == "" {
		return AppUser{}, false
	}
	s.runtimeMu.Lock()
	user, ok := s.sessions[token]
	s.runtimeMu.Unlock()
	if ok && user.User.Valid && time.Now().Before(user.ExpiresAt) {
		user = s.renewAdminSessionIfNeeded(token, user)
		return user.User, true
	}
	sessionUser, err := s.store.UserBySession(token)
	if err != nil {
		return AppUser{}, false
	}
	sessionUser = s.renewAdminSessionIfNeeded(token, sessionUser)
	s.runtimeMu.Lock()
	s.sessions[token] = sessionUser
	s.runtimeMu.Unlock()
	return sessionUser.User, true
}

func adminTokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func (s *Server) createSession(user AppUser) (string, error) {
	token := randomHex(32)
	expiresAt := time.Now().Add(adminSessionTTL)
	if err := s.store.DeleteExpiredSessions(); err != nil {
		return "", err
	}
	if err := s.store.CreateSession(token, user.ID, expiresAt); err != nil {
		return "", err
	}
	s.runtimeMu.Lock()
	s.sessions[token] = SessionUser{User: user, ExpiresAt: expiresAt}
	s.runtimeMu.Unlock()
	return token, nil
}

func (s *Server) renewAdminSessionIfNeeded(token string, sessionUser SessionUser) SessionUser {
	if time.Until(sessionUser.ExpiresAt) > adminSessionRenewBefore {
		return sessionUser
	}
	nextExpiresAt := time.Now().Add(adminSessionTTL)
	if err := s.store.RenewSession(token, nextExpiresAt); err != nil {
		log.Printf("renew admin session: %v", err)
		return sessionUser
	}
	sessionUser.ExpiresAt = nextExpiresAt
	s.runtimeMu.Lock()
	s.sessions[token] = sessionUser
	s.runtimeMu.Unlock()
	return sessionUser
}

func (s *Server) proxy(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, originalModel string) {
	models := []string{originalModel}
	if originalModel == "auto" || originalModel == "gemini-auto" {
		models = s.cfg.Models.Auto
	}

	var lastErrBody []byte
	var lastStatus int
	var chain []string
	attempts := 0
	requestID := randomHex(8)

	for _, model := range models {
		for attempts < s.cfg.Retry.MaxAttempts {
			key, ok := s.keyState.Next(model)
			if !ok {
				chain = append(chain, model+":all-keys-cooling")
				break
			}

			attempts++
			attemptStartedAt := time.Now()
			resp, err := s.forward(r, body, originalModel, model, key)
			if err != nil {
				s.recordModelUsage(user, key, model, 0, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, "", err.Error(), "")
				chain = append(chain, fmt.Sprintf("%s(%s):%s", model, key.Name, err.Error()))
				s.keyState.MarkError(model, key, 0, err.Error())
				s.refreshKeyStateStats()
				continue
			}

			var errorCode, errorMessage string
			disableKey := false
			if resp.StatusCode >= 400 {
				lastStatus = resp.StatusCode
				lastErrBody = drain(resp.Body, 64*1024)
				errorCode, errorMessage = parseUpstreamError(lastErrBody)
				disableKey = shouldDisableAPIKey(resp.StatusCode, errorCode, errorMessage, lastErrBody)
				s.recordModelUsage(user, key, model, resp.StatusCode, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, errorCode, errorMessage, string(lastErrBody))
				if resp.StatusCode == http.StatusTooManyRequests {
					s.keyState.MarkExhausted(model, key)
				} else if disableKey {
					s.keyState.MarkError(model, key, resp.StatusCode, string(lastErrBody))
					s.disableAPIKey(key, resp.StatusCode, errorCode, errorMessage, lastErrBody)
				} else {
					s.keyState.MarkError(model, key, resp.StatusCode, string(lastErrBody))
				}
				s.refreshKeyStateStats()
			} else {
				s.recordModelUsage(user, key, model, resp.StatusCode, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, "", "", "")
			}

			if shouldRetry(resp.StatusCode) {
				resp.Body.Close()
				chain = append(chain, fmt.Sprintf("%s(%s):%d", model, key.Name, resp.StatusCode))
				continue
			}
			if disableKey {
				resp.Body.Close()
				chain = append(chain, fmt.Sprintf("%s(%s):disabled:%d", model, key.Name, resp.StatusCode))
				continue
			}
			if resp.StatusCode >= 400 {
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(lastErrBody))
			} else {
				s.keyState.ClearError(key)
				s.refreshKeyStateStats()
			}

			defer resp.Body.Close()
			copyResponseHeaders(w.Header(), resp.Header)
			setCORS(w.Header())
			w.Header().Set("X-Proxy-Debug", strings.Join(chain, " -> "))
			w.Header().Set("X-Proxy-Key", key.Name)
			w.WriteHeader(resp.StatusCode)
			startedAt := time.Now()
			io.Copy(w, resp.Body)
			s.recordUsage(user, key, model, resp.StatusCode, time.Since(startedAt))
			return
		}
	}

	if lastStatus == 0 {
		lastStatus = http.StatusTooManyRequests
	}
	reason := "all keys or models failed"
	if attempts >= s.cfg.Retry.MaxAttempts {
		reason = "retry max_attempts reached"
		chain = append(chain, fmt.Sprintf("max-attempts:%d", s.cfg.Retry.MaxAttempts))
	}
	w.Header().Set("X-Proxy-Debug", strings.Join(chain, " -> "))
	writeJSON(w, lastStatus, map[string]any{
		"error":        reason,
		"attempts":     attempts,
		"max_attempts": s.cfg.Retry.MaxAttempts,
		"chain":        chain,
		"body":         string(lastErrBody),
	})
}

func (s *Server) forward(r *http.Request, body []byte, originalModel, currentModel string, key APIKey) (*http.Response, error) {
	upstream := *s.upstream
	upstream.Path = r.URL.Path
	upstream.RawQuery = r.URL.RawQuery
	if currentModel != originalModel {
		upstream.Path = strings.Replace(upstream.Path, "/models/"+originalModel, "/models/"+currentModel, 1)
	}
	q := upstream.Query()
	q.Set("key", key.Key)
	q.Del("model")
	upstream.RawQuery = q.Encode()

	ctx, cancel := context.WithCancel(r.Context())
	req, err := http.NewRequestWithContext(ctx, r.Method, upstream.String(), bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header = cleanHeaders(r.Header)
	resp, err := s.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func (k *KeyState) Next(model string) (APIKey, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	for i := 0; i < len(k.keys); i++ {
		idx := (k.next + i) % len(k.keys)
		key := k.keys[idx]
		expires, cooling := k.exhausted[cooldownKey(model, key.Name)]
		if cooling && now.Before(expires) {
			continue
		}
		if cooling {
			delete(k.exhausted, cooldownKey(model, key.Name))
		}
		k.next = (idx + 1) % len(k.keys)
		return key, true
	}
	return APIKey{}, false
}

func (k *KeyState) MarkExhausted(model string, key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.exhausted[cooldownKey(model, key.Name)] = time.Now().Add(k.cooldown + time.Duration(mathrand.Intn(500))*time.Millisecond)
	delete(k.errors, cooldownKey(model, key.Name))
}

func (k *KeyState) MarkError(model string, key APIKey, status int, message string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(message) > 500 {
		message = message[:500]
	}
	k.errors[cooldownKey(model, key.Name)] = KeyError{
		Key:       key.Name,
		Model:     model,
		Status:    status,
		Message:   strings.TrimSpace(message),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func (k *KeyState) Remove(key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	nextKeys := k.keys[:0]
	for _, item := range k.keys {
		if item.ID != key.ID {
			nextKeys = append(nextKeys, item)
		}
	}
	k.keys = nextKeys
	if len(k.keys) == 0 {
		k.next = 0
	} else if k.next >= len(k.keys) {
		k.next = k.next % len(k.keys)
	}
	suffix := "::" + key.Name
	for item := range k.exhausted {
		if strings.HasSuffix(item, suffix) {
			delete(k.exhausted, item)
		}
	}
}

func (k *KeyState) ClearError(key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for id, item := range k.errors {
		if item.Key == key.Name {
			delete(k.errors, id)
		}
	}
}

func (k *KeyState) Flush() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.exhausted = make(map[string]time.Time)
}

func (k *KeyState) Replace(keys []APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys = keys
	k.next = 0
	k.exhausted = make(map[string]time.Time)
	k.errors = make(map[string]KeyError)
}

func (k *KeyState) SnapshotExhausted() map[string]string {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	out := make(map[string]string)
	for key, expires := range k.exhausted {
		if now.After(expires) {
			delete(k.exhausted, key)
			continue
		}
		out[key] = expires.Format(time.RFC3339)
	}
	return out
}

func (k *KeyState) SnapshotErrors() map[string]KeyError {
	k.mu.Lock()
	defer k.mu.Unlock()

	out := make(map[string]KeyError, len(k.errors))
	for key, item := range k.errors {
		out[key] = item
	}
	return out
}

func (s *Server) disableAPIKey(key APIKey, status int, errorCode, errorMessage string, errorBody []byte) {
	if err := s.store.DisableAPIKey(key.ID, status, errorCode, errorMessage, string(errorBody)); err != nil {
		log.Printf("disable api key %s after %d %s: %v", key.Name, status, errorCode, err)
		return
	}
	s.keyState.Remove(key)
	log.Printf("disabled api key %s after %d %s", key.Name, status, errorCode)
}

func (s *Server) recordUsage(user AuthToken, key APIKey, model string, status int, latency time.Duration) {
	s.statsMu.Lock()
	s.stats.Requests++
	s.stats.ByUser[user.Name]++
	s.stats.ByKey[key.Name]++
	s.stats.LastUpdated = time.Now()
	s.statsMu.Unlock()

	if s.store != nil && s.usageCh != nil {
		record := UsageRecord{
			UserID:             user.UserID,
			UserName:           user.UserName,
			UserAPIKeyID:       user.ID,
			UserAPIKeyName:     user.Name,
			APIKeyID:           key.ID,
			APIKeyName:         key.Name,
			GoogleAccountID:    key.AccountID,
			GoogleAccountEmail: key.AccountEmail,
			Model:              model,
			Status:             status,
			LatencyMS:          latency.Milliseconds(),
		}
		select {
		case s.usageCh <- record:
		default:
			if err := s.store.InsertUsage(record); err != nil {
				log.Printf("record usage: %v", err)
			}
		}
	}
}

func (s *Server) recordModelUsage(user AuthToken, key APIKey, model string, status int, latency time.Duration, requestID string, attempt int, createdAt time.Time, errorCode, errorMessage, errorBody string) {
	if s.store == nil || s.modelUsageCh == nil {
		return
	}
	record := UsageRecord{
		RequestID:          requestID,
		Attempt:            attempt,
		UserID:             user.UserID,
		UserName:           user.UserName,
		UserAPIKeyID:       user.ID,
		UserAPIKeyName:     user.Name,
		APIKeyID:           key.ID,
		APIKeyName:         key.Name,
		GoogleAccountID:    key.AccountID,
		GoogleAccountEmail: key.AccountEmail,
		Model:              model,
		Status:             status,
		LatencyMS:          latency.Milliseconds(),
		CreatedAt:          createdAt,
		ErrorCode:          errorCode,
		ErrorMessage:       errorMessage,
		ErrorBody:          errorBody,
	}
	select {
	case s.modelUsageCh <- record:
	default:
		if err := s.store.InsertModelUsage(record); err != nil {
			log.Printf("record model usage: %v", err)
		}
	}
}

func (s *Server) usageWriter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]UsageRecord, 0, 100)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.store.InsertUsageBatch(batch); err != nil {
			log.Printf("record usage batch: %v", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case record := <-s.usageCh:
			batch = append(batch, record)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) modelUsageWriter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]UsageRecord, 0, 100)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.store.InsertModelUsageBatch(batch); err != nil {
			log.Printf("record model usage batch: %v", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case record := <-s.modelUsageCh:
			batch = append(batch, record)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) writeStats(w http.ResponseWriter) {
	s.refreshKeyStateStats()
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	writeJSON(w, http.StatusOK, s.stats)
}

func (s *Server) writeUsage(w http.ResponseWriter, r *http.Request, actor AppUser) {
	from, to, ok := parseUsageRange(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range"})
		return
	}
	keyID, _ := strconv.ParseInt(r.URL.Query().Get("key_id"), 10, 64)
	summary, err := s.store.UsageSummary(UsageQuery{
		UserID:       actor.ID,
		UserAPIKeyID: keyID,
		From:         from,
		To:           to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) writeModelUsage(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseUsageRange(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range"})
		return
	}
	keyID, _ := strconv.ParseInt(r.URL.Query().Get("key_id"), 10, 64)
	summary, err := s.store.ModelUsageSummary(ModelUsageQuery{
		APIKeyID: keyID,
		From:     from,
		To:       to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func parseUsageRange(r *http.Request) (time.Time, time.Time, bool) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 0, 1)
	if value := r.URL.Query().Get("from"); value != "" {
		parsed, err := parseUsageTime(value, now.Location(), false)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		from = parsed
	}
	if value := r.URL.Query().Get("to"); value != "" {
		parsed, err := parseUsageTime(value, now.Location(), true)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		to = parsed
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func parseUsageTime(value string, loc *time.Location, endOfDay bool) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, nil
		}
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return parsed.AddDate(0, 0, 1), nil
	}
	return parsed, nil
}

func (s *Server) refreshKeyStateStats() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.Exhausted = s.keyState.SnapshotExhausted()
	s.stats.KeyErrors = s.keyState.SnapshotErrors()
}

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.adminDir == "" {
		return false
	}
	if r.URL.Path == "/admin" || r.URL.Path == "/admin/" {
		indexPath := s.adminDir + "/index.html"
		if _, err := os.Stat(indexPath); err != nil {
			return false
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, indexPath)
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/admin/assets/") {
		filePath := strings.TrimPrefix(r.URL.Path, "/admin/")
		if _, err := os.Stat(s.adminDir + "/" + filePath); err != nil {
			return false
		}
		http.StripPrefix("/admin/", http.FileServer(http.Dir(s.adminDir))).ServeHTTP(w, r)
		return true
	}
	return false
}

func (s *Server) adminConfig() AdminConfig {
	keys, _ := s.store.AllAPIKeys()
	accounts, _ := s.store.GoogleAccounts()
	users, _ := s.store.Users()

	adminKeys := make([]AdminKey, 0, len(keys))
	for _, key := range keys {
		adminKeys = append(adminKeys, AdminKey{
			Name:          key.Name,
			Key:           maskSecret(key.Key),
			AccountEmail:  key.AccountEmail,
			AccountPrefix: key.AccountPrefix,
		})
	}
	adminAccounts := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		adminAccounts = append(adminAccounts, Account{Email: account.Email, Prefix: account.Prefix})
	}
	tokens := make([]string, 0, len(users))
	for _, user := range users {
		if user.Valid {
			tokens = append(tokens, user.Username)
		}
	}
	return AdminConfig{
		Addr:            s.cfg.Server.Addr,
		UpstreamBaseURL: s.cfg.Upstream.BaseURL,
		Timeout:         s.cfg.Upstream.Timeout,
		MaxAttempts:     s.cfg.Retry.MaxAttempts,
		CooldownSeconds: s.cfg.Retry.CooldownSeconds,
		Models:          append([]string(nil), s.cfg.Models.Auto...),
		Keys:            adminKeys,
		Accounts:        adminAccounts,
		Tokens:          tokens,
	}
}

func (s *Server) writeAdminData(w http.ResponseWriter, actor AppUser) {
	users, err := s.store.Users()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	userAPIKeys, err := s.store.UserAPIKeys()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	accounts, err := s.store.GoogleAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	keys, err := s.store.AllAPIKeys()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, AdminData{
		Config:      s.adminConfig(),
		Users:       users,
		UserAPIKeys: maskUsers(filterUserAPIKeys(userAPIKeys, actor)),
		Accounts:    accounts,
		Keys:        maskAPIKeys(keys),
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.store.Users()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, users)
	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if _, err := s.store.CreateAppUser(req.Username, req.Password, req.Role); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleUserAPIKeys(w http.ResponseWriter, r *http.Request, actor AppUser) {
	switch r.Method {
	case http.MethodGet:
		if id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64); id != 0 {
			key, err := s.store.UserAPIKey(id, actor.ID)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusOK, key)
			return
		}
		keys, err := s.store.UserAPIKeys()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, maskUsers(filterUserAPIKeys(keys, actor)))
	case http.MethodPost:
		var key AuthToken
		if !decodeJSON(w, r, &key) {
			return
		}
		key.UserID = actor.ID
		key.Valid = true
		if key.Token == "" {
			key.Token = "bh_" + randomHex(24)
		}
		created, err := s.store.CreateUserAPIKey(key)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.reloadRuntimeState(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, created)
	case http.MethodPut:
		var key AuthToken
		if !decodeJSON(w, r, &key) {
			return
		}
		if err := s.store.SetUserAPIKeyValid(key.ID, actor.ID, key.Valid); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := s.store.DeleteUserAPIKey(id, actor.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleGoogleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		accounts, err := s.store.GoogleAccounts()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, accounts)
	case http.MethodPost:
		var account GoogleAccount
		if !decodeJSON(w, r, &account) {
			return
		}
		if err := s.store.CreateGoogleAccount(account); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	case http.MethodPut:
		var account GoogleAccount
		if !decodeJSON(w, r, &account) {
			return
		}
		if err := s.store.UpdateGoogleAccount(account); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := s.store.DeleteGoogleAccount(id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, err := s.store.AllAPIKeys()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, maskAPIKeys(keys))
	case http.MethodPost:
		var key APIKey
		if !decodeJSON(w, r, &key) {
			return
		}
		if err := s.store.CreateAPIKey(key); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	case http.MethodPut:
		var key APIKey
		if !decodeJSON(w, r, &key) {
			return
		}
		if strings.Contains(key.Key, "...") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "full api key is required when updating"})
			return
		}
		if err := s.store.UpdateAPIKey(key); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	case http.MethodDelete:
		ids := parseIDs(r.URL.Query().Get("ids"))
		if len(ids) == 0 {
			if id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64); id != 0 {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}
		if err := s.store.DeleteAPIKeys(ids); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.reloadRuntime(w)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) reloadRuntime(w http.ResponseWriter) {
	if err := s.reloadRuntimeState(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) reloadRuntimeState() error {
	tokens, keys, err := s.store.ReloadRuntime()
	if err != nil {
		return err
	}
	s.runtimeMu.Lock()
	s.authTokens = tokens
	s.runtimeMu.Unlock()
	s.keyState.Replace(keys)
	s.refreshKeyStateStats()
	return nil
}

func (s *Server) reloadRuntimeNoResponse() {
	tokens, keys, err := s.store.ReloadRuntime()
	if err != nil {
		log.Printf("reload runtime: %v", err)
		return
	}
	s.runtimeMu.Lock()
	s.authTokens = tokens
	s.runtimeMu.Unlock()
	s.keyState.Replace(keys)
}

func maskAPIKeys(keys []APIKey) []APIKey {
	out := make([]APIKey, 0, len(keys))
	for _, key := range keys {
		key.Key = maskSecret(key.Key)
		out = append(out, key)
	}
	return out
}

func maskUsers(users []AuthToken) []AuthToken {
	out := make([]AuthToken, 0, len(users))
	for _, user := range users {
		out = append(out, maskUser(user))
	}
	return out
}

func filterUserAPIKeys(keys []AuthToken, actor AppUser) []AuthToken {
	if actor.Role == "admin" && actor.ID == 0 {
		return keys
	}
	out := make([]AuthToken, 0, len(keys))
	for _, key := range keys {
		if key.UserID == actor.ID {
			out = append(out, key)
		}
	}
	return out
}

func maskUser(user AuthToken) AuthToken {
	user.Token = maskSecret(user.Token)
	return user
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

func maskSecret(value string) string {
	if len(value) <= 10 {
		return "****"
	}
	return value[:6] + strings.Repeat(".", len(value)-10) + value[len(value)-4:]
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	const alphabet = "0123456789abcdef"
	out := make([]byte, len(buf)*2)
	for i, b := range buf {
		out[i*2] = alphabet[b>>4]
		out[i*2+1] = alphabet[b&0x0f]
	}
	return string(out)
}

func parseModel(path string, fallback string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			model := parts[i+1]
			if idx := strings.IndexByte(model, ':'); idx >= 0 {
				return model[:idx]
			}
			return model
		}
	}
	return fallback
}

func parseIDs(value string) []int64 {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func cleanHeaders(in http.Header) http.Header {
	out := in.Clone()
	for _, h := range []string{
		"Authorization",
		"Connection",
		"Content-Length",
		"Cf-Connecting-Ip",
		"Cf-Ipcountry",
		"Cf-Ray",
		"Cf-Visitor",
		"Host",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		"X-Forwarded-For",
		"X-Goog-Api-Key",
		"X-Real-Ip",
	} {
		out.Del(h)
	}
	return out
}

func copyResponseHeaders(dst, src http.Header) {
	for k, values := range src {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		dst.Del(k)
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

func setCORS(h http.Header) {
	for k, v := range corsHeaders {
		h.Set(k, v)
	}
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func shouldDisableAPIKey(status int, errorCode, errorMessage string, body []byte) bool {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return true
	}
	if status != http.StatusBadRequest {
		return false
	}
	text := strings.ToUpper(errorCode + "\n" + errorMessage + "\n" + string(body))
	return strings.Contains(text, "API_KEY_INVALID") || strings.Contains(text, "API KEY NOT VALID")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	setCORS(w.Header())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func drain(r io.Reader, max int64) []byte {
	if r == nil {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(r, max))
	return data
}

func parseUpstreamError(body []byte) (string, string) {
	var payload struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if len(body) == 0 || json.Unmarshal(body, &payload) != nil {
		return "", ""
	}
	return payload.Error.Status, payload.Error.Message
}

func cooldownKey(model, keyName string) string {
	return model + "::" + keyName
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

const adminHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>BuzzHive Admin</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f8fafc;
      --panel: #ffffff;
      --text: #0f172a;
      --muted: #64748b;
      --line: #d9e2ec;
      --strong-line: #cbd5e1;
      --accent: #0f766e;
      --accent-text: #ffffff;
      --warn: #b45309;
      --danger: #b91c1c;
      --shadow: 0 1px 2px rgba(15, 23, 42, .08);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 14px/1.45 ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    button, input { font: inherit; }
    .shell { min-height: 100vh; display: flex; flex-direction: column; }
    .topbar {
      height: 56px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 0 24px;
      border-bottom: 1px solid var(--line);
      background: rgba(255,255,255,.92);
      position: sticky;
      top: 0;
      z-index: 5;
      backdrop-filter: blur(10px);
    }
    .brand { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .mark {
      width: 28px;
      height: 28px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      display: grid;
      place-items: center;
      background: #ecfdf5;
      color: var(--accent);
      font-weight: 700;
    }
    h1 { font-size: 15px; line-height: 1; margin: 0; font-weight: 650; }
    .subtle { color: var(--muted); font-size: 12px; }
    .toolbar { display: flex; align-items: center; gap: 8px; }
    .wrap { width: min(1180px, 100%); margin: 0 auto; padding: 22px 24px 32px; }
    .grid { display: grid; gap: 14px; }
    .metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .cols { grid-template-columns: 1.1fr .9fr; align-items: start; margin-top: 14px; }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }
    .card-head {
      min-height: 48px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 13px 14px;
      border-bottom: 1px solid var(--line);
    }
    .card-title { font-size: 13px; font-weight: 650; margin: 0; }
    .card-body { padding: 14px; }
    .metric { padding: 14px; }
    .metric-label { color: var(--muted); font-size: 12px; margin-bottom: 8px; }
    .metric-value { font-size: 26px; line-height: 1; font-weight: 700; letter-spacing: 0; }
    .row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      min-height: 38px;
      border-bottom: 1px solid #edf2f7;
    }
    .row:last-child { border-bottom: 0; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; }
    .pill {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      padding: 0 8px;
      border-radius: 999px;
      border: 1px solid var(--line);
      background: #f8fafc;
      color: #334155;
      white-space: nowrap;
      font-size: 12px;
    }
    .pill.ok { border-color: #99f6e4; background: #ecfdf5; color: #0f766e; }
    .pill.warn { border-color: #fed7aa; background: #fff7ed; color: var(--warn); }
    .btn {
      height: 34px;
      padding: 0 11px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      background: #fff;
      color: var(--text);
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 7px;
    }
    .btn:hover { background: #f8fafc; }
    .btn.primary { border-color: var(--accent); background: var(--accent); color: var(--accent-text); }
    .btn.danger { color: var(--danger); }
    .btn:disabled { opacity: .55; cursor: default; }
    .login {
      min-height: calc(100vh - 56px);
      display: grid;
      place-items: center;
      padding: 24px;
    }
    .login-panel {
      width: min(420px, 100%);
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      padding: 18px;
    }
    .field { display: grid; gap: 8px; margin: 16px 0 12px; }
    label { font-size: 12px; color: var(--muted); }
    input {
      width: 100%;
      height: 38px;
      border: 1px solid var(--strong-line);
      border-radius: 7px;
      padding: 0 10px;
      background: #fff;
      color: var(--text);
    }
    input:focus { outline: 2px solid #99f6e4; border-color: var(--accent); }
    .error { color: var(--danger); min-height: 20px; font-size: 12px; margin-top: 8px; }
    .hidden { display: none !important; }
    .list { display: grid; gap: 4px; }
    .table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 9px 8px; border-bottom: 1px solid #edf2f7; vertical-align: middle; }
    th { color: var(--muted); font-weight: 600; font-size: 12px; }
    .right { text-align: right; }
    .empty { color: var(--muted); padding: 10px 0; }
    @media (max-width: 860px) {
      .metrics, .cols { grid-template-columns: 1fr; }
      .topbar { padding: 0 14px; }
      .wrap { padding: 14px; }
      .metric-value { font-size: 22px; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
        <div class="mark">B</div>
        <div>
          <h1>BuzzHive</h1>
          <div class="subtle" id="serverLine">Local Gemini Proxy</div>
        </div>
      </div>
      <div class="toolbar">
        <span class="pill ok hidden" id="userPill"></span>
        <button class="btn hidden" id="refreshBtn" type="button" title="Refresh">↻</button>
        <button class="btn hidden" id="logoutBtn" type="button">Logout</button>
      </div>
    </header>

    <main id="loginView" class="login">
      <section class="login-panel">
        <h2 class="card-title" id="loginTitle">Admin Login</h2>
        <div class="field">
          <label for="usernameInput">Username</label>
          <input id="usernameInput" autocomplete="username" autofocus>
        </div>
        <div class="field">
          <label for="passwordInput">Password</label>
          <input id="passwordInput" type="password" autocomplete="current-password">
        </div>
        <button class="btn primary" id="loginBtn" type="button">Login</button>
        <div class="error" id="loginError"></div>
      </section>
    </main>

    <main id="appView" class="wrap hidden">
      <section class="grid metrics">
        <div class="card metric">
          <div class="metric-label">Requests</div>
          <div class="metric-value" id="requestsMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">API Keys</div>
          <div class="metric-value" id="keysMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">Cooling</div>
          <div class="metric-value" id="coolingMetric">0</div>
        </div>
        <div class="card metric">
          <div class="metric-label">Models</div>
          <div class="metric-value" id="modelsMetric">0</div>
        </div>
      </section>

      <section class="grid cols">
        <div class="grid">
          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Auto Models</h2>
              <span class="pill" id="retryPill"></span>
            </div>
            <div class="card-body list" id="modelsList"></div>
          </section>

          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Key Usage</h2>
              <button class="btn danger" id="flushBtn" type="button">Clear Cooling</button>
            </div>
            <div class="card-body">
              <table class="table">
                <thead><tr><th>Name</th><th>Key</th><th class="right">Requests</th></tr></thead>
                <tbody id="keysTable"></tbody>
              </table>
            </div>
          </section>
        </div>

        <div class="grid">
          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Runtime</h2>
              <span class="pill ok" id="healthPill">online</span>
            </div>
            <div class="card-body list" id="runtimeList"></div>
          </section>

          <section class="card">
            <div class="card-head">
              <h2 class="card-title">Cooling Keys</h2>
              <span class="pill warn" id="coolingPill">0</span>
            </div>
            <div class="card-body list" id="coolingList"></div>
          </section>
        </div>
      </section>
    </main>
  </div>

  <script>
    const state = { token: localStorage.getItem('buzzhive-admin-key') || '', config: null, stats: null, setupRequired: false };
    const $ = (id) => document.getElementById(id);

    function headers() {
      return { 'Authorization': 'Bearer ' + state.token, 'Content-Type': 'application/json' };
    }
    async function api(path, options) {
      const res = await fetch(path, Object.assign({ headers: headers() }, options || {}));
      if (res.status === 401) throw new Error('unauthorized');
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    }
    function showApp(show) {
      $('loginView').classList.toggle('hidden', show);
      $('appView').classList.toggle('hidden', !show);
      $('logoutBtn').classList.toggle('hidden', !show);
      $('refreshBtn').classList.toggle('hidden', !show);
      $('userPill').classList.toggle('hidden', !show);
    }
    function fmtDate(value) {
      if (!value || value.startsWith('0001-')) return '-';
      return new Date(value).toLocaleString();
    }
    function row(label, value) {
      return '<div class="row"><span class="subtle">' + label + '</span><span class="mono">' + value + '</span></div>';
    }
    async function login() {
      $('loginError').textContent = '';
      try {
        const path = state.setupRequired ? '/admin/api/setup' : '/admin/api/login';
        const result = await api(path, {
          method: 'POST',
          body: JSON.stringify({
            username: $('usernameInput').value.trim(),
            password: $('passwordInput').value
          })
        });
        state.token = result.token;
        localStorage.setItem('buzzhive-admin-key', result.token);
        $('userPill').textContent = result.user.username || 'admin';
        showApp(true);
        await load();
      } catch (err) {
        $('loginError').textContent = 'Login failed';
        localStorage.removeItem('buzzhive-admin-key');
      }
    }
    async function load() {
      const [config, stats] = await Promise.all([api('/admin/api/config'), api('/admin/api/stats')]);
      state.config = config;
      state.stats = stats;
      render();
    }
    function render() {
      const config = state.config;
      const stats = state.stats;
      const exhausted = stats.exhausted || {};
      $('serverLine').textContent = config.addr + ' -> ' + config.upstream_base_url;
      $('requestsMetric').textContent = stats.requests || 0;
      $('keysMetric').textContent = config.keys.length;
      $('coolingMetric').textContent = Object.keys(exhausted).length;
      $('modelsMetric').textContent = config.models.length;
      $('retryPill').textContent = config.max_attempts + ' attempts / ' + config.cooldown_seconds + 's';
      $('coolingPill').textContent = Object.keys(exhausted).length;

      $('modelsList').innerHTML = config.models.map((model, index) =>
        '<div class="row"><span><span class="pill">' + (index + 1) + '</span> <span class="mono">' + model + '</span></span><span class="subtle">fallback</span></div>'
      ).join('');

      const usage = stats.by_key || {};
      $('keysTable').innerHTML = config.keys.map((key) =>
        '<tr><td class="mono">' + key.name + '</td><td class="mono">' + key.key + '</td><td class="right mono">' + (usage[key.name] || 0) + '</td></tr>'
      ).join('');

      $('runtimeList').innerHTML =
        row('Started', fmtDate(stats.started_at)) +
        row('Last request', fmtDate(stats.last_updated)) +
        row('Timeout', config.timeout) +
        row('Tokens', config.tokens.join(', ') || '-');

      const coolingKeys = Object.keys(exhausted);
      $('coolingList').innerHTML = coolingKeys.length
        ? coolingKeys.map((key) => row(key, fmtDate(exhausted[key]))).join('')
        : '<div class="empty">No cooling keys</div>';
    }
    $('loginBtn').addEventListener('click', login);
    $('passwordInput').addEventListener('keydown', (event) => { if (event.key === 'Enter') login(); });
    $('refreshBtn').addEventListener('click', load);
    $('logoutBtn').addEventListener('click', () => {
      state.token = '';
      localStorage.removeItem('buzzhive-admin-key');
      showApp(false);
      $('usernameInput').focus();
    });
    $('flushBtn').addEventListener('click', async () => {
      await api('/admin/api/flush-exhausted', { method: 'POST' });
      await load();
    });

    api('/admin/api/setup-state')
      .then((setup) => {
        state.setupRequired = setup.setup_required;
        $('loginTitle').textContent = setup.setup_required ? 'Create Initial Admin' : 'Admin Login';
        $('loginBtn').textContent = setup.setup_required ? 'Create admin' : 'Login';
        if (!state.token) return;
        return api('/admin/api/session').then((session) => {
          $('userPill').textContent = session.user.username || 'admin';
          showApp(true);
          return load();
        });
      })
      .catch(() => showApp(false));
  </script>
</body>
</html>`

func init() {
	if seed, err := strconv.ParseInt(os.Getenv("LOCAL_PROXY_RAND_SEED"), 10, 64); err == nil {
		mathrand.Seed(seed)
		return
	}
	mathrand.Seed(time.Now().UnixNano())
}
