package buzzhive

import "encoding/json"

type ModelPreset struct {
	ID              string `json:"id"`
	Family          string `json:"family"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	ContextWindow   int64  `json:"context_window"`
	MaxInputTokens  int64  `json:"max_input_tokens"`
	MaxOutputTokens int64  `json:"max_output_tokens"`
	Capabilities    string `json:"capabilities"`
	SelectionPolicy string `json:"selection_policy"`
}

func modelPresets() []ModelPreset {
	capText := modelPresetCapabilities(false, false)
	capVision := modelPresetCapabilities(true, false)
	capMultimodal := modelPresetCapabilities(true, true)

	return []ModelPreset{
		{ID: "gemini-3.5-flash", Family: "Gemini", Name: "gemini-3.5-flash", DisplayName: "Gemini 3.5 Flash", Description: "Google fast multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 65536, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "gemini-3.1-pro-preview", Family: "Gemini", Name: "gemini-3.1-pro-preview", DisplayName: "Gemini 3.1 Pro Preview", Description: "Google pro multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 65536, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "gemini-3.1-flash-lite", Family: "Gemini", Name: "gemini-3.1-flash-lite", DisplayName: "Gemini 3.1 Flash Lite", Description: "Google lite multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 65536, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},

		{ID: "gpt-5.5", Family: "OpenAI", Name: "gpt-5.5", DisplayName: "GPT 5.5", Description: "OpenAI multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 128000, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "gpt-5.4", Family: "OpenAI", Name: "gpt-5.4", DisplayName: "GPT 5.4", Description: "OpenAI multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 128000, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "gpt-5.4-mini", Family: "OpenAI", Name: "gpt-5.4-mini", DisplayName: "GPT 5.4 Mini", Description: "OpenAI mini multimodal model.", ContextWindow: 400000, MaxInputTokens: 400000, MaxOutputTokens: 128000, Capabilities: capVision, SelectionPolicy: "round_robin"},

		{ID: "claude-sonnet-4-6", Family: "Anthropic", Name: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Description: "Anthropic balanced model.", ContextWindow: 1000000, MaxInputTokens: 1000000, MaxOutputTokens: 128000, Capabilities: capVision, SelectionPolicy: "round_robin"},
		{ID: "claude-opus-4-8", Family: "Anthropic", Name: "claude-opus-4-8", DisplayName: "Claude Opus 4.8", Description: "Anthropic advanced model.", ContextWindow: 1000000, MaxInputTokens: 1000000, MaxOutputTokens: 128000, Capabilities: capVision, SelectionPolicy: "round_robin"},
		{ID: "claude-haiku-4-5", Family: "Anthropic", Name: "claude-haiku-4-5", DisplayName: "Claude Haiku 4.5", Description: "Anthropic fast model.", ContextWindow: 200000, MaxInputTokens: 200000, MaxOutputTokens: 64000, Capabilities: capVision, SelectionPolicy: "round_robin"},

		{ID: "mimo-v2.5", Family: "Mimo", Name: "mimo-v2.5", DisplayName: "MiMo-V2.5", Description: "Xiaomi multimodal model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 131072, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "mimo-v2.5-pro", Family: "Mimo", Name: "mimo-v2.5-pro", DisplayName: "MiMo-V2.5-Pro", Description: "Xiaomi text model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 131072, Capabilities: capText, SelectionPolicy: "round_robin"},

		{ID: "deepseek-v4-flash", Family: "DeepSeek", Name: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", Description: "DeepSeek fast model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 384000, Capabilities: capText, SelectionPolicy: "round_robin"},
		{ID: "deepseek-v4-pro", Family: "DeepSeek", Name: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", Description: "DeepSeek pro model.", ContextWindow: 1050000, MaxInputTokens: 1050000, MaxOutputTokens: 384000, Capabilities: capText, SelectionPolicy: "round_robin"},

		{ID: "qwen3.6-flash", Family: "Qwen", Name: "qwen3.6-flash", DisplayName: "Qwen 3.6 Flash", Description: "Qwen fast multimodal model.", ContextWindow: 1000000, MaxInputTokens: 1000000, MaxOutputTokens: 65536, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "qwen3.7-max", Family: "Qwen", Name: "qwen3.7-max", DisplayName: "Qwen 3.7 Max", Description: "Qwen text model.", ContextWindow: 1000000, MaxInputTokens: 1000000, MaxOutputTokens: 65536, Capabilities: capText, SelectionPolicy: "round_robin"},
		{ID: "qwen3.6-plus", Family: "Qwen", Name: "qwen3.6-plus", DisplayName: "Qwen 3.6 Plus", Description: "Qwen multimodal model.", ContextWindow: 1000000, MaxInputTokens: 1000000, MaxOutputTokens: 65536, Capabilities: capMultimodal, SelectionPolicy: "round_robin"},
		{ID: "qwen3-max", Family: "Qwen", Name: "qwen3-max", DisplayName: "Qwen 3 Max", Description: "Qwen text model.", ContextWindow: 262144, MaxInputTokens: 262144, MaxOutputTokens: 65536, Capabilities: capText, SelectionPolicy: "round_robin"},
		{ID: "kimi-k2.6", Family: "Moonshot", Name: "kimi-k2.6", DisplayName: "Kimi K2.6", Description: "Kimi vision model.", ContextWindow: 262100, MaxInputTokens: 262100, MaxOutputTokens: 262100, Capabilities: capVision, SelectionPolicy: "round_robin"},
		{ID: "kimi-k2.5", Family: "Moonshot", Name: "kimi-k2.5", DisplayName: "Kimi K2.5", Description: "Kimi vision model.", ContextWindow: 262100, MaxInputTokens: 262100, MaxOutputTokens: 262100, Capabilities: capVision, SelectionPolicy: "round_robin"},

		{ID: "glm-5", Family: "Zhipu", Name: "glm-5", DisplayName: "GLM 5", Description: "Zhipu text model.", ContextWindow: 202800, MaxInputTokens: 202800, MaxOutputTokens: 202800, Capabilities: capText, SelectionPolicy: "round_robin"},
		{ID: "glm-5.1", Family: "Zhipu", Name: "glm-5.1", DisplayName: "GLM 5.1", Description: "Zhipu text model.", ContextWindow: 202800, MaxInputTokens: 202800, MaxOutputTokens: 202800, Capabilities: capText, SelectionPolicy: "round_robin"},
	}
}

func findModelPreset(id string) (ModelPreset, bool) {
	for _, preset := range modelPresets() {
		if preset.ID == id {
			return preset, true
		}
	}
	return ModelPreset{}, false
}

func (p ModelPreset) Model() Model {
	return Model{
		Name:            p.Name,
		DisplayName:     p.DisplayName,
		Description:     p.Description,
		ContextWindow:   p.ContextWindow,
		MaxInputTokens:  p.MaxInputTokens,
		MaxOutputTokens: p.MaxOutputTokens,
		Capabilities:    p.Capabilities,
		SelectionPolicy: p.SelectionPolicy,
		Enabled:         true,
	}
}

func modelPresetCapabilities(vision, audioInput bool) string {
	data, err := json.MarshalIndent(map[string]any{
		"stream":      true,
		"tools":       true,
		"vision":      vision,
		"json_schema": true,
		"reasoning":   true,
		"audio_input": audioInput,
	}, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
