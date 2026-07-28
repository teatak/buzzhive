package buzzhive

import "testing"

func TestMimoPresetsDefineProtocolSpecificEndpoints(t *testing.T) {
	tests := []struct {
		id               string
		openAIBaseURL    string
		anthropicBaseURL string
	}{
		{
			id:               "mimo",
			openAIBaseURL:    "https://api.xiaomimimo.com/v1",
			anthropicBaseURL: "https://api.xiaomimimo.com/anthropic",
		},
		{
			id:               "mimo-plan",
			openAIBaseURL:    "https://token-plan-cn.xiaomimimo.com/v1",
			anthropicBaseURL: "https://token-plan-cn.xiaomimimo.com/anthropic",
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			preset, ok := findProviderPreset(test.id)
			if !ok {
				t.Fatalf("preset %q not found", test.id)
			}
			provider := preset.Provider()
			got := make(map[string]string, len(provider.Endpoints))
			for _, endpoint := range provider.Endpoints {
				got[endpoint.Protocol] = endpoint.BaseURL
			}
			if got[providerOpenAI] != test.openAIBaseURL {
				t.Fatalf("OpenAI endpoint = %q, want %q", got[providerOpenAI], test.openAIBaseURL)
			}
			if got[providerAnthropic] != test.anthropicBaseURL {
				t.Fatalf("Anthropic endpoint = %q, want %q", got[providerAnthropic], test.anthropicBaseURL)
			}
		})
	}
}
