package buzzhive

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

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
