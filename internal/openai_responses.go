package buzzhive

import (
	"encoding/json"
	"errors"
	"net/http"
)

type openAIResponsesRequest struct {
	Model string `json:"model"`
}

func (s *Server) handleOpenAIResponses(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req openAIResponsesRequest
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
	supportedTargets := targets[:0]
	for _, target := range targets {
		if target.SupportsResponses && isOpenAIProviderType(target.ProviderType) {
			supportedTargets = append(supportedTargets, target)
		}
	}
	if len(supportedTargets) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "unsupported_endpoint", "selected upstream does not support OpenAI Responses passthrough")
		return
	}
	s.proxyOpenAIRaw(w, r, body, user, req.Model, supportedTargets)
}
