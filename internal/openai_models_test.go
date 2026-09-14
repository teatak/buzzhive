package buzzhive

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPublicModelMetadataKeepsUnknownFieldsAbsent(t *testing.T) {
	for _, caps := range []string{"", "{}", "null", "invalid", `{"vision":"false"}`, `{"vision":true,"tools":false}`} {
		m := publicModelMetadata(Model{Name: "unknown", Capabilities: caps})
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"context_length", "max_input_tokens", "max_output_tokens", "capabilities", "architecture", "supported_parameters", "top_provider", "pricing"} {
			if strings.Contains(string(b), field) {
				t.Fatalf("invented %s in %s", field, b)
			}
		}
	}
}

func TestPublicModelMetadataKnownDisabledCapabilities(t *testing.T) {
	m := publicModelMetadata(Model{Name: "text", Capabilities: `{"vision":false,"audio_input":false,"tools":false,"reasoning":false,"json_schema":false}`})
	if m.Architecture == nil || !reflect.DeepEqual(m.Architecture.InputModalities, []string{"text"}) || m.SupportedParameters == nil || len(*m.SupportedParameters) != 0 {
		t.Fatalf("explicit disabled lost: %+v", m)
	}
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"supported_parameters":[]`) {
		t.Fatalf("empty list not preserved: %s", b)
	}
}
