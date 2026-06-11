package buzzhive

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) proxy(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, originalModel string) {
	if isAutoModel(originalModel) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auto model routing has been removed"})
		return
	}

	model := originalModel
	targets, err := s.resolveRouteTargets(model)
	if err != nil {
		if errors.Is(err, errModelRouteNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "model route not found",
				"model": model,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	actionModel, action, ok := parseGeminiModelAction(r.URL.Path)
	if ok && actionModel != "" {
		model = actionModel
	}
	targets = geminiTargets(targets)
	if len(targets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected upstream does not support Gemini"})
		return
	}
	target := targets[0]
	if protocol.ShouldPassthrough(providerGemini, target.ProviderType) {
		s.proxyGeminiRaw(w, r, body, user, model, targets)
		return
	}
	if action != "generateContent" && action != "streamGenerateContent" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported Gemini action"})
		return
	}
	var req protocol.GeminiGenerateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	canonicalReq, err := protocol.GeminiGenerateToCanonicalRequest(req, model, action == "streamGenerateContent")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	switch target.ProviderType {
	case providerOpenAI:
		s.proxyGeminiViaOpenAIChat(w, r, canonicalReq, user, model, targets)
	case providerOpenAIResponses:
		s.proxyGeminiViaOpenAIResponses(w, r, canonicalReq, user, model, targets)
	case providerAnthropic:
		s.proxyGeminiViaAnthropic(w, r, canonicalReq, user, model, targets)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected upstream does not support Gemini"})
	}
}

func geminiTargets(targets []RouteTarget) []RouteTarget {
	for _, protocol := range []string{providerGemini, providerOpenAI, providerOpenAIResponses, providerAnthropic} {
		out := routeTargetsByProtocol(targets, protocol)
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (s *Server) proxyGeminiRaw(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, model string, targets []RouteTarget) {
	maxAttempts := s.cfg.Retry.MaxAttempts

	result := s.doProviderTargetLoop(r.Context(), user, model, targets, func(target RouteTarget) ProviderRequest {
		return ProviderRequest{
			ProviderName:    target.ProviderName,
			InboundProtocol: "gemini",
			Method:          r.Method,
			Path:            r.URL.Path,
			RawQuery:        r.URL.RawQuery,
			Headers:         r.Header,
			Body:            body,
			RequestedModel:  model,
			Model:           target.UpstreamModel,
		}
	})
	if result.OK {
		defer result.Response.Body.Close()
		s.recordProviderResultUsage(user, model, result, result.Response.StatusCode)
		copyResponseHeaders(w.Header(), result.Response.Header)
		setCORS(w.Header())
		w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
		w.Header().Set("X-Proxy-Key", result.Key.Name)
		w.WriteHeader(result.Response.StatusCode)
		_ = copyResponseBody(w, result.Response.Body)
		return
	}

	lastStatus := http.StatusTooManyRequests
	var lastErrBody []byte
	if result.Response != nil {
		lastStatus = result.Response.StatusCode
		lastErrBody = drain(result.Response.Body, 64*1024)
	}
	s.recordProviderResultUsage(user, model, result, lastStatus)
	reason := "all keys failed"
	if result.Attempts >= maxAttempts {
		reason = "retry max_attempts reached"
		result.Chain = append(result.Chain, fmt.Sprintf("max-attempts:%d", maxAttempts))
	}
	w.Header().Set("X-Proxy-Debug", strings.Join(result.Chain, " -> "))
	writeJSON(w, lastStatus, map[string]any{
		"error":        reason,
		"attempts":     result.Attempts,
		"max_attempts": maxAttempts,
		"chain":        result.Chain,
		"body":         string(lastErrBody),
	})
}

func isAutoModel(model string) bool {
	return model == "auto" || model == "gemini-auto"
}

func parseGeminiModelAction(path string) (string, string, bool) {
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest, err := url.PathUnescape(strings.TrimPrefix(path, prefix))
	if err != nil {
		return "", "", false
	}
	model, action, ok := strings.Cut(rest, ":")
	if !ok || strings.TrimSpace(model) == "" || strings.TrimSpace(action) == "" {
		return "", "", false
	}
	return model, action, true
}
