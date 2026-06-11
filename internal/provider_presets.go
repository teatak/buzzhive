package buzzhive

type ProviderPreset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Protocols   []string `json:"protocols"`
	BaseURL     string   `json:"base_url"`
	Description string   `json:"description"`
}

func providerPresets() []ProviderPreset {
	return []ProviderPreset{
		{ID: "gemini", Name: "Google Gemini", Protocols: []string{"gemini"}, BaseURL: "https://generativelanguage.googleapis.com", Description: "Google Gemini native API."},
		{ID: "openai", Name: "OpenAI", Protocols: []string{"openai", "openai-responses"}, BaseURL: "https://api.openai.com/v1", Description: "OpenAI official API."},
		{ID: "anthropic", Name: "Anthropic Claude", Protocols: []string{"anthropic"}, BaseURL: "https://api.anthropic.com", Description: "Anthropic native Messages API."},
		{ID: "mimo", Name: "Mimo", Protocols: []string{"openai"}, BaseURL: "https://api.xiaomimimo.com/v1", Description: "Xiaomi Mimo standard OpenAI-compatible endpoint."},
		{ID: "mimo-plan", Name: "Mimo Plan", Protocols: []string{"openai"}, BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Description: "Xiaomi Mimo subscription endpoint."},
		{ID: "deepseek", Name: "DeepSeek", Protocols: []string{"openai"}, BaseURL: "https://api.deepseek.com", Description: "DeepSeek OpenAI-compatible endpoint."},
		{ID: "qwen", Name: "Qwen", Protocols: []string{"openai"}, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Description: "Alibaba Bailian Qwen OpenAI-compatible endpoint."},
		{ID: "moonshot", Name: "Kimi", Protocols: []string{"openai"}, BaseURL: "https://api.moonshot.cn/v1", Description: "Moonshot Kimi OpenAI-compatible endpoint."},
		{ID: "zhipu", Name: "GLM", Protocols: []string{"openai"}, BaseURL: "https://open.bigmodel.cn/api/paas/v4", Description: "Zhipu GLM OpenAI-compatible endpoint."},
		{ID: "openrouter", Name: "OpenRouter", Protocols: []string{"openai"}, BaseURL: "https://openrouter.ai/api/v1", Description: "OpenRouter OpenAI-compatible router."},
	}
}

func findProviderPreset(id string) (ProviderPreset, bool) {
	for _, preset := range providerPresets() {
		if preset.ID == id {
			return preset, true
		}
	}
	return ProviderPreset{}, false
}

func (p ProviderPreset) Provider() ProviderRecord {
	endpoints := make([]ProviderEndpoint, 0, len(p.Protocols))
	for _, protocol := range p.Protocols {
		endpoints = append(endpoints, ProviderEndpoint{
			Protocol: protocol,
			BaseURL:  p.BaseURL,
			Enabled:  true,
		})
	}
	return ProviderRecord{
		Name:      p.Name,
		PresetID:  p.ID,
		Endpoints: endpoints,
		Enabled:   true,
	}
}
