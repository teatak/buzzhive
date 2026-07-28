package protocol

import (
	"encoding/json"
	"strings"
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
			{
				Role: "assistant",
				Content: []AnthropicContent{{
					Type:      "thinking",
					Thinking:  "considering",
					Signature: "sig",
				}},
			},
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
		OutputConfig: &AnthropicOutputConfig{Format: &AnthropicJSONOutputFormat{
			Type:   "json_schema",
			Schema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "claude" || req.MaxOutputTokens == nil || *req.MaxOutputTokens != 100 {
		t.Fatalf("basic fields = %+v", req)
	}
	if len(req.Messages) != 5 || req.Messages[0].Role != "system" || req.Messages[1].Parts[1].Type != "image" {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[2].Role != "assistant" || req.Messages[2].Parts[0].Type != "reasoning" || req.Messages[2].Parts[0].Signature != "sig" {
		t.Fatalf("reasoning = %+v", req.Messages[2])
	}
	if req.Messages[3].Role != "assistant" || req.Messages[3].Parts[0].Type != "tool_call" || req.Messages[3].Parts[0].ToolCallID != "toolu_1" {
		t.Fatalf("assistant = %+v", req.Messages[3])
	}
	if req.Messages[4].Role != "tool" ||
		req.Messages[4].Parts[0].ToolCallID != "toolu_1" ||
		req.Messages[4].Parts[0].Name != "lookup" {
		t.Fatalf("tool = %+v", req.Messages[4])
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", req.Tools)
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || req.ToolChoice.AllowedFunctionNames[0] != "lookup" {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
	if req.ResponseFormat == nil || string(req.ResponseFormat.Schema) != `{"type":"object"}` {
		t.Fatalf("response format = %+v", req.ResponseFormat)
	}
	geminiReq, err := CanonicalToGeminiGenerateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	functionResponse := geminiReq.Contents[len(geminiReq.Contents)-1].Parts[0].FunctionResponse
	if functionResponse == nil ||
		functionResponse.Name != "lookup" ||
		string(functionResponse.Response) != `{"result":"world"}` {
		t.Fatalf("Gemini function response = %+v", functionResponse)
	}
}

func TestCanonicalToAnthropicRequiresMaxOutputTokens(t *testing.T) {
	_, err := CanonicalToAnthropicMessagesRequest(CanonicalRequest{})
	if err == nil || !strings.Contains(err.Error(), "max_output_tokens") {
		t.Fatalf("error = %v", err)
	}

	zero := 0
	_, err = CanonicalToAnthropicMessagesRequest(CanonicalRequest{MaxOutputTokens: &zero})
	if err == nil || !strings.Contains(err.Error(), "greater than zero") {
		t.Fatalf("zero max_output_tokens error = %v", err)
	}
}

func TestCanonicalToAnthropicRejectsUnsupportedToolChoiceSubset(t *testing.T) {
	maxTokens := 100
	_, err := CanonicalToAnthropicMessagesRequest(CanonicalRequest{
		MaxOutputTokens: &maxTokens,
		Tools: []CanonicalTool{
			{Name: "lookup"},
			{Name: "search"},
		},
		ToolChoice: &CanonicalToolChoice{
			Mode:                 "ANY",
			AllowedFunctionNames: []string{"lookup", "search"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "function subset") {
		t.Fatalf("error = %v", err)
	}
}

func TestCanonicalToAnthropicMessagesRequest(t *testing.T) {
	maxTokens := 100
	includeThoughts := true
	req, err := CanonicalToAnthropicMessagesRequest(CanonicalRequest{
		Model:           "claude",
		MaxOutputTokens: &maxTokens,
		Reasoning: &CanonicalReasoning{
			Effort:          "high",
			IncludeThoughts: &includeThoughts,
		},
		ResponseFormat: &CanonicalResponseFormat{
			MimeType: "application/json",
			Schema:   json.RawMessage(`{"type":"object"}`),
		},
		Messages: []CanonicalMessage{
			{Role: "system", Parts: []CanonicalPart{{Type: "text", Text: "be brief"}}},
			{Role: "developer", Parts: []CanonicalPart{{Type: "text", Text: "use json"}}},
			{Role: "user", Parts: []CanonicalPart{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Parts: []CanonicalPart{{
				Type:      "reasoning",
				Text:      "considering",
				Signature: "sig",
			}, {
				Type:       "tool_call",
				ToolCallID: "toolu_1",
				Name:       "lookup",
				Arguments:  json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "tool", Parts: []CanonicalPart{{
				Type:       "tool_response",
				ToolCallID: "toolu_1",
				Response:   json.RawMessage(`"world"`),
			}}},
		},
		Tools: []CanonicalTool{{Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.System == nil || len(req.Messages) != 3 {
		t.Fatalf("request = %+v", req)
	}
	if req.Thinking == nil || req.Thinking.Type != "adaptive" || req.Thinking.Display != "summarized" || req.OutputConfig == nil || req.OutputConfig.Effort != "high" {
		t.Fatalf("reasoning config = thinking:%+v output:%+v", req.Thinking, req.OutputConfig)
	}
	if req.OutputConfig.Format == nil || req.OutputConfig.Format.Type != "json_schema" ||
		string(req.OutputConfig.Format.Schema) != `{"type":"object"}` {
		t.Fatalf("output format = %+v", req.OutputConfig.Format)
	}
	if req.Messages[1].Content[0].Type != "thinking" || req.Messages[1].Content[0].Signature != "sig" {
		t.Fatalf("thinking content = %+v", req.Messages[1].Content)
	}
	system, ok := req.System.([]AnthropicContent)
	if !ok || len(system) != 2 || system[0].Text != "be brief" || system[1].Text != "use json" {
		t.Fatalf("system = %+v", req.System)
	}
	if req.Messages[1].Content[1].Type != "tool_use" || req.Messages[1].Content[1].ID != "toolu_1" {
		t.Fatalf("tool use = %+v", req.Messages[1])
	}
	if req.Messages[2].Content[0].Type != "tool_result" ||
		req.Messages[2].Content[0].ToolUseID != "toolu_1" ||
		req.Messages[2].Content[0].Content != "world" {
		t.Fatalf("tool result = %+v", req.Messages[2])
	}
}

func TestAnthropicMessagesResponseConversions(t *testing.T) {
	canonical, err := AnthropicMessagesResponseToCanonical(AnthropicMessagesResponse{
		ID:    "msg_1",
		Role:  "assistant",
		Model: "claude",
		Content: []AnthropicContent{
			{Type: "thinking", Thinking: "thinking", Signature: "sig"},
			{Type: "text", Text: "hello"},
		},
		Usage: AnthropicUsage{
			InputTokens:          10,
			OutputTokens:         5,
			CacheReadInputTokens: 3,
			OutputTokensDetails:  &AnthropicOutputTokensDetails{ThinkingTokens: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.ID != "msg_1" || canonical.Text != "hello" || canonical.Usage.PromptTokens != 13 ||
		canonical.Usage.TotalTokens != 18 || canonical.Usage.CachedTokens != 3 {
		t.Fatalf("canonical = %+v", canonical)
	}
	if canonical.Reasoning != "thinking" || canonical.Signature != "sig" || canonical.Usage.ReasoningTokens != 2 {
		t.Fatalf("canonical reasoning = %+v", canonical)
	}
	resp := CanonicalToAnthropicMessagesResponse(CanonicalResponse{
		ID:           "msg_2",
		Model:        "claude",
		Role:         "assistant",
		Reasoning:    "thinking",
		Signature:    "sig",
		FinishReason: "tool_calls",
		ToolCalls: []CanonicalToolCall{{
			ID:        "toolu_1",
			Name:      "lookup",
			Arguments: `{"q":"hello"}`,
		}},
	})
	if resp.StopReason != "tool_use" || len(resp.Content) != 2 || resp.Content[0].Type != "thinking" || resp.Content[1].Type != "tool_use" {
		t.Fatalf("anthropic response = %+v", resp)
	}
}

func TestAnthropicToolCallDoesNotOverrideLengthFinishReason(t *testing.T) {
	canonical, err := AnthropicMessagesResponseToCanonical(AnthropicMessagesResponse{
		StopReason: "max_tokens",
		Content: []AnthropicContent{{
			Type:  "tool_use",
			ID:    "call_1",
			Name:  "lookup",
			Input: json.RawMessage(`{"q":"hello"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.FinishReason != "length" {
		t.Fatalf("finish reason = %q", canonical.FinishReason)
	}
}

func TestAnthropicResponseRejectsUnsupportedContent(t *testing.T) {
	_, err := AnthropicMessagesResponseToCanonical(AnthropicMessagesResponse{
		Content: []AnthropicContent{{Type: "server_tool_use"}},
	})
	if err == nil || !strings.Contains(err.Error(), "server_tool_use") {
		t.Fatalf("error = %v", err)
	}
}
