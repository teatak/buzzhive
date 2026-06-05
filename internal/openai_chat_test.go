package buzzhive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func createGeminiRouteTestStore(t *testing.T, baseURL, publicModel, upstreamModel, keySecret string) (*Store, []APIKey) {
	t.Helper()

	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := store.CreateProvider(ProviderRecord{
		Name:    providerGemini,
		Type:    providerGemini,
		BaseURL: baseURL,
		Enabled: true,
	})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.CreateProviderKey(ProviderKey{
		ProviderID: provider.ID,
		Name:       "ga-1",
		Secret:     keySecret,
		Enabled:    true,
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	model, err := store.CreateModel(Model{
		Name:    publicModel,
		Enabled: true,
	})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.CreateModelRoute(ModelRoute{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		UpstreamModel: upstreamModel,
		Enabled:       true,
		Weight:        1,
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	keys, err := store.RuntimeProviderAPIKeys()
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store, keys
}

func TestOpenAIErrorTypeAndCodeMapsUpstreamStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantType string
		wantCode string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantType: "authentication_error", wantCode: "invalid_api_key"},
		{name: "forbidden", status: http.StatusForbidden, wantType: "permission_error", wantCode: "permission_denied"},
		{name: "too many requests", status: http.StatusTooManyRequests, wantType: "rate_limit_error", wantCode: "rate_limit_exceeded"},
		{name: "server error", status: http.StatusBadGateway, wantType: "server_error", wantCode: "upstream_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotCode := openAIErrorTypeAndCode(tt.status, "upstream_error")
			if gotType != tt.wantType || gotCode != tt.wantCode {
				t.Fatalf("type/code = %s/%s, want %s/%s", gotType, gotCode, tt.wantType, tt.wantCode)
			}
		})
	}
}

func TestOpenAIUpstreamErrorMessageParsesProviderBodies(t *testing.T) {
	tests := []struct {
		name   string
		status int
		raw    string
		want   string
	}{
		{
			name:   "gemini error object",
			status: http.StatusTooManyRequests,
			raw:    `{"error":{"code":429,"message":"Resource has been exhausted.","status":"RESOURCE_EXHAUSTED"}}`,
			want:   "Resource has been exhausted. (RESOURCE_EXHAUSTED)",
		},
		{
			name:   "string error",
			status: http.StatusUnauthorized,
			raw:    `{"error":"unauthorized"}`,
			want:   "unauthorized",
		},
		{
			name:   "empty body",
			status: http.StatusBadGateway,
			raw:    ``,
			want:   "Bad Gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := openAIUpstreamErrorMessage(tt.status, []byte(tt.raw)); got != tt.want {
				t.Fatalf("message = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenAIRetryErrorUsesMappedRateLimitError(t *testing.T) {
	resp := errorResponse(http.StatusTooManyRequests, `{"error":{"code":429,"message":"Resource has been exhausted.","status":"RESOURCE_EXHAUSTED"}}`)
	rr := httptest.NewRecorder()

	writeOpenAIRetryError(rr, resp, 2, 2, []string{"m(k1):429", "m(k2):429"})

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Type != "rate_limit_error" || got.Error.Code != "rate_limit_exceeded" {
		t.Fatalf("error = %+v", got.Error)
	}
	if !strings.Contains(got.Error.Message, "Resource has been exhausted.") {
		t.Fatalf("message = %q", got.Error.Message)
	}
	if rr.Header().Get("X-Proxy-Debug") != "m(k1):429 -> m(k2):429" {
		t.Fatalf("X-Proxy-Debug = %q", rr.Header().Get("X-Proxy-Debug"))
	}
}

func TestOpenAIChatCompletionsRoutesToGemini(t *testing.T) {
	var upstreamPath string
	var upstreamKey string
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamKey = r.URL.Query().Get("key")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "hello from gemini"}]},
				"finishReason": "STOP"
			}],
			"usageMetadata": {
				"promptTokenCount": 3,
				"candidatesTokenCount": 4,
				"totalTokenCount": 7
			}
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [
			{"role": "system", "content": "be brief"},
			{"role": "user", "content": "hi"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-3.5-flash:generateContent" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamKey != "AIza-secret" {
		t.Fatalf("upstream key = %q", upstreamKey)
	}
	if upstreamBody.SystemInstruction == nil || upstreamBody.SystemInstruction.Parts[0].Text != "be brief" {
		t.Fatalf("system instruction = %+v", upstreamBody.SystemInstruction)
	}
	if len(upstreamBody.Contents) != 1 || upstreamBody.Contents[0].Parts[0].Text != "hi" {
		t.Fatalf("contents = %+v", upstreamBody.Contents)
	}

	var got openAIChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].Message.Content == nil || *got.Choices[0].Message.Content != "hello from gemini" {
		t.Fatalf("content = %v", got.Choices[0].Message.Content)
	}
	if got.Usage.TotalTokens != 7 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestOpenAIChatTextPartsRouteToGemini(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "hi "},
				{"type": "text", "text": "there"}
			]
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamBody.Contents) != 1 || len(upstreamBody.Contents[0].Parts) != 2 {
		t.Fatalf("contents = %+v", upstreamBody.Contents)
	}
	if upstreamBody.Contents[0].Parts[0].Text != "hi " || upstreamBody.Contents[0].Parts[1].Text != "there" {
		t.Fatalf("contents = %+v", upstreamBody.Contents)
	}
}

func TestOpenAIChatToolsRouteToGemini(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamBody.Tools) != 1 || len(upstreamBody.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools = %+v", upstreamBody.Tools)
	}
	gotTool := upstreamBody.Tools[0].FunctionDeclarations[0]
	if gotTool.Name != "search" {
		t.Fatalf("tool = %+v", gotTool)
	}
	var parameters map[string]any
	if err := json.Unmarshal(gotTool.Parameters, &parameters); err != nil {
		t.Fatal(err)
	}
	if parameters["type"] != "object" {
		t.Fatalf("parameters = %+v", parameters)
	}
}

func TestOpenAIChatToolChoiceRoutesToGeminiToolConfig(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}],
		"tool_choice": {"type": "function", "function": {"name": "search"}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamBody.ToolConfig == nil || upstreamBody.ToolConfig.FunctionCallingConfig == nil {
		t.Fatalf("tool config = %+v", upstreamBody.ToolConfig)
	}
	cfg := upstreamBody.ToolConfig.FunctionCallingConfig
	if cfg.Mode != "ANY" {
		t.Fatalf("mode = %q", cfg.Mode)
	}
	if len(cfg.AllowedFunctionNames) != 1 || cfg.AllowedFunctionNames[0] != "search" {
		t.Fatalf("allowed functions = %+v", cfg.AllowedFunctionNames)
	}
}

func TestOpenAIChatToolChoiceRejectsUnknownFunction(t *testing.T) {
	_, err := openAIToCanonicalChatRequest(openAIChatRequest{
		Model: "gemini-3.5-flash",
		Messages: []openAIMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hi"`),
		}},
		Tools: json.RawMessage(`[{"type":"function","function":{"name":"search","parameters":{"type":"object"}}}]`),
		ToolChoice: json.RawMessage(`{
			"type": "function",
			"function": {"name": "lookup"}
		}`),
	})
	if err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIChatToolChoiceModes(t *testing.T) {
	tools := json.RawMessage(`[{"type":"function","function":{"name":"search","parameters":{"type":"object"}}}]`)
	for _, tt := range []struct {
		name string
		raw  json.RawMessage
		mode string
	}{
		{name: "none", raw: json.RawMessage(`"none"`), mode: "NONE"},
		{name: "required", raw: json.RawMessage(`"required"`), mode: "ANY"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := openAIToCanonicalChatRequest(openAIChatRequest{
				Model: "gemini-3.5-flash",
				Messages: []openAIMessage{{
					Role:    "user",
					Content: json.RawMessage(`"hi"`),
				}},
				Tools:      tools,
				ToolChoice: tt.raw,
			})
			if err != nil {
				t.Fatal(err)
			}
			if req.ToolChoice == nil || req.ToolChoice.Mode != tt.mode {
				t.Fatalf("tool choice = %+v", req.ToolChoice)
			}
		})
	}
}

func TestOpenAIChatGeminiFunctionCallToOpenAIToolCall(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"role": "model",
					"parts": [{
						"functionCall": {
							"name": "search",
							"args": {"query": "hello"}
						}
					}]
				},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got openAIChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].FinishReason == nil || *got.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %+v", got.Choices[0].FinishReason)
	}
	message := got.Choices[0].Message
	if message == nil || message.Content != nil || len(message.ToolCalls) != 1 {
		t.Fatalf("message = %+v", message)
	}
	call := message.ToolCalls[0]
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if call.Type != "function" || call.Function.Name != "search" || args["query"] != "hello" {
		t.Fatalf("tool call = %+v", call)
	}
}

func TestOpenAIChatReplaysGeminiThoughtSignature(t *testing.T) {
	var calls int
	var secondBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if calls == 1 {
			w.Write([]byte(`{
				"candidates": [{
					"content": {
						"role": "model",
						"parts": [{
							"thoughtSignature": "sig-abc",
							"functionCall": {
								"name": "search",
								"args": {"query": "hello"}
							}
						}]
					},
					"finishReason": "STOP"
				}]
			}`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&secondBody); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	firstBody := `{
		"model": "gemini-3.5-flash",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(firstBody))
	firstReq.Header.Set("Authorization", "Bearer bh_valid")
	firstRR := httptest.NewRecorder()
	srv.ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstRR.Code, firstRR.Body.String())
	}
	var firstResp openAIChatResponse
	if err := json.Unmarshal(firstRR.Body.Bytes(), &firstResp); err != nil {
		t.Fatal(err)
	}
	callID := firstResp.Choices[0].Message.ToolCalls[0].ID

	secondBodyJSON := `{
		"model": "gemini-3.5-flash",
		"messages": [
			{"role": "user", "content": "hi"},
			{"role": "assistant", "tool_calls": [{
				"id": "` + callID + `",
				"type": "function",
				"function": {"name": "search", "arguments": "{\"query\":\"hello\"}"}
			}]},
			{"role": "tool", "tool_call_id": "` + callID + `", "content": "{\"result\":\"ok\"}"}
		],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(secondBodyJSON))
	secondReq.Header.Set("Authorization", "Bearer bh_valid")
	secondRR := httptest.NewRecorder()
	srv.ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondRR.Code, secondRR.Body.String())
	}
	if len(secondBody.Contents) < 2 || len(secondBody.Contents[1].Parts) == 0 || secondBody.Contents[1].Parts[0].FunctionCall == nil {
		t.Fatalf("second body = %+v", secondBody)
	}
	if got := secondBody.Contents[1].Parts[0].ThoughtSignature; got != "sig-abc" {
		t.Fatalf("thought signature = %q", got)
	}
}

func TestOpenAIChatToolResultConvertsToGeminiFunctionResponse(t *testing.T) {
	req := openAIChatRequest{
		Model: "gemini-3.5-flash",
		Messages: []openAIMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`"what is the weather"`),
			},
			{
				Role:    "assistant",
				Content: json.RawMessage(`null`),
				ToolCalls: []openAIToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: openAIToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"Paris"}`,
					},
				}},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    json.RawMessage(`"{\"temperature\":21}"`),
			},
		},
	}

	canonicalReq, err := openAIToCanonicalChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	geminiReq, err := canonicalToGeminiGenerateRequest(canonicalReq)
	if err != nil {
		t.Fatal(err)
	}

	if len(geminiReq.Contents) != 3 {
		t.Fatalf("contents = %+v", geminiReq.Contents)
	}
	if geminiReq.Contents[1].Role != "model" || len(geminiReq.Contents[1].Parts) != 1 {
		t.Fatalf("assistant content = %+v", geminiReq.Contents[1])
	}
	call := geminiReq.Contents[1].Parts[0].FunctionCall
	if call == nil || call.Name != "get_weather" {
		t.Fatalf("function call = %+v", call)
	}
	var args map[string]any
	if err := json.Unmarshal(call.Args, &args); err != nil {
		t.Fatal(err)
	}
	if args["city"] != "Paris" {
		t.Fatalf("args = %+v", args)
	}

	if geminiReq.Contents[2].Role != "user" || len(geminiReq.Contents[2].Parts) != 1 {
		t.Fatalf("tool content = %+v", geminiReq.Contents[2])
	}
	response := geminiReq.Contents[2].Parts[0].FunctionResponse
	if response == nil || response.Name != "get_weather" {
		t.Fatalf("function response = %+v", response)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Response, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["temperature"] != float64(21) {
		t.Fatalf("response = %+v", payload)
	}
}

func TestOpenAIChatToolResultRoutesToGemini(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "sunny"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [
			{"role": "user", "content": "weather"},
			{
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {
						"name": "get_weather",
						"arguments": "{\"city\":\"Paris\"}"
					}
				}]
			},
			{
				"role": "tool",
				"tool_call_id": "call_1",
				"content": "{\"temperature\":21}"
			}
		],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"parameters": {"type": "object"}
			}
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamBody.Contents) != 3 {
		t.Fatalf("contents = %+v", upstreamBody.Contents)
	}
	if upstreamBody.Contents[1].Role != "model" || upstreamBody.Contents[1].Parts[0].FunctionCall == nil {
		t.Fatalf("assistant content = %+v", upstreamBody.Contents[1])
	}
	if upstreamBody.Contents[2].Role != "user" || upstreamBody.Contents[2].Parts[0].FunctionResponse == nil {
		t.Fatalf("tool content = %+v", upstreamBody.Contents[2])
	}
	response := upstreamBody.Contents[2].Parts[0].FunctionResponse
	if response.Name != "get_weather" {
		t.Fatalf("function response = %+v", response)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Response, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["temperature"] != float64(21) {
		t.Fatalf("response = %+v", payload)
	}

	var got openAIChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].Message.Content == nil || *got.Choices[0].Message.Content != "sunny" {
		t.Fatalf("content = %+v", got.Choices[0].Message)
	}
}

func TestOpenAIChatRejectsUnknownToolCallID(t *testing.T) {
	_, err := openAIToCanonicalChatRequest(openAIChatRequest{
		Model: "gemini-3.5-flash",
		Messages: []openAIMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
			{Role: "tool", ToolCallID: "missing", Content: json.RawMessage(`"ok"`)},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown tool_call_id") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIChatToolsStreamTranslatesGeminiFunctionCall(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search","args":{"query":"hello"}}}]},"finishReason":"STOP"}]}` + "\n\n"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"stream": true,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamBody.Tools) != 1 || len(upstreamBody.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools = %+v", upstreamBody.Tools)
	}
	var toolChunk openAIChatResponse
	for _, line := range strings.Split(rr.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var chunk openAIChatResponse
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			t.Fatal(err)
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			toolChunk = chunk
			break
		}
	}
	if len(toolChunk.Choices) == 0 {
		t.Fatalf("stream missing tool call chunk: %s", rr.Body.String())
	}
	call := toolChunk.Choices[0].Delta.ToolCalls[0]
	if call.Index == nil || *call.Index != 0 {
		t.Fatalf("tool call index = %v", call.Index)
	}
	if call.ID == "" || call.Type != "function" || call.Function.Name != "search" {
		t.Fatalf("tool call = %+v", call)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if args["query"] != "hello" {
		t.Fatalf("arguments = %+v", args)
	}
	if toolChunk.Choices[0].FinishReason == nil || *toolChunk.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %v", toolChunk.Choices[0].FinishReason)
	}
}

func TestOpenAIChatImageDataURLPartRoutesToGemini(t *testing.T) {
	var upstreamBody geminiGenerateRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model": "gemini-3.5-flash",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "describe"},
				{"type": "image_url", "image_url": {"url": "data:image/png;base64,aGVsbG8="}}
			]
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamBody.Contents) != 1 || len(upstreamBody.Contents[0].Parts) != 2 {
		t.Fatalf("contents = %+v", upstreamBody.Contents)
	}
	imagePart := upstreamBody.Contents[0].Parts[1]
	if imagePart.InlineData == nil || imagePart.InlineData.MimeType != "image/png" || imagePart.InlineData.Data != "aGVsbG8=" {
		t.Fatalf("image part = %+v", imagePart)
	}
}

func TestOpenAIChatRejectsRemoteImageURLPart(t *testing.T) {
	_, err := openAIToCanonicalChatRequest(openAIChatRequest{
		Model: "gemini-3.5-flash",
		Messages: []openAIMessage{{
			Role:    "user",
			Content: json.RawMessage(`[{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]`),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "data URL image") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIChatRejectsUnconfiguredModelRoute(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srv := &Server{store: store}
	body := `{"model":"missing-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "model_not_found") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestOpenAIChatUsesModelRouteUpstreamModel(t *testing.T) {
	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "target ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	provider, err := store.CreateProvider(ProviderRecord{
		Name:    providerGemini,
		Type:    providerGemini,
		BaseURL: upstream.URL,
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateProviderKey(ProviderKey{
		ProviderID: provider.ID,
		Name:       "k01-alpha",
		Secret:     "AIza-secret",
		Enabled:    true,
	}); err != nil {
		t.Fatal(err)
	}
	providerID := provider.ID
	now := time.Now().Format(time.RFC3339)
	modelID, err := store.insertReturningID(
		`INSERT INTO models (name, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
		"public-model", "round_robin", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.exec(
		`INSERT INTO model_routes (model_id, provider_id, upstream_model, quota_family, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, '', 1, 0, 1, ?, ?)`,
		modelID, providerID, "gemini-upstream", now, now,
	); err != nil {
		t.Fatal(err)
	}
	keys, err := store.ProviderAPIKeys(providerGemini)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{"model":"public-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-upstream:generateContent" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
}

func TestOpenAIChatStreamTranslatesGeminiSSE(t *testing.T) {
	var upstreamPath string
	var upstreamAlt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAlt = r.URL.Query().Get("alt")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hel\"}]}}]}\n\n"))
		w.Write([]byte("data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\"}]}\n\n"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, keys := createGeminiRouteTestStore(t, upstream.URL, "gemini-3.5-flash", "gemini-3.5-flash", "AIza-secret")
	defer store.Close()
	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{"model":"gemini-3.5-flash","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-3.5-flash:streamGenerateContent" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamAlt != "sse" {
		t.Fatalf("alt = %q", upstreamAlt)
	}
	bodyText := rr.Body.String()
	for _, want := range []string{
		`"role":"assistant"`,
		`"content":"hel"`,
		`"content":"lo"`,
		`"finish_reason":"stop"`,
		"data: [DONE]",
	} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("stream missing %q: %s", want, bodyText)
		}
	}
}

func TestOpenAIChatPassesThroughOpenAICompatibleProvider(t *testing.T) {
	var upstreamPath string
	var upstreamAuth string
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-upstream","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().Format(time.RFC3339)
	providerID, err := store.insertReturningID(
		`INSERT INTO providers (name, type, preset_id, base_url, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		"openrouter", "openai-compatible", "openrouter", upstream.URL, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.exec(
		`INSERT INTO provider_keys (provider_id, name, secret, secret_hint, enabled, priority, weight, labels, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 0, 1, '', ?, ?)`,
		providerID, "or-main", "sk-secret", "cret", now, now,
	); err != nil {
		t.Fatal(err)
	}
	modelID, err := store.insertReturningID(
		`INSERT INTO models (name, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
		"public-gpt", "round_robin", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.exec(
		`INSERT INTO model_routes (model_id, provider_id, upstream_model, quota_family, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, '', 1, 0, 1, ?, ?)`,
		modelID, providerID, "gpt-upstream", now, now,
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
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	body := `{
		"model":"public-gpt",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"type":"function","function":{"name":"search","parameters":{"type":"object"}}}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-secret" {
		t.Fatalf("upstream auth = %q", upstreamAuth)
	}
	if upstreamBody["model"] != "gpt-upstream" {
		t.Fatalf("upstream model = %v", upstreamBody["model"])
	}
	if tools, ok := upstreamBody["tools"].([]any); !ok || len(tools) != 1 {
		t.Fatalf("upstream tools = %#v", upstreamBody["tools"])
	}
	if !strings.Contains(rr.Body.String(), "chatcmpl-upstream") {
		t.Fatalf("response = %s", rr.Body.String())
	}
}

func TestOpenAICompatibleStreamPassThroughFlushesChunks(t *testing.T) {
	var releaseSecondOnce sync.Once
	releaseSecond := make(chan struct{})
	defer releaseSecondOnce.Do(func() { close(releaseSecond) })
	var upstreamAcceptEncoding string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: first\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-releaseSecond
		fmt.Fprint(w, "data: second\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().Format(time.RFC3339)
	providerID, err := store.insertReturningID(
		`INSERT INTO providers (name, type, preset_id, base_url, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		"openrouter", "openai-compatible", "openrouter", upstream.URL, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.exec(
		`INSERT INTO provider_keys (provider_id, name, secret, secret_hint, enabled, priority, weight, labels, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 0, 1, '', ?, ?)`,
		providerID, "or-main", "sk-secret", "cret", now, now,
	); err != nil {
		t.Fatal(err)
	}
	modelID, err := store.insertReturningID(
		`INSERT INTO models (name, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
		"public-gpt", "round_robin", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.exec(
		`INSERT INTO model_routes (model_id, provider_id, upstream_model, quota_family, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, '', 1, 0, 1, ?, ?)`,
		modelID, providerID, "gpt-upstream", now, now,
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
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 2

	proxy := httptest.NewServer(srv)
	defer proxy.Close()
	req, err := http.NewRequest(http.MethodPost, proxy.URL+"/v1/chat/completions", strings.NewReader(`{
		"model":"public-gpt",
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer bh_valid")
	req.Header.Set("Content-Type", "application/json")
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if upstreamAcceptEncoding != "identity" {
		t.Fatalf("upstream Accept-Encoding = %q, want identity", upstreamAcceptEncoding)
	}

	reader := bufio.NewReader(resp.Body)
	firstLine := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			readErr <- err
			return
		}
		firstLine <- line
	}()

	select {
	case line := <-firstLine:
		if line != "data: first\n" {
			t.Fatalf("first line = %q", line)
		}
	case err := <-readErr:
		t.Fatal(err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first stream chunk was buffered until the upstream response completed")
	}

	releaseSecondOnce.Do(func() { close(releaseSecond) })
}

func TestOpenAIModelsListsEnabledModels(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.CreateModel(Model{Name: "enabled-model", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateModel(Model{Name: "disabled-model", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		store: store,
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got openAIModelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "enabled-model" {
		t.Fatalf("models = %+v", got.Data)
	}
}

func TestNonOpenAIEndpointIsNotExposed(t *testing.T) {
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srv := &Server{store: store}
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/missing-model:generateContent", strings.NewReader(`{"contents":[]}`))
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not found") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestOpenAIChatSwitchesModelRoutesWhenRouteKeysCooling(t *testing.T) {
	var upstreamPaths []string
	var upstreamKeys []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPaths = append(upstreamPaths, r.URL.Path)
		upstreamKey := r.URL.Query().Get("key")
		upstreamKeys = append(upstreamKeys, upstreamKey)
		if strings.Contains(r.URL.Path, "/gemini-a:") {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "route ok"}]},
				"finishReason": "STOP"
			}]
		}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(DatabaseConfig{Path: filepath.Join(t.TempDir(), "buzzhive.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	provider, err := store.CreateProvider(ProviderRecord{
		Name:    providerGemini,
		Type:    providerGemini,
		BaseURL: upstream.URL,
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []ProviderKey{
		{ProviderID: provider.ID, Name: "k01-alpha", Secret: "AIza-a", Enabled: true},
		{ProviderID: provider.ID, Name: "k02-alpha", Secret: "AIza-b", Enabled: true},
	} {
		if _, err := store.CreateProviderKey(key); err != nil {
			t.Fatal(err)
		}
	}
	providerID := provider.ID
	now := time.Now().Format(time.RFC3339)
	modelID, err := store.insertReturningID(
		`INSERT INTO models (name, selection_policy, enabled, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
		"public-model", "round_robin", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		model string
	}{
		{"gemini-a"},
		{"gemini-b"},
	} {
		if _, err := store.exec(
			`INSERT INTO model_routes (model_id, provider_id, upstream_model, quota_family, enabled, priority, weight, created_at, updated_at) VALUES (?, ?, ?, '', 1, 0, 1, ?, ?)`,
			modelID, providerID, target.model, now, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := store.RuntimeProviderAPIKeys()
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		store:    store,
		upstream: upstreamURL,
		client:   upstream.Client(),
		providers: map[string]Provider{
			providerGemini: NewGeminiProvider(upstreamURL, upstream.Client()),
		},
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "alice-key", UserName: "alice", Valid: true},
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
	srv.cfg.Retry.MaxAttempts = 4

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"public-model","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(upstreamPaths) != 3 {
		t.Fatalf("upstream calls = %d, paths = %v keys = %v", len(upstreamPaths), upstreamPaths, upstreamKeys)
	}
	if upstreamPaths[0] != "/v1beta/models/gemini-a:generateContent" || upstreamPaths[1] != "/v1beta/models/gemini-a:generateContent" || upstreamPaths[2] != "/v1beta/models/gemini-b:generateContent" {
		t.Fatalf("upstream paths = %v", upstreamPaths)
	}
	if upstreamKeys[0] != "AIza-a" || upstreamKeys[1] != "AIza-b" || upstreamKeys[2] == "" {
		t.Fatalf("upstream keys = %v", upstreamKeys)
	}
}
