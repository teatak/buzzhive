package buzzhive

type ProviderPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	BaseURL     string `json:"base_url"`
	Description string `json:"description"`
}

func providerPresets() []ProviderPreset {
	return []ProviderPreset{
		{ID: "gemini", Name: "Google Gemini", Type: providerGemini, BaseURL: "https://generativelanguage.googleapis.com", Description: "Google Gemini native API."},
		{ID: "openai", Name: "OpenAI", Type: providerOpenAI, BaseURL: "https://api.openai.com/v1", Description: "OpenAI official API."},
		{ID: "anthropic", Name: "Anthropic Claude", Type: providerAnthropic, BaseURL: "https://api.anthropic.com", Description: "Anthropic native Messages API."},
		{ID: "mimo", Name: "Mimo", Type: providerOpenAICompatible, BaseURL: "https://api.xiaomimimo.com/v1", Description: "Xiaomi Mimo standard OpenAI-compatible endpoint."},
		{ID: "mimo-plan", Name: "Mimo Plan", Type: providerOpenAICompatible, BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Description: "Xiaomi Mimo subscription endpoint."},
		{ID: "deepseek", Name: "DeepSeek", Type: providerOpenAICompatible, BaseURL: "https://api.deepseek.com", Description: "DeepSeek OpenAI-compatible endpoint."},
		{ID: "qwen", Name: "Qwen", Type: providerOpenAICompatible, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Description: "Alibaba Bailian Qwen OpenAI-compatible endpoint."},
		{ID: "moonshot", Name: "Kimi", Type: providerOpenAICompatible, BaseURL: "https://api.moonshot.cn/v1", Description: "Moonshot Kimi OpenAI-compatible endpoint."},
		{ID: "zhipu", Name: "GLM", Type: providerOpenAICompatible, BaseURL: "https://open.bigmodel.cn/api/paas/v4", Description: "Zhipu GLM OpenAI-compatible endpoint."},
		{ID: "openrouter", Name: "OpenRouter", Type: providerOpenAICompatible, BaseURL: "https://openrouter.ai/api/v1", Description: "OpenRouter OpenAI-compatible router."},
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
	return ProviderRecord{
		Name:     p.Name,
		Type:     p.Type,
		PresetID: p.ID,
		BaseURL:  p.BaseURL,
		Enabled:  true,
	}
}
