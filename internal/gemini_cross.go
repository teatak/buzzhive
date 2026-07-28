package buzzhive

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) proxyGeminiViaOpenAIChat(w http.ResponseWriter, r *http.Request, canonicalReq protocol.CanonicalRequest, user AuthToken, model string, targets []RouteTarget) {
	chatReq, err := protocol.CanonicalToOpenAIChatRequest(canonicalReq)
	if err != nil {
		writeGeminiError(w, http.StatusBadRequest, err.Error())
		return
	}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		writeGeminiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerGemini,
			Method:          http.MethodPost,
			Path:            "/v1/chat/completions",
			Headers:         headers,
			Body:            rewriteOpenAIModel(chatBody, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	s.writeGeminiConvertedResult(w, result, user, model, func(resp *http.Response, requestID string, started int64) (protocol.GeminiGenerateResponse, TokenUsage, error) {
		raw := drain(resp.Body, 8*1024*1024)
		var chatResp protocol.OpenAIChatResponse
		if err := json.Unmarshal(raw, &chatResp); err != nil {
			return protocol.GeminiGenerateResponse{}, TokenUsage{}, err
		}
		canonicalResp := protocol.OpenAIChatResponseToCanonical(chatResp)
		canonicalResp.Model = model
		return protocol.CanonicalToGeminiGenerateResponse(canonicalResp), tokenUsageFromOpenAIResponseBody(raw), nil
	}, func(resp *http.Response, requestID string) TokenUsage {
		return s.streamOpenAIChatAsGemini(w, resp, requestID)
	})
}

func (s *Server) proxyGeminiViaOpenAIResponses(w http.ResponseWriter, r *http.Request, canonicalReq protocol.CanonicalRequest, user AuthToken, model string, targets []RouteTarget) {
	responsesReq, err := protocol.CanonicalToOpenAIResponsesRequest(canonicalReq)
	if err != nil {
		writeGeminiError(w, http.StatusBadRequest, err.Error())
		return
	}
	responsesBody, err := json.Marshal(responsesReq)
	if err != nil {
		writeGeminiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerGemini,
			Method:          http.MethodPost,
			Path:            "/v1/responses",
			Headers:         headers,
			Body:            rewriteOpenAIModel(responsesBody, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	s.writeGeminiConvertedResult(w, result, user, model, func(resp *http.Response, requestID string, started int64) (protocol.GeminiGenerateResponse, TokenUsage, error) {
		raw := drain(resp.Body, 8*1024*1024)
		var responsesResp protocol.OpenAIResponsesResponse
		if err := json.Unmarshal(raw, &responsesResp); err != nil {
			return protocol.GeminiGenerateResponse{}, TokenUsage{}, err
		}
		canonicalResp, err := protocol.OpenAIResponsesResponseToCanonical(responsesResp)
		if err != nil {
			return protocol.GeminiGenerateResponse{}, TokenUsage{}, err
		}
		canonicalResp.Model = model
		return protocol.CanonicalToGeminiGenerateResponse(canonicalResp), tokenUsageFromOpenAIResponseBody(raw), nil
	}, func(resp *http.Response, requestID string) TokenUsage {
		return s.streamResponsesAsGemini(w, resp)
	})
}

func (s *Server) proxyGeminiViaAnthropic(w http.ResponseWriter, r *http.Request, canonicalReq protocol.CanonicalRequest, user AuthToken, model string, targets []RouteTarget) {
	anthropicReq, err := protocol.CanonicalToAnthropicMessagesRequest(canonicalReq)
	if err != nil {
		writeGeminiError(w, http.StatusBadRequest, err.Error())
		return
	}
	anthropicBody, err := json.Marshal(anthropicReq)
	if err != nil {
		writeGeminiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerGemini,
			Method:          http.MethodPost,
			Path:            "/v1/messages",
			Headers:         headers,
			Body:            rewriteOpenAIModel(anthropicBody, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	s.writeGeminiConvertedResult(w, result, user, model, func(resp *http.Response, requestID string, started int64) (protocol.GeminiGenerateResponse, TokenUsage, error) {
		raw := drain(resp.Body, 8*1024*1024)
		var anthropicResp protocol.AnthropicMessagesResponse
		if err := json.Unmarshal(raw, &anthropicResp); err != nil {
			return protocol.GeminiGenerateResponse{}, TokenUsage{}, err
		}
		canonicalResp, err := protocol.AnthropicMessagesResponseToCanonical(anthropicResp)
		if err != nil {
			return protocol.GeminiGenerateResponse{}, TokenUsage{}, err
		}
		canonicalResp.Model = model
		return protocol.CanonicalToGeminiGenerateResponse(canonicalResp), tokenUsageFromCanonical(canonicalResp.Usage), nil
	}, func(resp *http.Response, requestID string) TokenUsage {
		return s.streamAnthropicAsGemini(w, resp)
	})
}

func (s *Server) writeGeminiConvertedResult(
	w http.ResponseWriter,
	result ProviderAttemptResult,
	user AuthToken,
	model string,
	nonStream func(*http.Response, string, int64) (protocol.GeminiGenerateResponse, TokenUsage, error),
	stream func(*http.Response, string) TokenUsage,
) {
	if !s.convertedResultReady(w, providerGemini, user, model, result) {
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		usage := stream(resp, result.RequestID)
		s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
		return
	}
	out, usage, err := nonStream(resp, result.RequestID, result.StartedAt.Unix())
	if err != nil {
		writeGeminiError(w, http.StatusBadGateway, err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
}

func (s *Server) streamOpenAIChatAsGemini(w http.ResponseWriter, resp *http.Response, requestID string) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	usage, err := readOpenAIChatStreamAsCanonical(resp.Body, func(event protocol.CanonicalStreamEvent) {
		writeGeminiStreamEvent(w, flusher, event)
	})
	if err != nil {
		writeGeminiStreamEvent(w, flusher, canonicalStreamReadError(err))
	}
	return tokenUsageFromCanonical(usage)
}

func (s *Server) streamResponsesAsGemini(w http.ResponseWriter, resp *http.Response) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	usage, err := readResponsesStreamAsCanonical(resp.Body, func(event protocol.CanonicalStreamEvent) {
		writeGeminiStreamEvent(w, flusher, event)
	})
	if err != nil {
		writeGeminiStreamEvent(w, flusher, canonicalStreamReadError(err))
	}
	return tokenUsageFromCanonical(usage)
}

func (s *Server) streamAnthropicAsGemini(w http.ResponseWriter, resp *http.Response) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	usage, err := readAnthropicStreamAsCanonical(resp.Body, func(event protocol.CanonicalStreamEvent) {
		writeGeminiStreamEvent(w, flusher, event)
	})
	if err != nil {
		writeGeminiStreamEvent(w, flusher, canonicalStreamReadError(err))
	}
	return tokenUsageFromCanonical(usage)
}

func tokenUsageFromCanonical(usage protocol.CanonicalUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:     int64(usage.PromptTokens),
		CompletionTokens: int64(usage.CompletionTokens),
		TotalTokens:      int64(usage.TotalTokens),
		CachedTokens:     int64(usage.CachedTokens),
		ReasoningTokens:  int64(usage.ReasoningTokens),
	}
}
