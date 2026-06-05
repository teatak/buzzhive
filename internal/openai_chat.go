package buzzhive

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

var errUnsupportedOpenAIContent = errors.New("only text and data URL image chat content are supported in this version")

type openAIChatRequest struct {
	Model           string          `json:"model"`
	Messages        []openAIMessage `json:"messages"`
	Stream          bool            `json:"stream"`
	Tools           json.RawMessage `json:"tools,omitempty"`
	ToolChoice      json.RawMessage `json:"tool_choice,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	MaxTokens       *int            `json:"max_tokens,omitempty"`
	MaxOutputTokens *int            `json:"max_completion_tokens,omitempty"`
	Stop            any             `json:"stop,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index        int                `json:"index"`
	Message      *openAIMessageOut  `json:"message,omitempty"`
	Delta        *openAIStreamDelta `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason"`
}

type openAIMessageOut struct {
	Role      string           `json:"role"`
	Content   *string          `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIStreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	Index    *int                   `json:"index,omitempty"`
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type geminiGenerateRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Tools             []geminiTool            `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *geminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (s *Server) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req openAIChatRequest
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
	target := targets[0]
	if isOpenAIProviderType(target.ProviderType) {
		s.proxyOpenAIRaw(w, r, body, user, req.Model, targets)
		return
	}

	canonicalReq, err := openAIToCanonicalChatRequest(req)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	s.applyToolSignatures(&canonicalReq)
	geminiReq, err := canonicalToGeminiGenerateRequest(canonicalReq)
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
		s.proxyOpenAIChatStream(w, r, geminiBody, user, req.Model, targets)
		return
	}
	s.proxyOpenAIChat(w, r, geminiBody, user, req.Model, targets)
}

func openAIStopSequences(value any) []string {
	switch stop := value.(type) {
	case string:
		return []string{stop}
	case []any:
		out := make([]string, 0, len(stop))
		for _, item := range stop {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func openAIMessageParts(raw json.RawMessage) ([]canonicalPart, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []canonicalPart{{Type: "text", Text: text}}, nil
	}
	var parts []openAIContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, errUnsupportedOpenAIContent
	}
	out := make([]canonicalPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text":
			out = append(out, canonicalPart{Type: "text", Text: part.Text})
		case "image_url":
			mimeType, data, err := parseOpenAIImageDataURL(part.ImageURL.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, canonicalPart{Type: "image", MimeType: mimeType, Data: data})
		default:
			return nil, errUnsupportedOpenAIContent
		}
	}
	return out, nil
}

type openAIContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

func parseOpenAIImageDataURL(value string) (string, string, error) {
	const prefix = "data:"
	if !strings.HasPrefix(value, prefix) {
		return "", "", errUnsupportedOpenAIContent
	}
	metaAndData := strings.SplitN(strings.TrimPrefix(value, prefix), ",", 2)
	if len(metaAndData) != 2 {
		return "", "", errUnsupportedOpenAIContent
	}
	meta := metaAndData[0]
	data := metaAndData[1]
	if !strings.HasSuffix(meta, ";base64") || data == "" {
		return "", "", errUnsupportedOpenAIContent
	}
	mimeType := strings.TrimSuffix(meta, ";base64")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return "", "", errUnsupportedOpenAIContent
	}
	return mimeType, data, nil
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
	var geminiResp geminiGenerateResponse
	if err := json.Unmarshal(raw, &geminiResp); err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "server_error", err.Error())
		s.recordProviderResultUsage(user, model, result, http.StatusBadGateway)
		return
	}

	canonicalResp := geminiToCanonicalChatResponse(geminiResp, model, result.RequestID, startedAt)
	if canonicalResp.FinishReason == "length" {
		logOpenAIDiagnostic(result, model, openAIDiagnosticRequest{}, http.StatusOK, "length", &canonicalResp.Usage.CompletionTokens)
	}
	s.rememberToolSignatures(canonicalResp.ToolCalls)
	out := canonicalToOpenAIChatResponse(canonicalResp)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", key.Name)
	writeJSON(w, http.StatusOK, out)
	s.recordProviderResultUsage(user, model, result, http.StatusOK)
}

func (s *Server) proxyOpenAIRaw(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget) {
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
	if !reqDiag.Stream && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		raw := drain(resp.Body, 8*1024*1024)
		logOpenAIDiagnosticResponse(result, model, reqDiag, resp.StatusCode, raw)
		resp.Body = io.NopCloser(bytes.NewReader(raw))
	}
	copyResponseHeaders(w.Header(), resp.Header)
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	w.Header().Set("X-Proxy-Key", result.Key.Name)
	w.WriteHeader(resp.StatusCode)
	_ = copyResponseBody(w, resp.Body)
	s.recordProviderResultUsage(user, model, result, resp.StatusCode)
}

func (s *Server) proxyOpenAIChatStream(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget) {
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
	writeOpenAIStreamChunk(w, flusher, openAIChatResponse{
		ID:      "chatcmpl-" + result.RequestID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []openAIChoice{{Index: 0, Delta: &openAIStreamDelta{Role: "assistant"}}},
	})

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var geminiResp geminiGenerateResponse
		if err := json.Unmarshal([]byte(payload), &geminiResp); err != nil {
			continue
		}
		event := geminiToCanonicalStreamEvent(geminiResp, result.RequestID)
		s.rememberToolSignatures(event.ToolCalls)
		if event.FinishReason == "length" {
			logOpenAIDiagnostic(result, model, openAIDiagnosticRequest{Stream: true}, http.StatusOK, "length", nil)
		}
		if event.Text != "" || len(event.ToolCalls) > 0 || event.FinishReason != "" {
			writeOpenAIStreamChunk(w, flusher, canonicalToOpenAIStreamChunk(event, "chatcmpl-"+result.RequestID, created, model))
		}
	}
	io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	s.recordProviderResultUsage(user, model, result, http.StatusOK)
}

func isOpenAIProviderType(providerType string) bool {
	switch strings.ToLower(providerType) {
	case "openai", "openai-compatible":
		return true
	default:
		return false
	}
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

func writeOpenAIStreamChunk(w io.Writer, flusher http.Flusher, chunk openAIChatResponse) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher != nil {
		flusher.Flush()
	}
}

func geminiResponseText(resp geminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func geminiFinishReason(resp geminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	return resp.Candidates[0].FinishReason
}

func openAIFinishReason(reason string) string {
	switch reason {
	case "", "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "content_filter"
	default:
		return strings.ToLower(reason)
	}
}
