package buzzhive

import (
	"encoding/json"
	"net/http"
	"strings"
)

type openAIModelsResponse struct {
	Object string              `json:"object"`
	Data   []openAIModelObject `json:"data"`
}

type openAIModelObject struct {
	ID              string          `json:"id"`
	Object          string          `json:"object"`
	Created         int64           `json:"created"`
	OwnedBy         string          `json:"owned_by"`
	Name            string          `json:"name,omitempty"`
	ContextLength   int64           `json:"context_length,omitempty"`
	MaxInputTokens  int64           `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int64           `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
}

func (s *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request, _ AuthToken) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.store == nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", "store is not configured")
		return
	}
	models, err := s.store.Models()
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	data := make([]openAIModelObject, 0, len(models))
	for _, model := range models {
		if !model.Enabled {
			continue
		}
		data = append(data, publicModelMetadata(model))
	}
	writeJSON(w, http.StatusOK, openAIModelsResponse{Object: "list", Data: data})
}

// Publish the saved model configuration, not preset or route-derived guesses.
// Unknown capabilities stay absent; explicitly disabled ones remain false.
func publicModelMetadata(model Model) openAIModelObject {
	var raw map[string]any
	capabilities := map[string]bool{}
	if json.Unmarshal([]byte(model.Capabilities), &raw) == nil {
		for key, value := range raw {
			if enabled, ok := value.(bool); ok {
				capabilities[key] = enabled
			}
		}
	}
	return openAIModelObject{
		ID: model.Name, Object: "model", OwnedBy: "buzzhive",
		Name:          strings.TrimSpace(model.DisplayName),
		ContextLength: model.ContextWindow, MaxInputTokens: model.MaxInputTokens,
		MaxOutputTokens: model.MaxOutputTokens, Capabilities: capabilities,
	}
}
