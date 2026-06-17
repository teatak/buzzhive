package buzzhive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/teatak/buzzhive/internal/protocol"
)

func createRouteTestServer(t *testing.T, proto string, baseURL string, publicModel string, upstreamModel string, keySecret string, client *http.Client) (*Server, *Store) {
	t.Helper()
	upstreamURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	store := openTestStore(t)
	provider, err := store.CreateProvider(ProviderRecord{
		Name: proto + "-provider",
		Endpoints: []ProviderEndpoint{{
			Protocol: proto,
			BaseURL:  baseURL,
			Enabled:  true,
		}},
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateProviderKey(ProviderKey{ProviderID: provider.ID, Name: proto + "-key", Secret: keySecret, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	model, err := store.CreateModel(Model{Name: publicModel, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateModelRoute(ModelRoute{ModelID: model.ID, ProviderID: provider.ID, UpstreamModel: upstreamModel, Enabled: true, Weight: 1}); err != nil {
		t.Fatal(err)
	}
	providerRecords, err := store.EnabledProviders()
	if err != nil {
		t.Fatal(err)
	}
	providers, err := newProviderRegistry(providerRecords, upstreamURL, client)
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
		client:    client,
		providers: providers,
		authTokens: map[string]AuthToken{
			"bh_valid": {Name: "user-key", UserName: "user1", Valid: true},
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
	return srv, store
}

func TestGeminiRoutesToOpenAIChat(t *testing.T) {
	var upstreamPath string
	var upstreamBody protocol.OpenAIChatRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-1","created":123,"model":"gpt-upstream","choices":[{"message":{"role":"assistant","content":"hello chat"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
	}))
	defer upstream.Close()
	srv, store := createRouteTestServer(t, providerOpenAI, upstream.URL+"/v1", "gemini-public-chat", "gpt-upstream", "sk-secret", upstream.Client())
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public-chat:generateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamBody.Model != "gpt-upstream" || len(upstreamBody.Messages) != 1 {
		t.Fatalf("upstream body = %+v", upstreamBody)
	}
	var got protocol.GeminiGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Content.Parts[0].Text != "hello chat" || got.UsageMetadata.TotalTokenCount != 5 {
		t.Fatalf("response = %+v", got)
	}
}

func TestGeminiRoutesToAnthropic(t *testing.T) {
	var upstreamPath string
	var upstreamBody protocol.AnthropicMessagesRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-upstream","content":[{"type":"text","text":"hello anthropic"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":6}}`))
	}))
	defer upstream.Close()
	srv, store := createRouteTestServer(t, providerAnthropic, upstream.URL, "gemini-public-anthropic", "claude-upstream", "sk-ant", upstream.Client())
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public-anthropic:generateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1/messages" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamBody.Model != "claude-upstream" || len(upstreamBody.Messages) != 1 {
		t.Fatalf("upstream body = %+v", upstreamBody)
	}
	var got protocol.GeminiGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Content.Parts[0].Text != "hello anthropic" || got.UsageMetadata.TotalTokenCount != 10 {
		t.Fatalf("response = %+v", got)
	}
}

func TestGeminiRoutesToOpenAIResponses(t *testing.T) {
	var upstreamPath string
	var upstreamBody protocol.OpenAIResponsesRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"resp_1","object":"response","created_at":123,"status":"completed","model":"resp-upstream","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello responses"}]}],"usage":{"input_tokens":2,"output_tokens":8,"total_tokens":10}}`))
	}))
	defer upstream.Close()
	srv, store := createRouteTestServer(t, providerOpenAIResponses, upstream.URL+"/v1", "gemini-public-responses", "resp-upstream", "sk-resp", upstream.Client())
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public-responses:generateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if upstreamPath != "/v1/responses" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamBody.Model != "resp-upstream" {
		t.Fatalf("upstream body = %+v", upstreamBody)
	}
	var got protocol.GeminiGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Content.Parts[0].Text != "hello responses" || got.UsageMetadata.TotalTokenCount != 10 {
		t.Fatalf("response = %+v", got)
	}
}

func TestOpenAIResponsesStreamRoutesToGemini(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"hello "}]},"finishReason":""}]}`+"\n\n")
		fmt.Fprint(w, `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"stream"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"totalTokenCount":5}}`+"\n\n")
	}))
	defer upstream.Close()
	srv, store := createRouteTestServer(t, providerGemini, upstream.URL, "responses-stream-gemini", "gemini-upstream", "AIza-secret", upstream.Client())
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"responses-stream-gemini","input":"hi","stream":true}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "response.output_text.delta") || !strings.Contains(body, "hello ") || !strings.Contains(body, "stream") || !strings.Contains(body, "response.completed") {
		t.Fatalf("stream body = %s", body)
	}
}

func TestAnthropicStreamRoutesToOpenAIChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"hello "}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"stream"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	srv, store := createRouteTestServer(t, providerOpenAI, upstream.URL+"/v1", "anthropic-stream-chat", "gpt-upstream", "sk-secret", upstream.Client())
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"anthropic-stream-chat","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	req.Header.Set("Authorization", "Bearer bh_valid")
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "message_start") || !strings.Contains(body, "content_block_delta") || !strings.Contains(body, "hello ") || !strings.Contains(body, "stream") || !strings.Contains(body, "message_stop") {
		t.Fatalf("stream body = %s", body)
	}
}

func TestReadAnthropicStreamUsageMergesMessageStart(t *testing.T) {
	stream := strings.NewReader(
		`data: {"type":"message_start","message":{"usage":{"input_tokens":25,"output_tokens":1,"cache_read_input_tokens":7}}}` + "\n\n" +
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}` + "\n\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}` + "\n\n",
	)
	var events []protocol.ChatStreamEvent
	usage := readAnthropicStreamAsCanonical(stream, func(event protocol.ChatStreamEvent) {
		events = append(events, event)
	})

	if usage.PromptTokens != 25 || usage.CompletionTokens != 15 || usage.TotalTokens != 40 || usage.CachedTokens != 7 {
		t.Fatalf("usage = %+v", usage)
	}
	if len(events) != 2 || events[0].Text != "hello" || events[1].FinishReason != "stop" {
		t.Fatalf("events = %+v", events)
	}
}
