package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAIResponsesToCanonicalRequest(t *testing.T) {
	maxTokens := 128
	temperature := 0.2
	strict := true
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
		Tools:      json.RawMessage(`[{"type":"function","name":"lookup","description":"Lookup data","parameters":{"type":"object"},"strict":true}]`),
		ToolChoice: json.RawMessage(`{"type":"function","name":"lookup"}`),
		Reasoning:  &OpenAIReasoning{Effort: "high"},
		Text: &OpenAIResponseTextConfig{Format: &OpenAIResponseTextFormat{
			Type:   "json_schema",
			Name:   "answer",
			Schema: json.RawMessage(`{"type":"object"}`),
			Strict: &strict,
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
	if req.Messages[3].Role != "tool" ||
		req.Messages[3].Parts[0].Name != "lookup" ||
		string(req.Messages[3].Parts[0].Response) != `"world"` {
		t.Fatalf("tool response = %+v", req.Messages[3])
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", req.Tools)
	}
	if req.Tools[0].Strict == nil || !*req.Tools[0].Strict {
		t.Fatalf("tool strict = %+v", req.Tools[0])
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || req.ToolChoice.AllowedFunctionNames[0] != "lookup" {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
	if req.Reasoning == nil || req.Reasoning.Effort != "high" {
		t.Fatalf("reasoning = %+v", req.Reasoning)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.MimeType != "application/json" || string(req.ResponseFormat.Schema) != `{"type":"object"}` {
		t.Fatalf("response format = %+v", req.ResponseFormat)
	}
	if req.ResponseFormat.Name != "answer" || req.ResponseFormat.Strict == nil || !*req.ResponseFormat.Strict {
		t.Fatalf("response format metadata = %+v", req.ResponseFormat)
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

func TestCanonicalToOpenAIResponsesRequest(t *testing.T) {
	level := "medium"
	strict := true
	req, err := CanonicalToOpenAIResponsesRequest(CanonicalRequest{
		Model:     "gpt-5",
		Stream:    true,
		Reasoning: &CanonicalReasoning{Effort: level},
		ResponseFormat: &CanonicalResponseFormat{
			MimeType: "application/json",
			Schema:   json.RawMessage(`{"type":"object"}`),
		},
		Tools: []CanonicalTool{{
			Name:        "lookup",
			Description: "Lookup data",
			Parameters:  json.RawMessage(`{"type":"object"}`),
			Strict:      &strict,
		}},
		ToolChoice: &CanonicalToolChoice{Mode: "ANY", AllowedFunctionNames: []string{"lookup"}},
		Messages: []CanonicalMessage{
			{Role: "system", Parts: []CanonicalPart{{Type: "text", Text: "be brief"}}},
			{Role: "user", Parts: []CanonicalPart{
				{Type: "text", Text: "hello"},
				{Type: "image", MimeType: "image/png", Data: "aW1hZ2U="},
			}},
			{Role: "assistant", Parts: []CanonicalPart{{
				Type:       "tool_call",
				ToolCallID: "call_1",
				Name:       "lookup",
				Arguments:  json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "tool", Parts: []CanonicalPart{{
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
	var tools []OpenAIResponsesFunctionTool
	if err := json.Unmarshal(req.Tools, &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "lookup" || tools[0].Strict == nil || !*tools[0].Strict {
		t.Fatalf("Responses tools = %+v", tools)
	}
	var toolChoice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.ToolChoice, &toolChoice); err != nil {
		t.Fatal(err)
	}
	if toolChoice.Type != "function" || toolChoice.Name != "lookup" {
		t.Fatalf("Responses tool choice = %+v", toolChoice)
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

func TestOpenAIResponsesIncompleteRefusalAndFailure(t *testing.T) {
	canonical, err := OpenAIResponsesResponseToCanonical(OpenAIResponsesResponse{
		Status:            "incomplete",
		IncompleteDetails: &OpenAIResponsesIncompleteDetails{Reason: "content_filter"},
		Output: []OpenAIResponseOutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []OpenAIResponseOutputPart{{
					Type:    "refusal",
					Refusal: "cannot comply",
				}},
			},
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "lookup",
				Arguments: `{}`,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.FinishReason != "content_filter" || canonical.Refusal != "cannot comply" {
		t.Fatalf("canonical = %+v", canonical)
	}

	out := CanonicalToOpenAIResponsesResponse(canonical)
	if out.Status != "incomplete" ||
		out.IncompleteDetails == nil ||
		out.IncompleteDetails.Reason != "content_filter" ||
		len(out.Output) != 2 ||
		out.Output[0].Content[0].Type != "refusal" {
		t.Fatalf("Responses response = %+v", out)
	}

	_, err = OpenAIResponsesResponseToCanonical(OpenAIResponsesResponse{
		Status: "failed",
		Error:  &OpenAIResponsesError{Code: "server_error", Message: "upstream failed"},
	})
	if err == nil || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenAIResponsesResponseConversions(t *testing.T) {
	canonical, err := OpenAIResponsesResponseToCanonical(OpenAIResponsesResponse{
		ID:        "resp_1",
		CreatedAt: 123,
		Status:    "completed",
		Model:     "gpt-5",
		Output: []OpenAIResponseOutputItem{
			{
				Type:             "reasoning",
				Summary:          []OpenAIResponseOutputPart{{Type: "summary_text", Text: "thinking"}},
				EncryptedContent: "encrypted",
			},
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
	if err != nil {
		t.Fatal(err)
	}
	if canonical.ID != "resp_1" || canonical.Text != "hello" || canonical.FinishReason != "tool_calls" {
		t.Fatalf("canonical = %+v", canonical)
	}
	if canonical.Reasoning != "thinking" || canonical.Signature != "encrypted" {
		t.Fatalf("canonical reasoning = %+v", canonical)
	}
	if len(canonical.ToolCalls) != 1 || canonical.ToolCalls[0].Name != "lookup" || canonical.ToolCalls[0].Arguments != `{"q":"hello"}` {
		t.Fatalf("tool calls = %+v", canonical.ToolCalls)
	}
	if canonical.Usage.PromptTokens != 10 || canonical.Usage.CachedTokens != 3 || canonical.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage = %+v", canonical.Usage)
	}

	resp := CanonicalToOpenAIResponsesResponse(CanonicalResponse{
		ID:           "resp_2",
		Created:      456,
		Model:        "gpt-5",
		Role:         "assistant",
		Text:         "world",
		Reasoning:    "thinking again",
		Signature:    "encrypted-2",
		FinishReason: "stop",
		ToolCalls: []CanonicalToolCall{{
			ID:        "call_2",
			Name:      "lookup",
			Arguments: `{"q":"world"}`,
		}},
		Usage: CanonicalUsage{
			PromptTokens:     20,
			CompletionTokens: 8,
			TotalTokens:      28,
			CachedTokens:     4,
			ReasoningTokens:  3,
		},
	})
	if resp.Status != "completed" || len(resp.Output) != 3 || resp.Output[0].Type != "reasoning" || resp.Output[2].Type != "function_call" {
		t.Fatalf("responses response = %+v", resp)
	}
	if resp.Output[0].Summary[0].Text != "thinking again" || resp.Output[0].EncryptedContent != "encrypted-2" {
		t.Fatalf("responses reasoning = %+v", resp.Output[0])
	}
	if resp.Usage == nil || resp.Usage.InputTokensDetails.CachedTokens != 4 || resp.Usage.OutputTokensDetails.ReasoningTokens != 3 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestOpenAIResponsesAllowedToolsAutoRoundTrip(t *testing.T) {
	canonical, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"function","name":"search","parameters":{"type":"object"}}
		]`),
		ToolChoice: json.RawMessage(`{
			"type":"allowed_tools",
			"mode":"auto",
			"tools":[
				{"type":"function","name":"lookup"},
				{"type":"function","name":"search"}
			]
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.ToolChoice == nil || canonical.ToolChoice.Mode != "AUTO" {
		t.Fatalf("tool choice = %+v", canonical.ToolChoice)
	}

	out, err := CanonicalToOpenAIResponsesRequest(canonical)
	if err != nil {
		t.Fatal(err)
	}
	var choice struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(out.ToolChoice, &choice); err != nil {
		t.Fatal(err)
	}
	if choice.Mode != "auto" {
		t.Fatalf("tool choice = %s", out.ToolChoice)
	}
}
