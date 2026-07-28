package protocol

import (
	"encoding/json"
	"testing"
)

func TestOpenAIChatResponseToCanonical(t *testing.T) {
	finish := "tool_calls"
	content := "hello"
	resp := OpenAIChatResponse{
		ID:      "chatcmpl-1",
		Created: 123,
		Model:   "model-a",
		Choices: []OpenAIChoice{{
			Message: &OpenAIMessageOut{
				Role:             "assistant",
				Content:          &content,
				ReasoningContent: "thinking",
				ToolCalls: []OpenAIToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "lookup",
						Arguments: `{"q":"hello"}`,
					},
					ExtraContent: openAIToolCallExtraContent("thought-sig"),
				}},
			},
			FinishReason: &finish,
		}},
		Usage: &OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			PromptTokensDetails: &OpenAIPromptTokensDetails{
				CachedTokens: 3,
			},
			CompletionTokensDetails: &OpenAICompletionTokensDetails{
				ReasoningTokens: 2,
			},
		},
	}
	got := OpenAIChatResponseToCanonical(resp)
	if got.ID != "chatcmpl-1" || got.Role != "assistant" || got.Text != "hello" || got.FinishReason != "tool_calls" {
		t.Fatalf("response = %+v", got)
	}
	if got.Reasoning != "thinking" {
		t.Fatalf("reasoning = %q", got.Reasoning)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "lookup" || got.ToolCalls[0].Arguments != `{"q":"hello"}` {
		t.Fatalf("tool calls = %+v", got.ToolCalls)
	}
	if got.ToolCalls[0].Signature != "thought-sig" {
		t.Fatalf("tool signature = %q", got.ToolCalls[0].Signature)
	}
	if got.Usage.PromptTokens != 10 || got.Usage.CachedTokens != 3 || got.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestCanonicalToGeminiGenerateResponse(t *testing.T) {
	resp := CanonicalToGeminiGenerateResponse(CanonicalResponse{
		Text:         "hello",
		Reasoning:    "thinking",
		Signature:    "reasoning-sig",
		FinishReason: "length",
		ToolCalls: []CanonicalToolCall{{
			Name:      "lookup",
			Arguments: `{"q":"hello"}`,
			Signature: "sig",
		}},
		Usage: CanonicalUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			CachedTokens:     3,
			ReasoningTokens:  2,
		},
	})
	if len(resp.Candidates) != 1 || resp.Candidates[0].FinishReason != "MAX_TOKENS" {
		t.Fatalf("candidates = %+v", resp.Candidates)
	}
	parts := resp.Candidates[0].Content.Parts
	if len(parts) != 3 ||
		!parts[0].Thought ||
		parts[0].Text != "thinking" ||
		parts[0].ThoughtSignature != "reasoning-sig" ||
		parts[1].Text != "hello" ||
		parts[2].FunctionCall == nil ||
		parts[2].FunctionCall.Name != "lookup" ||
		parts[2].ThoughtSignature != "sig" {
		t.Fatalf("parts = %+v", parts)
	}
	if resp.UsageMetadata.PromptTokenCount != 10 || resp.UsageMetadata.CachedContentTokenCount != 3 || resp.UsageMetadata.ThoughtsTokenCount != 2 {
		t.Fatalf("usage = %+v", resp.UsageMetadata)
	}
}

func TestGeminiSafetyBlockToCanonicalRefusal(t *testing.T) {
	got := GeminiToCanonicalResponse(GeminiGenerateResponse{
		PromptFeedback: &GeminiPromptFeedback{
			BlockReason:        "SAFETY",
			BlockReasonMessage: "blocked by safety policy",
		},
	}, "gemini", "response-id", 1, "request-id")
	if got.FinishReason != "content_filter" || got.Refusal != "blocked by safety policy" {
		t.Fatalf("canonical = %+v", got)
	}
	events := GeminiToCanonicalStreamEvents(GeminiGenerateResponse{
		PromptFeedback: &GeminiPromptFeedback{BlockReason: "SAFETY"},
	}, "request-id", 0)
	if len(events) != 2 ||
		events[0].Type != CanonicalStreamRefusalDelta ||
		events[1].FinishReason != "content_filter" {
		t.Fatalf("events = %+v", events)
	}
}

func TestGeminiToolCallDoesNotOverrideSafetyFinishReason(t *testing.T) {
	got := GeminiToCanonicalResponse(GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			FinishReason: "SAFETY",
			Content: GeminiContent{Parts: []GeminiPart{{
				FunctionCall: &GeminiFunctionCall{Name: "lookup", Args: json.RawMessage(`{}`)},
			}}},
		}},
	}, "gemini", "response-id", 1, "request-id")
	if got.FinishReason != "content_filter" {
		t.Fatalf("finish reason = %q", got.FinishReason)
	}

	events := GeminiToCanonicalStreamEvents(GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			FinishReason: "SAFETY",
			Content: GeminiContent{Parts: []GeminiPart{{
				FunctionCall: &GeminiFunctionCall{Name: "lookup", Args: json.RawMessage(`{}`)},
			}}},
		}},
	}, "request-id", 0)
	if events[len(events)-1].FinishReason != "content_filter" {
		t.Fatalf("stream finish reason = %q", events[len(events)-1].FinishReason)
	}
}

func TestOpenAIChatStreamChunkToCanonical(t *testing.T) {
	finish := "stop"
	chunk := OpenAIChatResponse{
		Choices: []OpenAIChoice{{
			Delta: &OpenAIStreamDelta{
				Content:          "delta",
				ReasoningContent: "think",
			},
			FinishReason: &finish,
		}},
	}
	got := OpenAIChatStreamChunkToCanonical(chunk)
	if len(got) != 3 ||
		got[0].Type != CanonicalStreamTextDelta ||
		got[0].Delta != "delta" ||
		got[1].Type != CanonicalStreamReasoningDelta ||
		got[1].Delta != "think" ||
		got[2].Type != CanonicalStreamResponseDone ||
		got[2].FinishReason != "stop" {
		t.Fatalf("stream events = %+v", got)
	}
}

func TestDeepSeekUsageToCanonical(t *testing.T) {
	got := OpenAIChatResponseToCanonical(OpenAIChatResponse{
		Usage: &OpenAIUsage{
			PromptTokens:          10,
			CompletionTokens:      5,
			TotalTokens:           15,
			PromptCacheHitTokens:  7,
			PromptCacheMissTokens: 3,
		},
	})
	if got.Usage.CachedTokens != 7 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestOpenAIChatToolStreamChunkToCanonical(t *testing.T) {
	index := 2
	chunk := OpenAIChatResponse{
		Choices: []OpenAIChoice{{
			Delta: &OpenAIStreamDelta{
				ToolCalls: []OpenAIToolCall{{
					Index: &index,
					ID:    "call_1",
					Type:  "function",
					Function: OpenAIToolCallFunction{
						Name:      "lookup",
						Arguments: `{"q":`,
					},
					ExtraContent: openAIToolCallExtraContent("thought-sig"),
				}},
			},
		}},
	}

	got := OpenAIChatStreamChunkToCanonical(chunk)
	if len(got) != 2 ||
		got[0].Type != CanonicalStreamToolCallStart ||
		got[0].Index != 2 ||
		got[0].CallID != "call_1" ||
		got[0].Name != "lookup" ||
		got[0].Signature != "thought-sig" ||
		got[1].Type != CanonicalStreamToolArgumentsDelta ||
		got[1].Delta != `{"q":` {
		t.Fatalf("stream events = %+v", got)
	}
}

func TestGeminiToolStreamToCanonical(t *testing.T) {
	events := GeminiToCanonicalStreamEvents(GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			Content: GeminiContent{
				Parts: []GeminiPart{{
					FunctionCall: &GeminiFunctionCall{
						ID:   "call_upstream",
						Name: "lookup",
						Args: []byte(`{"q":"hello"}`),
					},
					ThoughtSignature: "sig",
				}},
			},
			FinishReason: "STOP",
		}},
	}, "req", 3)

	if len(events) != 4 ||
		events[0].Type != CanonicalStreamToolCallStart ||
		events[0].Index != 3 ||
		events[0].CallID != "call_upstream" ||
		events[1].Type != CanonicalStreamToolArgumentsDelta ||
		events[2].Type != CanonicalStreamToolCallDone ||
		events[2].Arguments != `{"q":"hello"}` ||
		events[2].Signature != "sig" ||
		events[3].Type != CanonicalStreamResponseDone ||
		events[3].FinishReason != "tool_calls" {
		t.Fatalf("stream events = %+v", events)
	}
}
