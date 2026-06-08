package buzzhive

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
