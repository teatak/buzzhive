package buzzhive

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req protocol.OpenAIChatRequest
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
	if err := validateOpenAIChatParameterSupport(req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
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
	targets = openAIChatTargets(targets)
	if len(targets) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support OpenAI Chat Completions")
		return
	}
	target := targets[0]
	if protocol.ShouldPassthrough(providerOpenAI, target.ProviderType) {
		s.proxyRaw(w, r, body, user, req.Model, targets)
		return
	}

	canonicalReq, err := protocol.OpenAIChatToCanonical(req)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	thinkingLevel, err := geminiThinkingLevelForOpenAIReasoningEffort(req.ReasoningEffort, target.UpstreamModel)
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

	if req.Stream {
		s.proxyOpenAIChatStream(w, r, geminiBody, user, req.Model, targets, req.StreamOptions != nil && req.StreamOptions.IncludeUsage)
		return
	}
	s.proxyOpenAIChat(w, r, geminiBody, user, req.Model, targets)
}

func validateOpenAIChatParameterSupport(req protocol.OpenAIChatRequest) error {
	if req.N != nil {
		if *req.N < 1 {
			return errors.New("n must be at least 1")
		}
		if *req.N > 1 {
			return errors.New("n greater than 1 is not supported")
		}
	}
	if req.Logprobs != nil && *req.Logprobs {
		return errors.New("logprobs is not supported")
	}
	if req.TopLogprobs != nil {
		return errors.New("top_logprobs is not supported")
	}
	return nil
}

func openAIChatTargets(targets []RouteTarget) []RouteTarget {
	for _, protocol := range []string{providerOpenAI, providerGemini} {
		out := routeTargetsByProtocol(targets, protocol)
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func geminiThinkingLevelForOpenAIReasoningEffort(effort *string, model string) (*string, error) {
	if effort == nil || strings.TrimSpace(*effort) == "" {
		return nil, nil
	}
	if !strings.Contains(strings.ToLower(model), "gemini-3") {
		return nil, errors.New("reasoning_effort mapping is only supported for Gemini 3 models")
	}
	value := strings.ToLower(strings.TrimSpace(*effort))
	switch value {
	case "low", "high":
		level := strings.ToUpper(value)
		return &level, nil
	case "medium":
		level := "MEDIUM"
		return &level, nil
	case "minimal":
		if strings.Contains(strings.ToLower(model), "flash") {
			level := "MINIMAL"
			return &level, nil
		}
		level := "LOW"
		return &level, nil
	case "none":
		return nil, errors.New("reasoning_effort none is not supported for Gemini 3 models")
	case "xhigh":
		return nil, errors.New("reasoning_effort xhigh is not supported for Gemini models")
	default:
		return nil, fmt.Errorf("unsupported reasoning_effort %q", value)
	}
}

func (s *Server) proxyOpenAIChat(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget) {
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: "openai",
			Method:          http.MethodPost,
			Body:            body,
			RequestedModel:  model,
			Model:           target.UpstreamModel,
			Action:          "generateContent",
		}
	})
	if !result.OK {
		s.recordProviderResultUsage(user, model, result, providerResultStatus(result.Response))
		writeOpenAIRetryError(w, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	key := result.Key
	startedAt := result.StartedAt
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeOpenAIUpstreamError(w, resp.StatusCode, raw)
		s.recordProviderResultUsage(user, model, result, resp.StatusCode)
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

	canonicalResp := protocol.GeminiToCanonicalChatResponse(geminiResp, model, "chatcmpl-"+result.RequestID, startedAt.Unix(), result.RequestID)
	if canonicalResp.FinishReason == "length" {
		logOpenAIDiagnostic(result, model, openAIDiagnosticRequest{}, http.StatusOK, "length", &canonicalResp.Usage.CompletionTokens)
	}
	s.rememberToolSignatures(canonicalResp.ToolCalls)
	out := protocol.CanonicalToOpenAIChatResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
}

func (s *Server) proxyRaw(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget) {
	reqDiag := openAIDiagnosticRequestFromBody(body)
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: "openai",
			Method:          r.Method,
			Path:            r.URL.Path,
			RawQuery:        r.URL.RawQuery,
			Headers:         r.Header,
			Body:            rewriteOpenAIModel(body, model, target.UpstreamModel),
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if !result.OK {
		s.recordProviderResultUsage(user, model, result, providerResultStatus(result.Response))
		logOpenAIDiagnostic(result, model, reqDiag, providerResultStatus(result.Response), "", nil)
		writeOpenAIRetryError(w, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		logOpenAIDiagnostic(result, model, reqDiag, resp.StatusCode, "", nil)
	}
	usage := TokenUsage{}
	if !reqDiag.Stream && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		raw := drain(resp.Body, 8*1024*1024)
		usage = tokenUsageFromOpenAIResponseBody(raw)
		logOpenAIDiagnosticResponse(result, model, reqDiag, resp.StatusCode, raw)
		resp.Body = io.NopCloser(bytes.NewReader(raw))
	}
	copyResponseHeaders(w.Header(), resp.Header)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	w.WriteHeader(resp.StatusCode)
	if reqDiag.Stream && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		usage = copyOpenAIStreamResponseBody(w, resp.Body)
	} else {
		_ = copyResponseBody(w, resp.Body)
	}
	s.recordProviderResultUsage(user, model, result, resp.StatusCode, usage)
}

func (s *Server) proxyOpenAIChatStream(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget, includeUsage bool) {
	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: "openai",
			Method:          http.MethodPost,
			Body:            body,
			RequestedModel:  model,
			Model:           target.UpstreamModel,
			Action:          "streamGenerateContent",
		}
	})
	if !result.OK {
		s.recordProviderResultUsage(user, model, result, providerResultStatus(result.Response))
		writeOpenAIRetryError(w, result.Response, result.Attempts, s.cfg.Retry.MaxAttempts, result.Chain)
		return
	}
	resp := result.Response
	key := result.Key
	startedAt := result.StartedAt
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw := drain(resp.Body, 64*1024)
		writeOpenAIUpstreamError(w, resp.StatusCode, raw)
		s.recordProviderResultUsage(user, model, result, resp.StatusCode)
		return
	}

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", key.Name)
	w.WriteHeader(http.StatusOK)

	created := startedAt.Unix()
	writeOpenAIStreamChunk(w, flusher, protocol.OpenAIChatRoleStreamChunk("chatcmpl-"+result.RequestID, created, model))

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	usage := TokenUsage{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var geminiResp protocol.GeminiGenerateResponse
		if err := json.Unmarshal([]byte(payload), &geminiResp); err != nil {
			continue
		}
		if chunkUsage := tokenUsageFromGeminiResponseBody([]byte(payload), geminiResp); !chunkUsage.IsZero() {
			usage = chunkUsage
		}
		event := protocol.GeminiToCanonicalStreamEvent(geminiResp, result.RequestID)
		s.rememberToolSignatures(event.ToolCalls)
		if event.FinishReason == "length" {
			logOpenAIDiagnostic(result, model, openAIDiagnosticRequest{Stream: true}, http.StatusOK, "length", nil)
		}
		if event.Text != "" || len(event.ToolCalls) > 0 || event.FinishReason != "" {
			writeOpenAIStreamChunk(w, flusher, protocol.CanonicalToOpenAIStreamChunk(event, "chatcmpl-"+result.RequestID, created, model, includeUsage))
		}
	}
	io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	s.recordProviderResultUsage(user, model, result, http.StatusOK, usage)
}

func copyOpenAIStreamResponseBody(w http.ResponseWriter, r io.Reader) TokenUsage {
	reader := bufio.NewReader(r)
	flusher, _ := w.(http.Flusher)
	usage := TokenUsage{}
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, writeErr := w.Write(line); writeErr != nil {
				return usage
			}
			if flusher != nil {
				flusher.Flush()
			}
			if chunkUsage := tokenUsageFromOpenAIStreamLine(line); !chunkUsage.IsZero() {
				usage = chunkUsage
			}
		}
		if err != nil {
			return usage
		}
	}
}

func tokenUsageFromOpenAIStreamLine(line []byte) TokenUsage {
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return TokenUsage{}
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return TokenUsage{}
	}
	return tokenUsageFromOpenAIResponseBody(payload)
}

func isOpenAIProviderType(providerType string) bool {
	t := strings.ToLower(providerType)
	return t == "openai" || t == "openai-responses"
}

func rewriteOpenAIModel(body []byte, publicModel, upstreamModel string) []byte {
	if upstreamModel == "" || upstreamModel == publicModel {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["model"] = upstreamModel
	nextBody, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return nextBody
}

type openAIDiagnosticRequest struct {
	MaxTokens           *int `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`
	Stream              bool `json:"stream,omitempty"`
}

func openAIDiagnosticRequestFromBody(body []byte) openAIDiagnosticRequest {
	var req openAIDiagnosticRequest
	_ = json.Unmarshal(body, &req)
	return req
}

func logOpenAIDiagnosticResponse(result ProviderAttemptResult, publicModel string, req openAIDiagnosticRequest, status int, raw []byte) {
	var resp struct {
		Choices []struct {
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &resp) != nil || len(resp.Choices) == 0 || resp.Choices[0].FinishReason == nil {
		return
	}
	if *resp.Choices[0].FinishReason != "length" {
		return
	}
	var completionTokens *int
	if resp.Usage != nil {
		completionTokens = &resp.Usage.CompletionTokens
	}
	logOpenAIDiagnostic(result, publicModel, req, status, "length", completionTokens)
}

func logOpenAIDiagnostic(result ProviderAttemptResult, publicModel string, req openAIDiagnosticRequest, status int, finishReason string, completionTokens *int) {
	if status < 400 && finishReason != "length" {
		return
	}
	log.Printf(
		"openai diagnostic request_id=%s model=%s provider=%s upstream_model=%s status=%d finish_reason=%s max_tokens=%s max_completion_tokens=%s completion_tokens=%s attempts=%d chain=%s",
		result.RequestID,
		publicModel,
		result.Target.ProviderName,
		result.Target.UpstreamModel,
		status,
		finishReason,
		intPtrString(req.MaxTokens),
		intPtrString(req.MaxCompletionTokens),
		intPtrString(completionTokens),
		result.Attempts,
		strings.Join(result.Chain, " -> "),
	)
}

func intPtrString(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func writeOpenAIRetryError(w http.ResponseWriter, resp *http.Response, attempts, maxAttempts int, chain []string) {
	status := http.StatusTooManyRequests
	raw := []byte{}
	if resp != nil {
		status = resp.StatusCode
		raw = drain(resp.Body, 64*1024)
	}
	w.Header().Set("X-Proxy-Debug", strings.Join(chain, " -> "))
	message := openAIUpstreamErrorMessage(status, raw)
	writeOpenAIError(w, status, "upstream_error", fmt.Sprintf("upstream failed after %d/%d attempts: %s", attempts, maxAttempts, message))
}

func writeOpenAIUpstreamError(w http.ResponseWriter, status int, raw []byte) {
	writeOpenAIError(w, status, "upstream_error", openAIUpstreamErrorMessage(status, raw))
}

func writeOpenAIError(w http.ResponseWriter, status int, code, message string) {
	errorType, errorCode := openAIErrorTypeAndCode(status, code)
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"code":    errorCode,
		},
	})
}

func openAIErrorTypeAndCode(status int, code string) (string, string) {
	if code != "" && code != "upstream_error" {
		switch code {
		case "method_not_allowed", "model_not_found":
			return "invalid_request_error", code
		default:
			return code, code
		}
	}

	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "invalid_request_error", "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error", "invalid_api_key"
	case http.StatusForbidden:
		return "permission_error", "permission_denied"
	case http.StatusNotFound:
		return "invalid_request_error", "not_found"
	case http.StatusRequestTimeout:
		return "timeout_error", "timeout"
	case http.StatusConflict:
		return "conflict_error", "conflict"
	case http.StatusTooManyRequests:
		return "rate_limit_error", "rate_limit_exceeded"
	default:
		if status >= 500 {
			return "server_error", "upstream_error"
		}
		return "upstream_error", "upstream_error"
	}
}

func openAIUpstreamErrorMessage(status int, raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		if statusText := http.StatusText(status); statusText != "" {
			return statusText
		}
		return "upstream error"
	}

	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && len(payload.Error) > 0 {
		var message string
		if json.Unmarshal(payload.Error, &message) == nil && strings.TrimSpace(message) != "" {
			return strings.TrimSpace(message)
		}

		var detail struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		}
		if json.Unmarshal(payload.Error, &detail) == nil {
			message = strings.TrimSpace(detail.Message)
			statusName := strings.TrimSpace(detail.Status)
			switch {
			case message != "" && statusName != "" && !strings.Contains(message, statusName):
				return message + " (" + statusName + ")"
			case message != "":
				return message
			case statusName != "":
				return statusName
			}
		}
	}

	return text
}

func writeOpenAIStreamChunk(w io.Writer, flusher http.Flusher, chunk protocol.OpenAIChatResponse) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher != nil {
		flusher.Flush()
	}
}
