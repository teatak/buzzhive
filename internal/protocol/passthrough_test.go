package protocol

import "testing"

func TestShouldPassthrough(t *testing.T) {
	if !ShouldPassthrough(" openai ", "OpenAI") {
		t.Fatal("expected matching protocols to passthrough")
	}
	if ShouldPassthrough(OpenAIChat, Gemini) {
		t.Fatal("expected different protocols to use conversion")
	}
}
