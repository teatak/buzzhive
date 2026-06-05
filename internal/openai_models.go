package buzzhive

import (
	"net/http"
)

type openAIModelsResponse struct {
	Object string              `json:"object"`
	Data   []openAIModelObject `json:"data"`
}

type openAIModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
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
		data = append(data, openAIModelObject{
			ID:      model.Name,
			Object:  "model",
			Created: 0,
			OwnedBy: "buzzhive",
		})
	}
	writeJSON(w, http.StatusOK, openAIModelsResponse{Object: "list", Data: data})
}
