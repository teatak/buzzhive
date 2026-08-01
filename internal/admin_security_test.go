package buzzhive

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAdminDataRedactsSensitiveDataForNonAdmin(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	userToken := createAdminRouteTestSession(t, srv, "alice", "user")

	if _, err := srv.store.CreateUserAPIKey(AuthToken{UserID: 1, Name: "alice-key", Token: "bh_alice_secret", Valid: true}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/data", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	srv.adminAPI.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var data AdminData
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Users) != 0 || len(data.Config.Tokens) != 0 {
		t.Fatalf("non-admin data leaked sensitive fields: %+v", data)
	}
	if len(data.UserAPIKeys) != 1 || data.UserAPIKeys[0].Token == "bh_alice_secret" {
		t.Fatalf("user api keys were not limited and masked: %+v", data.UserAPIKeys)
	}
}

func TestAdminOnlyRoutesRejectNonAdmin(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	userToken := createAdminRouteTestSession(t, srv, "bob", "user")

	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/api/config"},
		{http.MethodPost, "/admin/api/flush-exhausted"},
		{http.MethodGet, "/admin/api/users/1"},
		{http.MethodGet, "/admin/api/users/1/api-keys"},
		{http.MethodGet, "/admin/api/users/1/usage"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			srv.adminAPI.ServeHTTP(rr, req)
			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403, body = %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminCanManageAndInspectAnotherUsersAPIKeys(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	adminToken := createAdminRouteTestSession(t, srv, "admin", "admin")
	target, err := srv.store.CreateAppUser("target", "password", "user")
	if err != nil {
		t.Fatal(err)
	}

	createBody := bytes.NewBufferString(`{"name":"managed-key"}`)
	createdRR := httptest.NewRecorder()
	createdReq := httptest.NewRequest(http.MethodPost, "/admin/api/users/"+strconv.FormatInt(target.ID, 10)+"/api-keys", createBody)
	createdReq.Header.Set("Authorization", "Bearer "+adminToken)
	createdReq.Header.Set("Content-Type", "application/json")
	srv.adminAPI.ServeHTTP(createdRR, createdReq)
	if createdRR.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createdRR.Code, createdRR.Body.String())
	}
	var created AuthToken
	if err := json.Unmarshal(createdRR.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.UserID != target.ID || !strings.HasPrefix(created.Token, "bh_") {
		t.Fatalf("created key = %+v", created)
	}

	if err := srv.store.InsertUsageBatch([]UsageRecord{{
		UserID:         target.ID,
		UserName:       target.Username,
		UserAPIKeyID:   created.ID,
		UserAPIKeyName: created.Name,
		Model:          "test-model",
		Status:         http.StatusOK,
		TotalTokens:    42,
		CreatedAt:      time.Now().UTC(),
	}}); err != nil {
		t.Fatal(err)
	}

	keysRR := httptest.NewRecorder()
	keysReq := httptest.NewRequest(http.MethodGet, "/admin/api/users/"+strconv.FormatInt(target.ID, 10)+"/api-keys", nil)
	keysReq.Header.Set("Authorization", "Bearer "+adminToken)
	srv.adminAPI.ServeHTTP(keysRR, keysReq)
	if keysRR.Code != http.StatusOK {
		t.Fatalf("keys status = %d, body = %s", keysRR.Code, keysRR.Body.String())
	}
	var keys []UserAPIKeyDetails
	if err := json.Unmarshal(keysRR.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].UserID != target.ID || keys[0].Token == created.Token || keys[0].Requests != 1 || keys[0].TotalTokens != 42 {
		t.Fatalf("keys = %+v", keys)
	}

	other, err := srv.store.CreateAppUser("other", "password", "user")
	if err != nil {
		t.Fatal(err)
	}
	wrongOwnerRR := httptest.NewRecorder()
	wrongOwnerReq := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/users/"+strconv.FormatInt(other.ID, 10)+"/api-keys/"+strconv.FormatInt(created.ID, 10),
		bytes.NewBufferString(`{"valid":false}`),
	)
	wrongOwnerReq.Header.Set("Authorization", "Bearer "+adminToken)
	wrongOwnerReq.Header.Set("Content-Type", "application/json")
	srv.adminAPI.ServeHTTP(wrongOwnerRR, wrongOwnerReq)
	if wrongOwnerRR.Code != http.StatusNotFound {
		t.Fatalf("wrong owner status = %d, want 404, body = %s", wrongOwnerRR.Code, wrongOwnerRR.Body.String())
	}

	query := url.Values{}
	query.Set("from", time.Now().Add(-time.Hour).Format(time.RFC3339))
	query.Set("to", time.Now().Add(time.Hour).Format(time.RFC3339))
	usageRR := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/admin/api/users/"+strconv.FormatInt(target.ID, 10)+"/usage?"+query.Encode(), nil)
	usageReq.Header.Set("Authorization", "Bearer "+adminToken)
	srv.adminAPI.ServeHTTP(usageRR, usageReq)
	if usageRR.Code != http.StatusOK {
		t.Fatalf("usage status = %d, body = %s", usageRR.Code, usageRR.Body.String())
	}
	var usage UsageSummary
	if err := json.Unmarshal(usageRR.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Requests != 1 || usage.TotalTokens != 42 {
		t.Fatalf("usage = %+v", usage)
	}

	selfRR := httptest.NewRecorder()
	selfReq := httptest.NewRequest(http.MethodGet, "/admin/api/user-api-keys", nil)
	selfReq.Header.Set("Authorization", "Bearer "+adminToken)
	srv.adminAPI.ServeHTTP(selfRR, selfReq)
	if selfRR.Code != http.StatusOK {
		t.Fatalf("self keys status = %d, body = %s", selfRR.Code, selfRR.Body.String())
	}
	var selfKeys []AuthToken
	if err := json.Unmarshal(selfRR.Body.Bytes(), &selfKeys); err != nil {
		t.Fatal(err)
	}
	if len(selfKeys) != 0 {
		t.Fatalf("admin self keys leaked target keys: %+v", selfKeys)
	}
}

func TestPublicStatsAndFlushAreNotExposed(t *testing.T) {
	srv := newAdminRouteTestServer(t)
	srv.keyState.MarkExhausted("gemini", APIKey{Name: "k1"})

	for _, path := range []string{"/stats", "/flush-exhausted"} {
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rr.Code)
		}
	}
	if len(srv.keyState.SnapshotExhausted()) == 0 {
		t.Fatal("public flush-exhausted cleared key cooldown state")
	}
}

func TestPublicAdminRouteWrongMethodDoesNotBypassAuth(t *testing.T) {
	srv := newAdminRouteTestServer(t)

	rr := httptest.NewRecorder()
	srv.adminAPI.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/api/login", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func newAdminRouteTestServer(t *testing.T) *Server {
	t.Helper()
	store := openTestStore(t)
	srv := &Server{
		store:         store,
		adminSessions: make(map[string]SessionUser),
		keyState: &KeyState{
			cooldown:  time.Minute,
			exhausted: make(map[string]time.Time),
			errors:    make(map[string]KeyError),
		},
		stats: Stats{
			StartedAt: time.Now(),
			Exhausted: make(map[string]string),
			KeyErrors: make(map[string]KeyError),
		},
	}
	srv.cfg.Upstream.Timeout = "10m"
	srv.cfg.Retry.CooldownSeconds = 60
	srv.adminAPI = srv.newAdminAPI()
	return srv
}

func createAdminRouteTestSession(t *testing.T, srv *Server, username, role string) string {
	t.Helper()
	user, err := srv.store.CreateAppUser(username, "password", role)
	if err != nil {
		t.Fatal(err)
	}
	token, err := srv.createSession(user)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
