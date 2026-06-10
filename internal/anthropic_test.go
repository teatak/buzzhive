package buzzhive

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAnthropicPassthrough(t *testing.T) {
	var upstreamPath string
	var upstreamMethod string
	var upstreamApiKey string
	var upstreamVersion string
	var upstreamBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamMethod = r.Method
		upstreamApiKey = r.Header.Get("x-api-key")
		upstreamVersion = r.Header.Get("anthropic-version")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		upstreamBody = buf[:n]

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"hello from anthropic passthrough"}]}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	store := openTestStore(t)
	defer store.Close()

	now := storeNow()
	providerID, err := store.insertReturningID(
		`INSERT INTO providers (name, preset_id, base_url, protocols, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		"claude-provider", "anthropic", upstream.URL, "anthropic", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.exec(
		`INSERT INTO provider_keys (provider_id, name, secret, secret_hint, enabled, priority, weight, labels, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 0, 1, '', ?, ?)`,
		providerID, "claude-key-1", "mock-anthropic-secret-key", "key1", now, now,
	); err != nil {
		t.Fatal(err)
	}

	modelID, err := store.insertReturningID(
		`INSERT INTO models (name, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
		"claude-3-5-sonnet", "round_robin", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.exec(
		`INSERT INTO model_routes (model_id, provider_id, upstream_model, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, 1, 0, 1, ?, ?)`,
		modelID, providerID, "claude-3-5-sonnet-upstream", now, now,
	); err != nil {
		t.Fatal(err)
	}

	providerRecords, err := store.EnabledProviders()
	if err != nil {
		t.Fatal(err)
	}
	providers, err := newProviderRegistry(providerRecords, upstreamURL, upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	keys, err := store.RuntimeProviderAPIKeys()
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		store:     store,
		upstream:  upstreamURL,
		client:    upstream.Client(),
		providers: providers,
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "user-key-1", UserName: "user1", Valid: true},
		},
		keyState: &KeyState{
			keys:         keys,
			cooldown:     time.Minute,
			rpdCooldown:  time.Hour,
			exhausted:    make(map[string]time.Time),
			cooldownHits: make(map[string]int),
			rpdLike:      make(map[string]bool),
			errors:       make(map[string]KeyError),
		},
		stats: Stats{
			StartedAt: time.Now(),
			Exhausted: make(map[string]string),
			RPDLike:   make(map[string]bool),
			KeyErrors: make(map[string]KeyError),
		},
	}

	body := `{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1/messages" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamMethod != http.MethodPost {
		t.Fatalf("upstream method = %q", upstreamMethod)
	}
	if upstreamApiKey != "mock-anthropic-secret-key" {
		t.Fatalf("upstream api key = %q", upstreamApiKey)
	}
	if upstreamVersion != "2023-06-01" {
		t.Fatalf("upstream version = %q", upstreamVersion)
	}

	var reqBody map[string]any
	if err := json.Unmarshal(upstreamBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	// Upstream model rewritten correctly
	if reqBody["model"] != "claude-3-5-sonnet-upstream" {
		t.Fatalf("upstream model = %q", reqBody["model"])
	}

	var respBody map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &respBody); err != nil {
		t.Fatal(err)
	}
	if respBody["id"] != "msg_123" {
		t.Fatalf("response id = %q", respBody["id"])
	}
}
