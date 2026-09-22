package buzzhive

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMimoPresets(t *testing.T) {
	for _, preset := range modelPresets() {
		if preset.Family == "Mimo" && (!strings.HasPrefix(preset.DisplayName, "Mimo ") || strings.Contains(preset.DisplayName, "MiMo")) {
			t.Errorf("inconsistent display name: %s", preset.DisplayName)
		}
	}
	for _, variant := range []string{"flash", "pro"} {
		preset, found := findModelPreset("mimo-v2.6-" + variant)
		if !found || preset.Family != "Mimo" || preset.ContextWindow != 1000000 || preset.MaxInputTokens != 1000000 || preset.MaxOutputTokens != 131072 {
			t.Fatalf("incomplete V2.6 preset: %+v", preset)
		}
		var capabilities map[string]bool
		if err := json.Unmarshal([]byte(preset.Capabilities), &capabilities); err != nil {
			t.Fatal(err)
		}
		for _, capability := range []string{"vision", "audio_input", "tools", "reasoning"} {
			if !capabilities[capability] {
				t.Errorf("%s missing %s", preset.ID, capability)
			}
		}
	}
}
