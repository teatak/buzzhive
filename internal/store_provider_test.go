package buzzhive

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProviderAPIKeysUseProviderTables(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	provider, err := store.CreateProvider(ProviderRecord{
		Name:    providerGemini,
		Type:    providerGemini,
		BaseURL: "https://gemini.example.test",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	createdKey, err := store.CreateProviderKey(ProviderKey{
		ProviderID: provider.ID,
		Name:       "k01-alpha",
		Secret:     "AIza-secret-alpha",
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	keys, err := store.ProviderAPIKeys(providerGemini)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("provider keys = %d, want 1", len(keys))
	}
	key := keys[0]
	if key.ProviderName != providerGemini || key.ProviderKeyID != createdKey.ID {
		t.Fatalf("provider metadata = %+v", key)
	}
	if key.Name != "k01-alpha" || key.Key != "AIza-secret-alpha" {
		t.Fatalf("provider key = %+v", key)
	}

	if err := store.DisableProviderKey(key.ID, 403, "PERMISSION_DENIED", "disabled", "body"); err != nil {
		t.Fatal(err)
	}
	keys, err = store.ProviderAPIKeys(providerGemini)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("enabled provider keys = %d, want 0", len(keys))
	}

	var status int
	if err := store.queryRow(`SELECT disabled_status FROM provider_keys WHERE name = ?`, "k01-alpha").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != 403 {
		t.Fatalf("disabled_status = %d, want 403", status)
	}
}

func TestDisableProviderKey(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := "2026-01-01T00:00:00Z"
	providerID, err := store.insertReturningID(
		`INSERT INTO providers (name, type, preset_id, base_url, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		"openrouter", "openai-compatible", "openrouter", "https://openrouter.example.test/v1", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := store.insertReturningID(
		`INSERT INTO provider_keys (provider_id, name, secret, secret_hint, enabled, priority, weight, labels, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 0, 1, '', ?, ?)`,
		providerID, "or-main", "sk-secret", "cret", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DisableProviderKey(keyID, 401, "unauthorized", "bad key", "body"); err != nil {
		t.Fatal(err)
	}
	keys, err := store.RuntimeProviderAPIKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("enabled keys = %d, want 0", len(keys))
	}
	var status int
	if err := store.queryRow(`SELECT disabled_status FROM provider_keys WHERE id = ?`, keyID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != 401 {
		t.Fatalf("disabled_status = %d, want 401", status)
	}
}

func TestProviderManagementCRUD(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	provider, err := store.CreateProvider(ProviderRecord{
		Name:    "openrouter",
		Type:    "openai-compatible",
		BaseURL: "https://openrouter.example.test/v1",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.CreateProviderKey(ProviderKey{
		ProviderID: provider.ID,
		Secret:     "sk-test-secret",
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if key.Secret == "sk-test-secret" || key.SecretHint != "cret" {
		t.Fatalf("key masking/hint = %+v", key)
	}

	model, err := store.CreateModel(Model{
		Name:    "mimo-v2.5",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	route, err := store.CreateModelRoute(ModelRoute{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		UpstreamModel: "mimo/v2.5",
		Enabled:       true,
		Weight:        2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.ProviderName != "openrouter" {
		t.Fatalf("route provider metadata = %+v", route)
	}

	targets, ok, err := store.ResolveModelRoutes("mimo-v2.5")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(targets) != 1 || targets[0].ProviderName != "openrouter" || targets[0].UpstreamModel != "mimo/v2.5" {
		t.Fatalf("resolved targets = %+v, ok=%v", targets, ok)
	}

	if err := store.DeleteProvider(provider.ID); err != nil {
		t.Fatal(err)
	}
	targets, ok, err = store.ResolveModelRoutes("mimo-v2.5")
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(targets) != 0 {
		t.Fatalf("targets after provider delete = %+v, ok=%v", targets, ok)
	}
}

func TestAdminModelUpdateAllowsEmptyDisplayName(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	adminToken := createAdminRouteTestSession(t, srv, "admin", "admin")

	model, err := srv.store.CreateModel(Model{
		Name:            "owl-alpha",
		DisplayName:     "Owl Alpha",
		ContextWindow:   1050000,
		MaxInputTokens:  1050000,
		MaxOutputTokens: 262100,
		Capabilities:    `{"stream":true}`,
		SelectionPolicy: "round_robin",
		Enabled:         true,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"id":` + strconv.FormatInt(model.ID, 10) + `,"name":"owl-alpha","display_name":"","description":"","context_window":1050000,"max_input_tokens":1050000,"max_output_tokens":262100,"capabilities":"{\"stream\":true}","selection_policy":"round_robin","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/models", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	srv.adminAPI.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var updated Model
	if err := json.Unmarshal(rr.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "" {
		t.Fatalf("display_name = %q, want empty", updated.DisplayName)
	}
}

func TestAdminProviderRoutesCreateRuntimeProvider(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	adminToken := createAdminRouteTestSession(t, srv, "admin", "admin")

	body := bytes.NewBufferString(`{"name":"openai","type":"openai","base_url":"https://api.openai.example/v1","enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	srv.adminAPI.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var provider ProviderRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &provider); err != nil {
		t.Fatal(err)
	}
	if provider.ID == 0 || provider.Name != "openai" {
		t.Fatalf("provider = %+v", provider)
	}
	if _, ok := srv.providers["openai"]; !ok {
		t.Fatalf("runtime providers = %+v, want openai", srv.providers)
	}

	updateBody := bytes.NewBufferString(`{"id":` + strconv.FormatInt(provider.ID, 10) + `,"name":"openai","type":"openai","base_url":"https://api.openai.example/v2","enabled":true}`)
	req = httptest.NewRequest(http.MethodPut, "/admin/api/providers", updateBody)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	srv.adminAPI.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var updated ProviderRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != provider.ID || updated.BaseURL != "https://api.openai.example/v2" {
		t.Fatalf("updated provider = %+v, want id %d and v2 base URL", updated, provider.ID)
	}
	providers, err := srv.store.Providers()
	if err != nil {
		t.Fatal(err)
	}
	openaiCount := 0
	for _, item := range providers {
		if item.Name == "openai" {
			openaiCount++
		}
	}
	if openaiCount != 1 {
		t.Fatalf("openai providers = %d, want 1 after update; providers = %+v", openaiCount, providers)
	}
}
