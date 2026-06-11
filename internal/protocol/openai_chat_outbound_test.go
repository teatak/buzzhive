package protocol

import (
	"encoding/json"
	"testing"
)

func TestCanonicalToOpenAIChatRequest(t *testing.T) {
	maxTokens := 128
	temperature := 0.2
	req, err := CanonicalToOpenAIChatRequest(ChatRequest{
		Model:           "public-model",
		Stream:          true,
		Temperature:     &temperature,
		MaxOutputTokens: &maxTokens,
		StopSequences:   []string{"END"},
		Tools: []ChatTool{{
			Name:        "lookup",
			Description: "Lookup data",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
		}},
		ToolChoice: &ChatToolChoice{Mode: "ANY", AllowedFunctionNames: []string{"lookup"}},
		Messages: []ChatMessage{
			{Role: "system", Parts: []ChatPart{{Type: "text", Text: "be brief"}}},
			{Role: "user", Parts: []ChatPart{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Parts: []ChatPart{{
				Type:       "tool_call",
				ToolCallID: "call_1",
				Name:       "lookup",
				Arguments:  json.RawMessage(`{"q":"hello"}`),
			}}},
			{Role: "tool", Parts: []ChatPart{{
				Type:       "tool_response",
				ToolCallID: "call_1",
				Response:   json.RawMessage(`{"result":"world"}`),
			}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "public-model" || !req.Stream || req.MaxOutputTokens == nil || *req.MaxOutputTokens != 128 {
		t.Fatalf("basic fields not mapped: %+v", req)
	}
	if got, ok := req.Stop.(string); !ok || got != "END" {
		t.Fatalf("stop = %#v, want END", req.Stop)
	}
	if len(req.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(req.Messages))
	}
	if string(req.Messages[1].Content) != `"hello"` {
		t.Fatalf("user content = %s", req.Messages[1].Content)
	}
	if len(req.Messages[2].ToolCalls) != 1 || req.Messages[2].ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("assistant tool calls = %+v", req.Messages[2].ToolCalls)
	}
	if string(req.Messages[2].Content) != "null" {
		t.Fatalf("assistant tool content = %s, want null", req.Messages[2].Content)
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].ToolCallID != "call_1" {
		t.Fatalf("tool message = %+v", req.Messages[3])
	}
	if string(req.Messages[3].Content) != `"{\"result\":\"world\"}"` {
		t.Fatalf("tool content = %s", req.Messages[3].Content)
	}

	var tools []OpenAITool
	if err := json.Unmarshal(req.Tools, &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Function.Name != "lookup" {
		t.Fatalf("tools = %+v", tools)
	}
	var choice struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(req.ToolChoice, &choice); err != nil {
		t.Fatal(err)
	}
	if choice.Type != "function" || choice.Function.Name != "lookup" {
		t.Fatalf("tool_choice = %+v", choice)
	}
}

func TestCanonicalToOpenAIChatRequestMultimodal(t *testing.T) {
	req, err := CanonicalToOpenAIChatRequest(ChatRequest{
		Model: "vision",
		Messages: []ChatMessage{{
			Role: "user",
			Parts: []ChatPart{
				{Type: "text", Text: "describe"},
				{Type: "image", MimeType: "image/png", Data: "aW1hZ2U="},
				{Type: "audio", MimeType: "audio/wav", Data: "YXVkaW8="},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var parts []openAIContentPart
	if err := json.Unmarshal(req.Messages[0].Content, &parts); err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(parts))
	}
	if parts[1].ImageURL.URL != "data:image/png;base64,aW1hZ2U=" {
		t.Fatalf("image url = %s", parts[1].ImageURL.URL)
	}
	if parts[2].InputAudio.Format != "wav" {
		t.Fatalf("audio format = %s", parts[2].InputAudio.Format)
	}
}

func TestCanonicalToOpenAIChatRequestResponseFormat(t *testing.T) {
	req, err := CanonicalToOpenAIChatRequest(ChatRequest{
		Model: "json-model",
		ResponseFormat: &ChatResponseFormat{
			MimeType: "application/json",
			Schema:   json.RawMessage(`{"type":"object"}`),
		},
		Messages: []ChatMessage{{Role: "user", Parts: []ChatPart{{Type: "text", Text: "json"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var format struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(req.ResponseFormat, &format); err != nil {
		t.Fatal(err)
	}
	if format.Type != "json_schema" || format.JSONSchema.Name != "canonical_schema" || string(format.JSONSchema.Schema) != `{"type":"object"}` {
		t.Fatalf("response_format = %+v", format)
	}
}

func TestCanonicalToOpenAIChatRequestAllowedTools(t *testing.T) {
	req, err := CanonicalToOpenAIChatRequest(ChatRequest{
		Model: "public-model",
		Tools: []ChatTool{
			{Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)},
			{Name: "search", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolChoice: &ChatToolChoice{Mode: "ANY", AllowedFunctionNames: []string{"lookup", "search"}},
		Messages:   []ChatMessage{{Role: "user", Parts: []ChatPart{{Type: "text", Text: "hello"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var choice struct {
		Type         string `json:"type"`
		AllowedTools struct {
			Mode  string `json:"mode"`
			Tools []struct {
				Type     string `json:"type"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		} `json:"allowed_tools"`
	}
	if err := json.Unmarshal(req.ToolChoice, &choice); err != nil {
		t.Fatal(err)
	}
	if choice.Type != "allowed_tools" || choice.AllowedTools.Mode != "required" || len(choice.AllowedTools.Tools) != 2 {
		t.Fatalf("tool_choice = %+v", choice)
	}
	if choice.AllowedTools.Tools[0].Function.Name != "lookup" || choice.AllowedTools.Tools[1].Function.Name != "search" {
		t.Fatalf("allowed tools = %+v", choice.AllowedTools.Tools)
	}
}

func TestOpenAIChatToCanonicalAllowedTools(t *testing.T) {
	req, err := OpenAIChatToCanonical(OpenAIChatRequest{
		Model:    "public-model",
		Messages: []OpenAIMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
		Tools: json.RawMessage(`[
			{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}},
			{"type":"function","function":{"name":"search","parameters":{"type":"object"}}}
		]`),
		ToolChoice: json.RawMessage(`{
			"type":"allowed_tools",
			"allowed_tools":{
				"mode":"required",
				"tools":[
					{"type":"function","function":{"name":"lookup"}},
					{"type":"function","function":{"name":"search"}}
				]
			}
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || len(req.ToolChoice.AllowedFunctionNames) != 2 {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
	if req.ToolChoice.AllowedFunctionNames[0] != "lookup" || req.ToolChoice.AllowedFunctionNames[1] != "search" {
		t.Fatalf("allowed names = %+v", req.ToolChoice.AllowedFunctionNames)
	}
}
