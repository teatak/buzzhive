package protocol

import (
	"strings"
	"testing"
)

func TestCanonicalUsageIsZero(t *testing.T) {
	if !(CanonicalUsage{}).IsZero() {
		t.Fatal("zero usage was not recognized")
	}

	fields := []CanonicalUsage{
		{PromptTokens: 1},
		{CompletionTokens: 1},
		{TotalTokens: 1},
		{CachedTokens: 1},
		{ReasoningTokens: 1},
	}
	for _, usage := range fields {
		if usage.IsZero() {
			t.Fatalf("non-zero usage was not recognized: %+v", usage)
		}
	}
}

func TestCanonicalReasoningRequestPolicies(t *testing.T) {
	maxTokens := 256
	t.Run("OpenAI Chat rejects budget", func(t *testing.T) {
		budget := 1024
		_, err := CanonicalToOpenAIChatRequest(CanonicalRequest{
			Reasoning: &CanonicalReasoning{BudgetTokens: &budget},
		})
		if err == nil || !strings.Contains(err.Error(), "budget_tokens") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("Responses maps include thoughts to summary", func(t *testing.T) {
		include := true
		got, err := CanonicalToOpenAIResponsesRequest(CanonicalRequest{
			Reasoning: &CanonicalReasoning{Effort: "HIGH", IncludeThoughts: &include},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Reasoning == nil || got.Reasoning.Effort != "high" || got.Reasoning.Summary != "auto" {
			t.Fatalf("reasoning = %+v", got.Reasoning)
		}
	})

	t.Run("Anthropic rejects unsupported effort", func(t *testing.T) {
		_, err := CanonicalToAnthropicMessagesRequest(CanonicalRequest{
			MaxOutputTokens: &maxTokens,
			Reasoning:       &CanonicalReasoning{Effort: "minimal"},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot be represented") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("Anthropic maps automatic summary", func(t *testing.T) {
		got, err := CanonicalToAnthropicMessagesRequest(CanonicalRequest{
			MaxOutputTokens: &maxTokens,
			Reasoning:       &CanonicalReasoning{Effort: "HIGH", Summary: "auto"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Thinking == nil || got.Thinking.Display != "summarized" ||
			got.OutputConfig == nil || got.OutputConfig.Effort != "high" {
			t.Fatalf("request = %+v", got)
		}
	})

	t.Run("Gemini rejects budget", func(t *testing.T) {
		budget := 1024
		_, err := CanonicalToGeminiGenerateRequest(CanonicalRequest{
			Reasoning: &CanonicalReasoning{BudgetTokens: &budget},
		})
		if err == nil || !strings.Contains(err.Error(), "budget_tokens") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("Gemini normalizes level and maps summary", func(t *testing.T) {
		got, err := CanonicalToGeminiGenerateRequest(CanonicalRequest{
			Reasoning: &CanonicalReasoning{Effort: "high", Summary: "auto"},
			Messages: []CanonicalMessage{{
				Role:  "user",
				Parts: []CanonicalPart{{Type: "text", Text: "hi"}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		thinking := got.GenerationConfig.ThinkingConfig
		if thinking == nil || thinking.ThinkingLevel != "HIGH" ||
			thinking.IncludeThoughts == nil || !*thinking.IncludeThoughts {
			t.Fatalf("thinking = %+v", thinking)
		}
	})
}
