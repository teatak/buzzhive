package buzzhive

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicModelMetadataKeepsUnknownFieldsAbsent(t *testing.T) {
	for _, caps := range []string{"", "{}", "null", "invalid", `{"vision":"false"}`} {
		m := publicModelMetadata(Model{Name: "unknown", Capabilities: caps})
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"context_length", "max_input_tokens", "max_output_tokens", "capabilities"} {
			if strings.Contains(string(b), field) {
				t.Fatalf("invented %s in %s", field, b)
			}
		}
	}
}
