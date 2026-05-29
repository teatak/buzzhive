package buzzhive

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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
