package buzzhive

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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
