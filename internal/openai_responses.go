package buzzhive

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) handleOpenAIResponses(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req protocol.OpenAIResponsesRequest
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
	targets = openAIResponsesTargets(targets)
	if len(targets) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support OpenAI Responses")
		return
	}
	target := targets[0]
	if protocol.ShouldPassthrough(providerOpenAIResponses, target.ProviderType) {
		s.proxyRaw(w, r, body, user, req.Model, targets)
		return
	}
	canonicalReq, err := protocol.OpenAIResponsesToCanonicalRequest(req)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	switch target.ProviderType {
	case providerOpenAI:
		s.proxyOpenAIResponsesViaOpenAIChat(w, r, canonicalReq, user, req.Model, targets)
	case providerGemini:
		thinkingLevel, err := geminiThinkingLevelForOpenAIReasoningEffort(canonicalReq.ThinkingLevel, target.UpstreamModel)
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		canonicalReq.ThinkingLevel = thinkingLevel
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
		s.proxyOpenAIResponsesViaGemini(w, r, geminiBody, user, req.Model, targets, req.Stream)
	default:
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support OpenAI Responses")
	}
}

func openAIResponsesTargets(targets []RouteTarget) []RouteTarget {
	for _, protocol := range []string{providerOpenAIResponses, providerOpenAI, providerGemini} {
		out := routeTargetsByProtocol(targets, protocol)
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (s *Server) proxyOpenAIResponsesViaOpenAIChat(w http.ResponseWriter, r *http.Request, canonicalReq protocol.ChatRequest, user AuthToken, model string, targets []RouteTarget) {
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
			InboundProtocol: providerOpenAIResponses,
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
		usage := s.streamOpenAIChatAsResponses(w, resp, "resp_"+result.RequestID, result.StartedAt.Unix(), model)
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
	out := protocol.CanonicalToOpenAIResponsesResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromOpenAIResponseBody(raw))
}

func (s *Server) proxyOpenAIResponsesViaGemini(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget, stream bool) {
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerOpenAIResponses,
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
		usage := s.streamGeminiAsResponses(w, resp, result.RequestID, "resp_"+result.RequestID, result.StartedAt.Unix(), model)
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
	canonicalResp := protocol.GeminiToCanonicalChatResponse(geminiResp, model, "resp_"+result.RequestID, result.StartedAt.Unix(), result.RequestID)
	s.rememberToolSignatures(canonicalResp.ToolCalls)
	out := protocol.CanonicalToOpenAIResponsesResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
}

func (s *Server) streamOpenAIChatAsResponses(w http.ResponseWriter, resp *http.Response, id string, created int64, model string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	writeResponsesStreamStart(w, flusher, id, created, model)
	var text strings.Builder
	sequence := 4
	usage := readOpenAIChatStreamAsCanonical(resp.Body, func(event protocol.ChatStreamEvent) {
		if event.Text != "" {
			text.WriteString(event.Text)
			writeResponsesStreamDelta(w, flusher, id, sequence, event.Text)
			sequence++
		}
	})
	writeResponsesStreamDone(w, flusher, id, created, model, text.String(), usage, sequence)
	return tokenUsageFromCanonical(usage)
}

func (s *Server) streamGeminiAsResponses(w http.ResponseWriter, resp *http.Response, requestID string, id string, created int64, model string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	writeResponsesStreamStart(w, flusher, id, created, model)
	var text strings.Builder
	sequence := 4
	usage := readGeminiStreamAsCanonical(resp.Body, requestID, func(event protocol.ChatStreamEvent) {
		if event.Text != "" {
			text.WriteString(event.Text)
			writeResponsesStreamDelta(w, flusher, id, sequence, event.Text)
			sequence++
		}
	})
	writeResponsesStreamDone(w, flusher, id, created, model, text.String(), usage, sequence)
	return tokenUsageFromCanonical(usage)
}
