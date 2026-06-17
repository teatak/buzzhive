package protocol

import (
	"encoding/json"
	"testing"
)

func TestAnthropicMessagesToCanonicalRequest(t *testing.T) {
	maxTokens := 100
	req, err := AnthropicMessagesToCanonicalRequest(AnthropicMessagesRequest{
		Model:     "claude",
		System:    "be brief",
		MaxTokens: &maxTokens,
		Messages: []AnthropicMessage{
			{Role: "user", Content: []AnthropicContent{
				{Type: "text", Text: "hello"},
				{Type: "image", Source: &AnthropicSource{Type: "base64", MediaType: "image/png", Data: "aW1hZ2U="}},
			}},
			{Role: "assistant", Content: []AnthropicContent{{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "lookup",
				Input: json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "user", Content: []AnthropicContent{{
				Type:      "tool_result",
				ToolUseID: "toolu_1",
				Content:   "world",
			}}},
		},
		Tools: []AnthropicTool{{
			Name:        "lookup",
			Description: "Lookup data",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		ToolChoice: &AnthropicToolChoice{Type: "tool", Name: "lookup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "claude" || req.MaxOutputTokens == nil || *req.MaxOutputTokens != 100 {
		t.Fatalf("basic fields = %+v", req)
	}
	if len(req.Messages) != 4 || req.Messages[0].Role != "system" || req.Messages[1].Parts[1].Type != "image" {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[2].Role != "assistant" || req.Messages[2].Parts[0].Type != "tool_call" || req.Messages[2].Parts[0].ToolCallID != "toolu_1" {
		t.Fatalf("assistant = %+v", req.Messages[2])
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].Parts[0].ToolCallID != "toolu_1" {
		t.Fatalf("tool = %+v", req.Messages[3])
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", req.Tools)
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || req.ToolChoice.AllowedFunctionNames[0] != "lookup" {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
}

func TestCanonicalToAnthropicMessagesRequest(t *testing.T) {
	maxTokens := 100
	req, err := CanonicalToAnthropicMessagesRequest(ChatRequest{
		Model:           "claude",
		MaxOutputTokens: &maxTokens,
		Messages: []ChatMessage{
			{Role: "system", Parts: []ChatPart{{Type: "text", Text: "be brief"}}},
			{Role: "developer", Parts: []ChatPart{{Type: "text", Text: "use json"}}},
			{Role: "user", Parts: []ChatPart{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Parts: []ChatPart{{
				Type:       "tool_call",
				ToolCallID: "toolu_1",
				Name:       "lookup",
				Arguments:  json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "tool", Parts: []ChatPart{{
				Type:       "tool_response",
				ToolCallID: "toolu_1",
				Response:   json.RawMessage(`"world"`),
			}}},
		},
		Tools: []ChatTool{{Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.System == nil || len(req.Messages) != 3 {
		t.Fatalf("request = %+v", req)
	}
	system, ok := req.System.([]AnthropicContent)
	if !ok || len(system) != 2 || system[0].Text != "be brief" || system[1].Text != "use json" {
		t.Fatalf("system = %+v", req.System)
	}
	if req.Messages[1].Content[0].Type != "tool_use" || req.Messages[1].Content[0].ID != "toolu_1" {
		t.Fatalf("tool use = %+v", req.Messages[1])
	}
	if req.Messages[2].Content[0].Type != "tool_result" || req.Messages[2].Content[0].ToolUseID != "toolu_1" {
		t.Fatalf("tool result = %+v", req.Messages[2])
	}
}

func TestAnthropicMessagesResponseConversions(t *testing.T) {
	canonical := AnthropicMessagesResponseToCanonical(AnthropicMessagesResponse{
		ID:      "msg_1",
		Role:    "assistant",
		Model:   "claude",
		Content: []AnthropicContent{{Type: "text", Text: "hello"}},
		Usage:   AnthropicUsage{InputTokens: 10, OutputTokens: 5, CacheReadInputTokens: 3},
	})
	if canonical.ID != "msg_1" || canonical.Text != "hello" || canonical.Usage.TotalTokens != 15 || canonical.Usage.CachedTokens != 3 {
		t.Fatalf("canonical = %+v", canonical)
	}
	resp := CanonicalToAnthropicMessagesResponse(ChatResponse{
		ID:           "msg_2",
		Model:        "claude",
		Role:         "assistant",
		FinishReason: "tool_calls",
		ToolCalls: []ChatToolCall{{
			ID:        "toolu_1",
			Name:      "lookup",
			Arguments: `{"q":"hello"}`,
		}},
	})
	if resp.StopReason != "tool_use" || len(resp.Content) != 1 || resp.Content[0].Type != "tool_use" {
		t.Fatalf("anthropic response = %+v", resp)
	}
}
