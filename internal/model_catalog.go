package buzzhive

import (
	"encoding/json"
	"io"
	"slices"
	"strings"
)

// ModelMetadata is a draft imported into the model configuration when a route is
// saved. Omitted capabilities are unknown; false is an explicit upstream value.
type ModelMetadata struct {
	ContextWindow   int64           `json:"context_window,omitempty"`
	MaxInputTokens  int64           `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int64           `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
}

type upstreamModel struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	ModelMetadata
}

type modelArchitecture struct {
	InputModalities  []string `json:"input_modalities,omitempty"`
	OutputModalities []string `json:"output_modalities,omitempty"`
}

type modelTopProvider struct {
	ContextLength       int64 `json:"context_length,omitempty"`
	MaxCompletionTokens int64 `json:"max_completion_tokens,omitempty"`
}

// OpenRouter's discovery fields are also used by BuzzHive's public directory.
type catalogModel struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name,omitempty"`
	Description         string             `json:"description,omitempty"`
	ContextLength       int64              `json:"context_length,omitempty"`
	CostMultiplier      *float64           `json:"cost_multiplier,omitempty"`
	Architecture        *modelArchitecture `json:"architecture,omitempty"`
	SupportedParameters *[]string          `json:"supported_parameters,omitempty"`
	TopProvider         *modelTopProvider  `json:"top_provider,omitempty"`
}

func decodeUpstreamModels(body io.Reader, protocol string) ([]upstreamModel, error) {
	var payload struct {
		Data []struct {
			catalogModel
			DisplayName    string `json:"display_name"`
			MaxInputTokens int64  `json:"max_input_tokens"`
			MaxTokens      int64  `json:"max_tokens"`
			Capabilities   struct {
				ImageInput struct {
					Supported *bool `json:"supported"`
				} `json:"image_input"`
				Thinking struct {
					Supported *bool `json:"supported"`
				} `json:"thinking"`
				StructuredOutputs struct {
					Supported *bool `json:"supported"`
				} `json:"structured_outputs"`
			} `json:"capabilities"`
		} `json:"data"`
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			InputTokenLimit            int64    `json:"inputTokenLimit"`
			OutputTokenLimit           int64    `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			Thinking                   *bool    `json:"thinking"`
		} `json:"models"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]upstreamModel, 0)
	if protocol == providerGemini {
		for _, m := range payload.Models {
			if !slices.Contains(m.SupportedGenerationMethods, "generateContent") {
				continue
			}
			candidate := upstreamModel{ID: strings.TrimPrefix(m.Name, "models/"), Name: m.DisplayName,
				ModelMetadata: ModelMetadata{ContextWindow: m.InputTokenLimit, MaxInputTokens: m.InputTokenLimit, MaxOutputTokens: m.OutputTokenLimit, Capabilities: map[string]bool{}}}
			if m.Thinking != nil {
				candidate.Capabilities["reasoning"] = *m.Thinking
			}
			models = append(models, candidate)
		}
	} else {
		for _, m := range payload.Data {
			candidate := upstreamModel{ID: m.ID, Name: m.Name, ModelMetadata: ModelMetadata{ContextWindow: m.ContextLength, Capabilities: map[string]bool{}}}
			if protocol == providerAnthropic {
				candidate.Name = m.DisplayName
				candidate.ContextWindow = m.MaxInputTokens
				candidate.MaxInputTokens = m.MaxInputTokens
				candidate.MaxOutputTokens = m.MaxTokens
				for key, value := range map[string]*bool{"vision": m.Capabilities.ImageInput.Supported, "reasoning": m.Capabilities.Thinking.Supported, "json_schema": m.Capabilities.StructuredOutputs.Supported} {
					if value != nil {
						candidate.Capabilities[key] = *value
					}
				}
			} else {
				if m.TopProvider != nil {
					candidate.MaxOutputTokens = m.TopProvider.MaxCompletionTokens
				}
				if m.Architecture != nil && m.Architecture.InputModalities != nil {
					candidate.Capabilities["vision"] = slices.Contains(m.Architecture.InputModalities, "image")
					candidate.Capabilities["audio_input"] = slices.Contains(m.Architecture.InputModalities, "audio")
				}
				if m.SupportedParameters != nil {
					candidate.Capabilities["tools"] = slices.Contains(*m.SupportedParameters, "tools")
					candidate.Capabilities["reasoning"] = slices.Contains(*m.SupportedParameters, "reasoning")
					candidate.Capabilities["json_schema"] = slices.Contains(*m.SupportedParameters, "structured_outputs")
				}
			}
			models = append(models, candidate)
		}
	}
	result := make([]upstreamModel, 0, len(models))
	seen := map[string]bool{}
	for _, m := range models {
		m.ID = strings.TrimSpace(m.ID)
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		// Exact preset IDs supplement ID-only endpoints such as DeepSeek; never guess
		// a hosted model's limits from a family name or a stripped provider prefix.
		if preset, ok := findModelPreset(m.ID); ok {
			if m.ContextWindow <= 0 {
				m.ContextWindow = preset.ContextWindow
			}
			if m.MaxInputTokens <= 0 {
				m.MaxInputTokens = preset.MaxInputTokens
			}
			if m.MaxOutputTokens <= 0 {
				m.MaxOutputTokens = preset.MaxOutputTokens
			}
			if m.Name == "" {
				m.Name = preset.DisplayName
			}
			for key, value := range savedModelCapabilities(preset.Capabilities) {
				if _, known := m.Capabilities[key]; !known {
					m.Capabilities[key] = value
				}
			}
		}
		m.ContextWindow = max(0, m.ContextWindow)
		m.MaxInputTokens = max(0, m.MaxInputTokens)
		m.MaxOutputTokens = max(0, m.MaxOutputTokens)
		result = append(result, m)
	}
	return result, nil
}

func savedModelCapabilities(value string) map[string]bool {
	var raw map[string]any
	result := map[string]bool{}
	if json.Unmarshal([]byte(value), &raw) == nil {
		for key, value := range raw {
			if enabled, ok := value.(bool); ok {
				result[key] = enabled
			}
		}
	}
	return result
}
