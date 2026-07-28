package buzzhive

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) handleAnthropicPassthrough(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken) {
	if r.Method != http.MethodPost {
		writeAnthropicError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}
	var req protocol.AnthropicMessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if req.Model == "" {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if isAutoModel(req.Model) {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "auto model routing has been removed")
		return
	}

	targets, err := s.resolveRouteTargets(req.Model)
	if err != nil {
		if errors.Is(err, errModelRouteNotFound) {
			writeAnthropicError(w, http.StatusNotFound, "not_found_error", err.Error())
			return
		}
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	targets = anthropicTargets(targets)
	if len(targets) == 0 {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "selected upstream does not support Anthropic Messages")
		return
	}
	target := targets[0]
	if protocol.ShouldPassthrough(providerAnthropic, target.ProviderType) {
		s.proxyRaw(w, r, body, user, req.Model, targets)
		return
	}
	if err := decodeCrossProtocolJSON(body, &req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if err := validateAnthropicCrossProtocolContent(body); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	canonicalReq, err := protocol.AnthropicMessagesToCanonicalRequest(req)
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	switch target.ProviderType {
	case providerOpenAI:
		s.proxyAnthropicViaOpenAIChat(w, r, canonicalReq, user, req.Model, targets)
	case providerOpenAIResponses:
		s.proxyAnthropicViaOpenAIResponses(w, r, canonicalReq, user, req.Model, targets)
	case providerGemini:
		s.applyToolSignatures(&canonicalReq)
		geminiReq, err := protocol.CanonicalToGeminiGenerateRequest(canonicalReq)
		if err != nil {
			writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		geminiBody, err := json.Marshal(geminiReq)
		if err != nil {
			writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
			return
		}
		s.proxyAnthropicViaGemini(w, r, geminiBody, user, req.Model, targets, req.Stream)
	default:
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "selected upstream does not support Anthropic Messages")
	}
}

type anthropicStrictMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func validateAnthropicCrossProtocolContent(body []byte) error {
	var request struct {
		Model         string                          `json:"model"`
		System        json.RawMessage                 `json:"system,omitempty"`
		Messages      []anthropicStrictMessage        `json:"messages"`
		MaxTokens     *int                            `json:"max_tokens,omitempty"`
		Temperature   *float64                        `json:"temperature,omitempty"`
		TopP          *float64                        `json:"top_p,omitempty"`
		StopSequences []string                        `json:"stop_sequences,omitempty"`
		Tools         []protocol.AnthropicTool        `json:"tools,omitempty"`
		ToolChoice    *protocol.AnthropicToolChoice   `json:"tool_choice,omitempty"`
		Thinking      *protocol.AnthropicThinking     `json:"thinking,omitempty"`
		OutputConfig  *protocol.AnthropicOutputConfig `json:"output_config,omitempty"`
		Stream        bool                            `json:"stream,omitempty"`
	}
	if err := decodeCrossProtocolJSON(body, &request); err != nil {
		return err
	}
	if len(request.System) > 0 {
		if err := validateAnthropicContentValue(request.System); err != nil {
			return err
		}
	}
	for _, message := range request.Messages {
		if err := validateAnthropicContentValue(message.Content); err != nil {
			return err
		}
	}
	return nil
}

func validateAnthropicContentValue(raw json.RawMessage) error {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return nil
	}
	var blocks []json.RawMessage
	if err := decodeCrossProtocolJSON(raw, &blocks); err != nil {
		return err
	}
	for _, block := range blocks {
		var content protocol.AnthropicContent
		if err := decodeCrossProtocolJSON(block, &content); err != nil {
			return err
		}
	}
	return nil
}

func anthropicTargets(targets []RouteTarget) []RouteTarget {
	for _, protocol := range []string{providerAnthropic, providerOpenAI, providerOpenAIResponses, providerGemini} {
		out := routeTargetsByProtocol(targets, protocol)
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (s *Server) proxyAnthropicViaOpenAIChat(w http.ResponseWriter, r *http.Request, canonicalReq protocol.CanonicalRequest, user AuthToken, model string, targets []RouteTarget) {
	chatReq, err := protocol.CanonicalToOpenAIChatRequest(canonicalReq)
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerAnthropic,
			Method:          http.MethodPost,
			Path:            "/v1/chat/completions",
			Headers:         headers,
			Body:            rewriteOpenAIModel(chatBody, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !result.OK {
		s.recordProviderResultUsage(user, model, result, providerResultStatus(result.Response))
		writeInboundRetryError(w, providerAnthropic, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeInboundUpstreamError(w, providerAnthropic, resp.StatusCode, raw)
		s.recordProviderResultUsage(user, model, result, resp.StatusCode)
		return
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		usage := s.streamOpenAIChatAsAnthropic(w, resp, "msg_"+result.RequestID, model)
		s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var chatResp protocol.OpenAIChatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp := protocol.OpenAIChatResponseToCanonical(chatResp)
	canonicalResp.Model = model
	out := protocol.CanonicalToAnthropicMessagesResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromOpenAIResponseBody(raw))
}

func (s *Server) proxyAnthropicViaGemini(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget, stream bool) {
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerAnthropic,
			Method:          http.MethodPost,
			Body:            body,
			RequestedModel:  model,
			Model:           target.UpstreamModel,
			Action:          action,
		}
	})
	if !result.OK {
		s.recordProviderResultUsage(user, model, result, providerResultStatus(result.Response))
		writeInboundRetryError(w, providerAnthropic, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeInboundUpstreamError(w, providerAnthropic, resp.StatusCode, raw)
		s.recordProviderResultUsage(user, model, result, resp.StatusCode)
		return
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		usage := s.streamGeminiAsAnthropic(w, resp, result.RequestID, "msg_"+result.RequestID, model)
		s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var geminiResp protocol.GeminiGenerateResponse
	if err := json.Unmarshal(raw, &geminiResp); err != nil {
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	usage := tokenUsageFromGeminiResponseBody(raw, geminiResp)
	canonicalResp := protocol.GeminiToCanonicalResponse(geminiResp, model, "msg_"+result.RequestID, result.StartedAt.Unix(), result.RequestID)
	s.rememberToolSignatures(canonicalResp.ToolCalls)
	out := protocol.CanonicalToAnthropicMessagesResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
}

func (s *Server) streamOpenAIChatAsAnthropic(w http.ResponseWriter, resp *http.Response, id string, model string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	encoder := newAnthropicStreamEncoder(w, flusher, id, model)
	usage, err := readOpenAIChatStreamAsCanonical(resp.Body, func(event protocol.CanonicalStreamEvent) {
		encoder.writeEvent(event)
	})
	if err != nil {
		encoder.writeEvent(canonicalStreamReadError(err))
	} else {
		encoder.finish("stop", usage)
	}
	return tokenUsageFromCanonical(usage)
}

func (s *Server) streamGeminiAsAnthropic(w http.ResponseWriter, resp *http.Response, requestID string, id string, model string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	encoder := newAnthropicStreamEncoder(w, flusher, id, model)
	usage, err := readGeminiStreamAsCanonical(resp.Body, requestID, func(event protocol.CanonicalStreamEvent) {
		encoder.writeEvent(event)
	})
	if err != nil {
		encoder.writeEvent(canonicalStreamReadError(err))
	} else {
		encoder.finish("stop", usage)
	}
	return tokenUsageFromCanonical(usage)
}
