package buzzhive

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) proxyOpenAIChatViaResponses(
	w http.ResponseWriter,
	r *http.Request,
	canonicalReq protocol.CanonicalRequest,
	user AuthToken,
	model string,
	targets []RouteTarget,
	includeUsage bool,
) {
	upstreamReq, err := protocol.CanonicalToOpenAIResponsesRequest(canonicalReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	body, err := json.Marshal(upstreamReq)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerOpenAI,
			Method:          http.MethodPost,
			Path:            "/v1/responses",
			Headers:         headers,
			Body:            rewriteOpenAIModel(body, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !s.convertedResultReady(w, providerOpenAI, user, model, result) {
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if isEventStream(resp) {
		usage := streamCanonicalAsOpenAIChat(
			w,
			"chatcmpl-"+result.RequestID,
			result.StartedAt.Unix(),
			model,
			includeUsage,
			func(emit func(protocol.CanonicalStreamEvent)) (protocol.CanonicalUsage, error) {
				return readResponsesStreamAsCanonical(resp.Body, emit)
			},
		)
		s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var upstreamResp protocol.OpenAIResponsesResponse
	if err := json.Unmarshal(raw, &upstreamResp); err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp, err := protocol.OpenAIResponsesResponseToCanonical(upstreamResp)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp.ID = "chatcmpl-" + result.RequestID
	canonicalResp.Created = result.StartedAt.Unix()
	canonicalResp.Model = model
	writeConvertedHeaders(w, result)
	writeJSON(w, http.StatusOK, protocol.CanonicalToOpenAIChatResponse(canonicalResp))
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(canonicalResp.Usage))
}

func (s *Server) proxyOpenAIChatViaAnthropic(
	w http.ResponseWriter,
	r *http.Request,
	canonicalReq protocol.CanonicalRequest,
	user AuthToken,
	model string,
	targets []RouteTarget,
	includeUsage bool,
) {
	upstreamReq, err := protocol.CanonicalToAnthropicMessagesRequest(canonicalReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	body, err := json.Marshal(upstreamReq)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		headers := cleanHeaders(r.Header)
		headers.Set("Content-Type", "application/json")
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: providerOpenAI,
			Method:          http.MethodPost,
			Path:            "/v1/messages",
			Headers:         headers,
			Body:            rewriteOpenAIModel(body, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !s.convertedResultReady(w, providerOpenAI, user, model, result) {
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if isEventStream(resp) {
		usage := streamCanonicalAsOpenAIChat(
			w,
			"chatcmpl-"+result.RequestID,
			result.StartedAt.Unix(),
			model,
			includeUsage,
			func(emit func(protocol.CanonicalStreamEvent)) (protocol.CanonicalUsage, error) {
				return readAnthropicStreamAsCanonical(resp.Body, emit)
			},
		)
		s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var upstreamResp protocol.AnthropicMessagesResponse
	if err := json.Unmarshal(raw, &upstreamResp); err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp, err := protocol.AnthropicMessagesResponseToCanonical(upstreamResp)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp.ID = "chatcmpl-" + result.RequestID
	canonicalResp.Created = result.StartedAt.Unix()
	canonicalResp.Model = model
	writeConvertedHeaders(w, result)
	writeJSON(w, http.StatusOK, protocol.CanonicalToOpenAIChatResponse(canonicalResp))
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(canonicalResp.Usage))
}

func (s *Server) proxyOpenAIResponsesViaAnthropic(
	w http.ResponseWriter,
	r *http.Request,
	canonicalReq protocol.CanonicalRequest,
	user AuthToken,
	model string,
	targets []RouteTarget,
) {
	upstreamReq, err := protocol.CanonicalToAnthropicMessagesRequest(canonicalReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	body, err := json.Marshal(upstreamReq)
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
			Path:            "/v1/messages",
			Headers:         headers,
			Body:            rewriteOpenAIModel(body, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !s.convertedResultReady(w, providerOpenAIResponses, user, model, result) {
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if isEventStream(resp) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		writeConvertedHeaders(w, result)
		w.WriteHeader(http.StatusOK)
		encoder := newResponsesStreamEncoder(w, flusher, "resp_"+result.RequestID, result.StartedAt.Unix(), model)
		usage, err := readAnthropicStreamAsCanonical(resp.Body, encoder.writeEvent)
		if err != nil {
			encoder.writeEvent(canonicalStreamReadError(err))
		} else {
			encoder.finish("stop", usage)
		}
		s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(usage))
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var upstreamResp protocol.AnthropicMessagesResponse
	if err := json.Unmarshal(raw, &upstreamResp); err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp, err := protocol.AnthropicMessagesResponseToCanonical(upstreamResp)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp.ID = "resp_" + result.RequestID
	canonicalResp.Created = result.StartedAt.Unix()
	canonicalResp.Model = model
	writeConvertedHeaders(w, result)
	writeJSON(w, http.StatusOK, protocol.CanonicalToOpenAIResponsesResponse(canonicalResp))
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(canonicalResp.Usage))
}

func (s *Server) proxyAnthropicViaOpenAIResponses(
	w http.ResponseWriter,
	r *http.Request,
	canonicalReq protocol.CanonicalRequest,
	user AuthToken,
	model string,
	targets []RouteTarget,
) {
	upstreamReq, err := protocol.CanonicalToOpenAIResponsesRequest(canonicalReq)
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	body, err := json.Marshal(upstreamReq)
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
			Path:            "/v1/responses",
			Headers:         headers,
			Body:            rewriteOpenAIModel(body, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !s.convertedResultReady(w, providerAnthropic, user, model, result) {
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if isEventStream(resp) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		writeConvertedHeaders(w, result)
		w.WriteHeader(http.StatusOK)
		encoder := newAnthropicStreamEncoder(w, flusher, "msg_"+result.RequestID, model)
		usage, err := readResponsesStreamAsCanonical(resp.Body, encoder.writeEvent)
		if err != nil {
			encoder.writeEvent(canonicalStreamReadError(err))
		} else {
			encoder.finish("stop", usage)
		}
		s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(usage))
		return
	}
	raw := drain(resp.Body, 8*1024*1024)
	var upstreamResp protocol.OpenAIResponsesResponse
	if err := json.Unmarshal(raw, &upstreamResp); err != nil {
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp, err := protocol.OpenAIResponsesResponseToCanonical(upstreamResp)
	if err != nil {
		writeAnthropicError(w, http.StatusBadGateway, "api_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}
	canonicalResp.ID = "msg_" + result.RequestID
	canonicalResp.Model = model
	writeConvertedHeaders(w, result)
	writeJSON(w, http.StatusOK, protocol.CanonicalToAnthropicMessagesResponse(canonicalResp))
	s.recordProviderResultUsage(user, model, result, http.StatusOK, tokenUsageFromCanonical(canonicalResp.Usage))
}

func streamCanonicalAsOpenAIChat(
	w http.ResponseWriter,
	id string,
	created int64,
	model string,
	includeUsage bool,
	read func(func(protocol.CanonicalStreamEvent)) (protocol.CanonicalUsage, error),
) TokenUsage {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	roleWritten := false
	failed := false
	usage, readErr := read(func(event protocol.CanonicalStreamEvent) {
		if event.Type == protocol.CanonicalStreamError {
			if event.Error != nil {
				writeSSEJSON(w, flusher, "", map[string]any{
					"error": map[string]any{
						"message": event.Error.Message,
						"type":    "server_error",
						"code":    event.Error.Code,
					},
				})
			}
			failed = true
			return
		}
		if failed {
			return
		}
		if !roleWritten && event.Type != protocol.CanonicalStreamResponseStart && event.Type != protocol.CanonicalStreamUsage {
			writeOpenAIStreamChunk(w, flusher, protocol.OpenAIChatRoleStreamChunk(id, created, model))
			roleWritten = true
		}
		if event.Type == protocol.CanonicalStreamMessageStart && roleWritten {
			return
		}
		if chunk, ok := protocol.CanonicalToOpenAIStreamChunk(event, id, created, model, includeUsage); ok {
			writeOpenAIStreamChunk(w, flusher, chunk)
		}
	})
	if readErr != nil {
		event := canonicalStreamReadError(readErr)
		if event.Error != nil {
			writeSSEJSON(w, flusher, "", map[string]any{
				"error": map[string]any{
					"message": event.Error.Message,
					"type":    "server_error",
					"code":    event.Error.Code,
				},
			})
		}
		failed = true
	}
	if failed {
		return tokenUsageFromCanonical(usage)
	}
	if !roleWritten {
		writeOpenAIStreamChunk(w, flusher, protocol.OpenAIChatRoleStreamChunk(id, created, model))
	}
	io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	return tokenUsageFromCanonical(usage)
}

func isEventStream(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
}

func writeConvertedHeaders(w http.ResponseWriter, result ProviderAttemptResult) {
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
}
