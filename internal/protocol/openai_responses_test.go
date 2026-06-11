package protocol

import (
	"encoding/json"
	"testing"
)

func TestOpenAIResponsesToCanonicalRequest(t *testing.T) {
	maxTokens := 128
	temperature := 0.2
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model:           "gpt-5",
		Stream:          true,
		Instructions:    "be brief",
		MaxOutputTokens: &maxTokens,
		Temperature:     &temperature,
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":[
				{"type":"input_text","text":"hello"},
				{"type":"input_image","image_url":"data:image/png;base64,aW1hZ2U="}
			]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"hello\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"world"}
		]`),
		Tools:      json.RawMessage(`[{"type":"function","function":{"name":"lookup","description":"Lookup data","parameters":{"type":"object"}}}]`),
		ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"lookup"}}`),
		Reasoning:  &OpenAIReasoning{Effort: "high"},
		Text: &OpenAIResponseTextConfig{Format: &OpenAIResponseTextFormat{
			Type:   "json_schema",
			Schema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-5" || !req.Stream || req.MaxOutputTokens == nil || *req.MaxOutputTokens != 128 {
		t.Fatalf("basic fields = %+v", req)
	}
	if len(req.Messages) != 4 || req.Messages[0].Role != "system" || req.Messages[1].Parts[1].Type != "image" {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[2].Role != "assistant" || req.Messages[2].Parts[0].Name != "lookup" || string(req.Messages[2].Parts[0].Arguments) != `{"q":"hello"}` {
		t.Fatalf("tool call = %+v", req.Messages[2])
	}
	if req.Messages[3].Role != "tool" || string(req.Messages[3].Parts[0].Response) != `"world"` {
		t.Fatalf("tool response = %+v", req.Messages[3])
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", req.Tools)
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || req.ToolChoice.AllowedFunctionNames[0] != "lookup" {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
	if req.ThinkingLevel == nil || *req.ThinkingLevel != "high" {
		t.Fatalf("thinking = %+v", req.ThinkingLevel)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.MimeType != "application/json" || string(req.ResponseFormat.Schema) != `{"type":"object"}` {
		t.Fatalf("response format = %+v", req.ResponseFormat)
	}
}

func TestCanonicalToOpenAIResponsesRequest(t *testing.T) {
	level := "medium"
	req, err := CanonicalToOpenAIResponsesRequest(ChatRequest{
		Model:         "gpt-5",
		Stream:        true,
		ThinkingLevel: &level,
		ResponseFormat: &ChatResponseFormat{
			MimeType: "application/json",
			Schema:   json.RawMessage(`{"type":"object"}`),
		},
		Messages: []ChatMessage{
			{Role: "system", Parts: []ChatPart{{Type: "text", Text: "be brief"}}},
			{Role: "user", Parts: []ChatPart{
				{Type: "text", Text: "hello"},
				{Type: "image", MimeType: "image/png", Data: "aW1hZ2U="},
			}},
			{Role: "assistant", Parts: []ChatPart{{
				Type:       "tool_call",
				ToolCallID: "call_1",
				Name:       "lookup",
				Arguments:  json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "tool", Parts: []ChatPart{{
				Type:       "tool_response",
				ToolCallID: "call_1",
				Response:   json.RawMessage(`"world"`),
			}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Instructions != "be brief" || req.Reasoning == nil || req.Reasoning.Effort != "medium" {
		t.Fatalf("request = %+v", req)
	}
	if req.Text == nil || req.Text.Format == nil || req.Text.Format.Type != "json_schema" {
		t.Fatalf("text format = %+v", req.Text)
	}
	var items []OpenAIResponseInputItem
	if err := json.Unmarshal(req.Input, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Type != "message" || items[1].Type != "function_call" || items[2].Type != "function_call_output" {
		t.Fatalf("items = %+v", items)
	}
	if items[1].CallID != "call_1" || items[1].Name != "lookup" || items[1].Arguments != `{"q":"hello"}` {
		t.Fatalf("function call = %+v", items[1])
	}
	if items[2].Output != `"world"` {
		t.Fatalf("function output = %q", items[2].Output)
	}
	var content []OpenAIResponseContentPart
	if err := json.Unmarshal(items[0].Content, &content); err != nil {
		t.Fatal(err)
	}
	if len(content) != 2 || content[1].ImageURL != "data:image/png;base64,aW1hZ2U=" {
		t.Fatalf("content = %+v", content)
	}
}

func TestOpenAIResponsesResponseConversions(t *testing.T) {
	canonical := OpenAIResponsesResponseToCanonical(OpenAIResponsesResponse{
		ID:        "resp_1",
		CreatedAt: 123,
		Status:    "completed",
		Model:     "gpt-5",
		Output: []OpenAIResponseOutputItem{
			{Type: "message", Role: "assistant", Content: []OpenAIResponseOutputPart{{Type: "output_text", Text: "hello"}}},
			{Type: "function_call", CallID: "call_1", Name: "lookup", Arguments: `{"q":"hello"}`},
		},
		Usage: &OpenAIResponsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
			InputTokensDetails: OpenAIResponsesInputTokensDetails{
				CachedTokens: 3,
			},
			OutputTokensDetails: OpenAIResponsesOutputTokensDetails{
				ReasoningTokens: 2,
			},
		},
	})
	if canonical.ID != "resp_1" || canonical.Text != "hello" || canonical.FinishReason != "tool_calls" {
		t.Fatalf("canonical = %+v", canonical)
	}
	if len(canonical.ToolCalls) != 1 || canonical.ToolCalls[0].Name != "lookup" || canonical.ToolCalls[0].Arguments != `{"q":"hello"}` {
		t.Fatalf("tool calls = %+v", canonical.ToolCalls)
	}
	if canonical.Usage.PromptTokens != 10 || canonical.Usage.CachedTokens != 3 || canonical.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage = %+v", canonical.Usage)
	}

	resp := CanonicalToOpenAIResponsesResponse(ChatResponse{
		ID:           "resp_2",
		Created:      456,
		Model:        "gpt-5",
		Role:         "assistant",
		Text:         "world",
		FinishReason: "stop",
		ToolCalls: []ChatToolCall{{
			ID:        "call_2",
			Name:      "lookup",
			Arguments: `{"q":"world"}`,
		}},
		Usage: ChatUsage{
			PromptTokens:     20,
			CompletionTokens: 8,
			TotalTokens:      28,
			CachedTokens:     4,
			ReasoningTokens:  3,
		},
	})
	if resp.Status != "completed" || len(resp.Output) != 2 || resp.Output[1].Type != "function_call" {
		t.Fatalf("responses response = %+v", resp)
	}
	if resp.Usage == nil || resp.Usage.InputTokensDetails.CachedTokens != 4 || resp.Usage.OutputTokensDetails.ReasoningTokens != 3 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}
