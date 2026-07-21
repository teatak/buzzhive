package buzzhive

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/teatak/cart/v3"
)

func (s *Server) handleProviderUpstreamModelsAdmin(c *cart.Context) error {
	providerID, err := c.ParamInt64("id")
	if err != nil {
		c.JSON(http.StatusBadRequest, cart.H{"error": "invalid provider id"})
		return nil
	}

	provider, err := s.store.Provider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, cart.H{"error": "provider not found"})
		return nil
	}
	if len(provider.Endpoints) == 0 {
		c.JSON(http.StatusBadRequest, cart.H{"error": "provider has no endpoints"})
		return nil
	}

	keys, err := s.store.ProviderKeys(providerID, true)
	if err != nil || len(keys) == 0 {
		c.JSON(http.StatusBadRequest, cart.H{"error": "provider has no active keys"})
		return nil
	}

	endpoint := provider.Endpoints[0]
	var activeKey ProviderKey
	found := false
	for _, k := range keys {
		if k.Enabled && k.DisabledStatus == 0 {
			activeKey = k
			found = true
			break
		}
	}
	if !found {
		activeKey = keys[0] // fallback
	}

	baseURL := endpoint.BaseURL
	url := providerRequestPath(baseURL, "/v1/models")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, cart.H{"error": "failed to create request"})
		return nil
	}

	if endpoint.Protocol == "anthropic" {
		req.Header.Set("x-api-key", activeKey.Secret)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+activeKey.Secret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, cart.H{"error": "failed to fetch upstream models", "details": err.Error()})
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.JSON(http.StatusBadGateway, cart.H{"error": "upstream returned error", "status": resp.StatusCode})
		return nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusBadGateway, cart.H{"error": "failed to decode upstream response"})
		return nil
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	c.JSON(http.StatusOK, models)
	return nil
}
