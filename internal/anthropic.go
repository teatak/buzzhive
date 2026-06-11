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
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req protocol.AnthropicMessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if req.Model == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if isAutoModel(req.Model) {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "auto model routing has been removed")
		return
	}

	targets, err := s.resolveRouteTargets(req.Model)
	if err != nil {
		if errors.Is(err, errModelRouteNotFound) {
			writeOpenAIError(w, http.StatusNotFound, "model_not_found", err.Error())
			return
		}
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	targets = anthropicTargets(targets)
	if len(targets) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support Anthropic Messages")
		return
	}
	target := targets[0]
	if protocol.ShouldPassthrough(providerAnthropic, target.ProviderType) {
		s.proxyRaw(w, r, body, user, req.Model, targets)
		return
	}
	canonicalReq, err := protocol.AnthropicMessagesToCanonicalRequest(req)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	switch target.ProviderType {
	case providerOpenAI:
		s.proxyAnthropicViaOpenAIChat(w, r, canonicalReq, user, req.Model, targets)
	case providerGemini:
		s.applyToolSignatures(&canonicalReq)
		geminiReq, err := protocol.CanonicalToGeminiGenerateRequest(canonicalReq)
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		geminiBody, err := json.Marshal(geminiReq)
		if err != nil {
			writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		s.proxyAnthropicViaGemini(w, r, geminiBody, user, req.Model, targets, req.Stream)
	default:
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support Anthropic Messages")
	}
}

func anthropicTargets(targets []RouteTarget) []RouteTarget {
	for _, protocol := range []string{providerAnthropic, providerOpenAI, providerGemini} {
		out := routeTargetsByProtocol(targets, protocol)
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (s *Server) proxyAnthropicViaOpenAIChat(w http.ResponseWriter, r *http.Request, canonicalReq protocol.ChatRequest, user AuthToken, model string, targets []RouteTarget) {
	chatReq, err := protocol.CanonicalToOpenAIChatRequest(canonicalReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
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
		writeOpenAIRetryError(w, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeOpenAIUpstreamError(w, resp.StatusCode, raw)
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
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
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
		writeOpenAIRetryError(w, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeOpenAIUpstreamError(w, resp.StatusCode, raw)
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
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	usage := tokenUsageFromGeminiResponseBody(raw, geminiResp)
	canonicalResp := protocol.GeminiToCanonicalChatResponse(geminiResp, model, "msg_"+result.RequestID, result.StartedAt.Unix(), result.RequestID)
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
	writeAnthropicStreamStart(w, flusher, id, model)
	usage := readOpenAIChatStreamAsCanonical(resp.Body, func(event protocol.ChatStreamEvent) {
		writeAnthropicTextDelta(w, flusher, event.Text)
	})
	writeAnthropicStreamDone(w, flusher, usage)
	return tokenUsageFromCanonical(usage)
}

func (s *Server) streamGeminiAsAnthropic(w http.ResponseWriter, resp *http.Response, requestID string, id string, model string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	writeAnthropicStreamStart(w, flusher, id, model)
	usage := readGeminiStreamAsCanonical(resp.Body, requestID, func(event protocol.ChatStreamEvent) {
		writeAnthropicTextDelta(w, flusher, event.Text)
	})
	writeAnthropicStreamDone(w, flusher, usage)
	return tokenUsageFromCanonical(usage)
}
