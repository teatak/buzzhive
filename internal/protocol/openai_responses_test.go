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

func TestOpenAIResponsesToCanonicalAcceptsSafeOfficialOptions(t *testing.T) {
	background := false
	store := false
	parallelToolCalls := true
	maxToolCalls := 4
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model:                "deepseek-chat",
		Input:                json.RawMessage(`"hello"`),
		Background:           &background,
		Store:                &store,
		ParallelToolCalls:    &parallelToolCalls,
		MaxToolCalls:         &maxToolCalls,
		Include:              []string{"reasoning.encrypted_content"},
		Metadata:             json.RawMessage(`{"trace_id":"test"}`),
		PromptCacheKey:       "cache-key",
		PromptCacheOptions:   json.RawMessage(`{"mode":"implicit","ttl":"30m"}`),
		PromptCacheRetention: "in_memory",
		SafetyIdentifier:     "user-hash",
		ServiceTier:          "auto",
		StreamOptions:        json.RawMessage(`{"include_obfuscation":false}`),
		Truncation:           "disabled",
		User:                 "legacy-user",
		Reasoning: &OpenAIReasoning{
			Context: "current_turn",
			Effort:  "low",
			Mode:    "standard",
		},
		Text: &OpenAIResponseTextConfig{Verbosity: "medium"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "deepseek-chat" || len(req.Messages) != 1 || req.Messages[0].Parts[0].Text != "hello" {
		t.Fatalf("canonical request = %+v", req)
	}
	if req.Reasoning == nil || req.Reasoning.Effort != "low" {
		t.Fatalf("reasoning = %+v", req.Reasoning)
	}
}

func TestOpenAIResponsesToCanonicalAcceptsReturnedItemsAsInput(t *testing.T) {
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: json.RawMessage(`[
			{
				"type":"reasoning",
				"id":"rs_1",
				"status":"completed",
				"summary":[{"type":"summary_text","text":"considered options"}],
				"content":[{"type":"reasoning_text","text":" in detail"}],
				"encrypted_content":"encrypted"
			},
			{
				"type":"message",
				"id":"msg_1",
				"status":"completed",
				"phase":"final_answer",
				"role":"assistant",
				"content":[{
					"type":"output_text",
					"text":"previous answer",
					"annotations":[],
					"logprobs":[]
				}]
			},
			{
				"type":"message",
				"status":"completed",
				"role":"user",
				"content":[{"type":"input_text","text":"next question"}]
			}
		]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[0].Parts[0].Text != "considered options in detail" ||
		req.Messages[0].Parts[0].Signature != "encrypted" {
		t.Fatalf("reasoning = %+v", req.Messages[0])
	}
	if req.Messages[0].Role != "assistant" ||
		req.Messages[0].Parts[1].Text != "previous answer" ||
		req.Messages[1].Role != "user" ||
		req.Messages[1].Parts[0].Text != "next question" {
		t.Fatalf("messages = %+v", req.Messages)
	}
}

func TestOpenAIResponsesReasoningAndToolCallsBecomeOneAssistantMessage(t *testing.T) {
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: json.RawMessage(`[
			{
				"type":"reasoning",
				"id":"rs_1",
				"status":"completed",
				"summary":[{"type":"summary_text","text":"check weather"}],
				"encrypted_content":"encrypted"
			},
			{
				"type":"function_call",
				"id":"fc_1",
				"call_id":"call_weather",
				"status":"completed",
				"name":"get_weather",
				"arguments":"{\"city\":\"Beijing\"}"
			},
			{
				"type":"function_call",
				"id":"fc_2",
				"call_id":"call_time",
				"status":"completed",
				"name":"get_time",
				"arguments":"{}"
			},
			{
				"type":"function_call_output",
				"call_id":"call_weather",
				"status":"completed",
				"output":"sunny"
			},
			{
				"type":"function_call_output",
				"call_id":"call_time",
				"status":"completed",
				"output":"12:00"
			}
		]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 3 || req.Messages[0].Role != "assistant" ||
		len(req.Messages[0].Parts) != 3 {
		t.Fatalf("messages = %+v", req.Messages)
	}

	chat, err := CanonicalToOpenAIChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(chat.Messages) != 3 ||
		chat.Messages[0].Role != "assistant" ||
		chat.Messages[0].ReasoningContent != "check weather" ||
		len(chat.Messages[0].ToolCalls) != 2 {
		t.Fatalf("chat messages = %+v", chat.Messages)
	}
}

func TestOpenAIResponsesSignatureOnlyReasoningSerializesValidGeminiPart(t *testing.T) {
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model: "gemini-3.6-flash",
		Input: json.RawMessage(`[
			{
				"type":"reasoning",
				"id":"rs_1",
				"status":"completed",
				"summary":[],
				"encrypted_content":"thought-signature"
			},
			{
				"type":"function_call",
				"id":"fc_1",
				"call_id":"call_weather",
				"status":"completed",
				"name":"get_weather",
				"arguments":"{\"city\":\"Beijing\"}"
			},
			{
				"type":"function_call_output",
				"call_id":"call_weather",
				"status":"completed",
				"output":"sunny"
			},
			{
				"type":"message",
				"status":"completed",
				"role":"user",
				"content":[{"type":"input_text","text":"summarize"}]
			}
		]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	gemini, err := CanonicalToGeminiGenerateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(gemini)
	if err != nil {
		t.Fatal(err)
	}

	var encoded map[string]any
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}
	contents := encoded["contents"].([]any)
	modelParts := contents[0].(map[string]any)["parts"].([]any)
	signaturePart := modelParts[0].(map[string]any)
	if text, ok := signaturePart["text"]; !ok || text != "" {
		t.Fatalf("signature-only reasoning must initialize text data: %s", body)
	}
	if signaturePart["thoughtSignature"] != "thought-signature" {
		t.Fatalf("thought signature was not preserved: %s", body)
	}
	if _, ok := modelParts[1].(map[string]any)["functionCall"]; !ok {
		t.Fatalf("function call was not preserved: %s", body)
	}
}

func TestOpenAIResponsesEmptyHistoricalOutputIsOmittedFromGemini(t *testing.T) {
	req, err := OpenAIResponsesToCanonicalRequest(OpenAIResponsesRequest{
		Model: "gemini-3.6-flash",
		Input: json.RawMessage(`[
			{
				"type":"message",
				"status":"completed",
				"role":"assistant",
				"content":[{"type":"output_text","text":"","annotations":[]}]
			},
			{
				"type":"message",
				"status":"completed",
				"role":"user",
				"content":[{"type":"input_text","text":"continue"}]
			}
		]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	gemini, err := CanonicalToGeminiGenerateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(gemini.Contents) != 1 ||
		gemini.Contents[0].Role != "user" ||
		len(gemini.Contents[0].Parts) != 1 ||
		gemini.Contents[0].Parts[0].Text != "continue" {
		t.Fatalf("contents = %+v", gemini.Contents)
	}
	if _, err := json.Marshal(gemini); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIResponsesToCanonicalRejectsStatefulOfficialOptions(t *testing.T) {
	enabled := true
	disabled := false
	topLogprobs := 2
	tests := []struct {
		name    string
		mutate  func(*OpenAIResponsesRequest)
		wantErr string
	}{
		{
			name: "store",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Store = &enabled
			},
			wantErr: "store=true",
		},
		{
			name: "background",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Background = &enabled
			},
			wantErr: "background=true",
		},
		{
			name: "previous response",
			mutate: func(req *OpenAIResponsesRequest) {
				req.PreviousResponseID = "resp_previous"
			},
			wantErr: "previous_response_id",
		},
		{
			name: "conversation",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Conversation = json.RawMessage(`"conv_123"`)
			},
			wantErr: "conversation",
		},
		{
			name: "prompt template",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Prompt = json.RawMessage(`{"id":"pmpt_123"}`)
			},
			wantErr: "prompt templates",
		},
		{
			name: "parallel tools disabled",
			mutate: func(req *OpenAIResponsesRequest) {
				req.ParallelToolCalls = &disabled
			},
			wantErr: "parallel_tool_calls=false",
		},
		{
			name: "top logprobs",
			mutate: func(req *OpenAIResponsesRequest) {
				req.TopLogprobs = &topLogprobs
			},
			wantErr: "top_logprobs",
		},
		{
			name: "automatic truncation",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Truncation = "auto"
			},
			wantErr: "truncation",
		},
		{
			name: "low verbosity",
			mutate: func(req *OpenAIResponsesRequest) {
				req.Text = &OpenAIResponseTextConfig{Verbosity: "low"}
			},
			wantErr: "text verbosity",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := OpenAIResponsesRequest{
				Model: "deepseek-chat",
				Input: json.RawMessage(`"hello"`),
			}
			tt.mutate(&req)
			_, err := OpenAIResponsesToCanonicalRequest(req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
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

	refusal := CanonicalToOpenAIResponsesResponse(CanonicalResponse{
		ID:           "resp_refusal",
		Role:         "assistant",
		Refusal:      "cannot comply",
		FinishReason: "content_filter",
	})
	if refusal.Status != "completed" ||
		refusal.IncompleteDetails != nil ||
		len(refusal.Output) != 1 ||
		refusal.Output[0].Status != "completed" ||
		refusal.Output[0].Content[0].Type != "refusal" {
		t.Fatalf("completed refusal = %+v", refusal)
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
	for index, item := range resp.Output {
		if item.ID == "" {
			t.Fatalf("output[%d] has empty ID: %+v", index, item)
		}
	}
	if got := string(resp.Output[1].Content[0].Annotations); got != "[]" {
		t.Fatalf("output_text annotations = %s", got)
	}
	if resp.Output[2].CallID != "call_2" || resp.Output[2].ID == resp.Output[2].CallID {
		t.Fatalf("function item identifiers = id:%q call_id:%q", resp.Output[2].ID, resp.Output[2].CallID)
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
