package protocol

import "testing"

func TestOpenAIChatResponseToCanonical(t *testing.T) {
	finish := "tool_calls"
	content := "hello"
	resp := OpenAIChatResponse{
		ID:      "chatcmpl-1",
		Created: 123,
		Model:   "model-a",
		Choices: []OpenAIChoice{{
			Message: &OpenAIMessageOut{
				Role:    "assistant",
				Content: &content,
				ToolCalls: []OpenAIToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "lookup",
						Arguments: `{"q":"hello"}`,
					},
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
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "lookup" || got.ToolCalls[0].Arguments != `{"q":"hello"}` {
		t.Fatalf("tool calls = %+v", got.ToolCalls)
	}
	if got.Usage.PromptTokens != 10 || got.Usage.CachedTokens != 3 || got.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestCanonicalToGeminiGenerateResponse(t *testing.T) {
	resp := CanonicalToGeminiGenerateResponse(ChatResponse{
		Text:         "hello",
		FinishReason: "length",
		ToolCalls: []ChatToolCall{{
			Name:      "lookup",
			Arguments: `{"q":"hello"}`,
			Signature: "sig",
		}},
		Usage: ChatUsage{
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
	if len(parts) != 2 || parts[0].Text != "hello" || parts[1].FunctionCall == nil || parts[1].FunctionCall.Name != "lookup" || parts[1].ThoughtSignature != "sig" {
		t.Fatalf("parts = %+v", parts)
	}
	if resp.UsageMetadata.PromptTokenCount != 10 || resp.UsageMetadata.CachedContentTokenCount != 3 || resp.UsageMetadata.ThoughtsTokenCount != 2 {
		t.Fatalf("usage = %+v", resp.UsageMetadata)
	}
}

func TestOpenAIChatStreamChunkToCanonical(t *testing.T) {
	finish := "stop"
	chunk := OpenAIChatResponse{
		Choices: []OpenAIChoice{{
			Delta: &OpenAIStreamDelta{
				Content: "delta",
			},
			FinishReason: &finish,
		}},
	}
	got := OpenAIChatStreamChunkToCanonical(chunk)
	if got.Text != "delta" || got.FinishReason != "stop" {
		t.Fatalf("stream event = %+v", got)
	}
}
