package protocol

import (
	"encoding/json"
	"testing"
)

func TestGeminiGenerateToCanonicalRequest(t *testing.T) {
	temp := 0.4
	maxTokens := 256
	req, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		SystemInstruction: &GeminiContent{Parts: []GeminiPart{{Text: "be brief"}}},
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: "hello"},
					{InlineData: &GeminiInlineData{MimeType: "image/png", Data: "aW1hZ2U="}},
				},
			},
			{
				Role: "model",
				Parts: []GeminiPart{{
					FunctionCall:     &GeminiFunctionCall{ID: "call_explicit", Name: "lookup", Args: json.RawMessage(`{"q":"hello"}`)},
					ThoughtSignature: "sig",
				}},
			},
			{
				Role: "user",
				Parts: []GeminiPart{{
					FunctionResponse: &GeminiFunctionResponse{ID: "call_explicit", Name: "lookup", Response: json.RawMessage(`{"result":"world"}`)},
				}},
			},
		},
		Tools: []GeminiTool{{FunctionDeclarations: []GeminiFunctionDeclaration{{
			Name:        "lookup",
			Description: "Lookup data",
			Parameters:  json.RawMessage(`{"type":"object"}`),
		}}}},
		ToolConfig: &GeminiToolConfig{FunctionCallingConfig: &GeminiFunctionCallingConfig{
			Mode:                 "ANY",
			AllowedFunctionNames: []string{"lookup"},
		}},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      &temp,
			MaxOutputTokens:  &maxTokens,
			StopSequences:    []string{"END"},
			ResponseMimeType: "application/json",
			ResponseSchema:   json.RawMessage(`{"type":"object"}`),
			ThinkingConfig:   &GeminiThinkingConfig{ThinkingLevel: "HIGH"},
		},
	}, "gemini-model", true)
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gemini-model" || !req.Stream || req.Temperature == nil || *req.Temperature != temp || req.MaxOutputTokens == nil || *req.MaxOutputTokens != maxTokens {
		t.Fatalf("basic fields not mapped: %+v", req)
	}
	if req.Reasoning == nil || req.Reasoning.Effort != "HIGH" {
		t.Fatalf("reasoning = %+v", req.Reasoning)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.MimeType != "application/json" || string(req.ResponseFormat.Schema) != `{"type":"object"}` {
		t.Fatalf("response format = %+v", req.ResponseFormat)
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", req.Tools)
	}
	if req.ToolChoice == nil || req.ToolChoice.Mode != "ANY" || len(req.ToolChoice.AllowedFunctionNames) != 1 || req.ToolChoice.AllowedFunctionNames[0] != "lookup" {
		t.Fatalf("tool choice = %+v", req.ToolChoice)
	}
	if len(req.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Parts[0].Text != "be brief" {
		t.Fatalf("system = %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Parts[1].Type != "image" {
		t.Fatalf("user = %+v", req.Messages[1])
	}
	if req.Messages[2].Role != "assistant" || req.Messages[2].Parts[0].Type != "tool_call" || req.Messages[2].Parts[0].Signature != "sig" {
		t.Fatalf("assistant = %+v", req.Messages[2])
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].Parts[0].Type != "tool_response" {
		t.Fatalf("tool = %+v", req.Messages[3])
	}
	if req.Messages[2].Parts[0].ToolCallID != req.Messages[3].Parts[0].ToolCallID {
		t.Fatalf("tool call ID %q does not match response ID %q", req.Messages[2].Parts[0].ToolCallID, req.Messages[3].Parts[0].ToolCallID)
	}
	if req.Messages[2].Parts[0].ToolCallID != "call_explicit" {
		t.Fatalf("tool call ID = %q", req.Messages[2].Parts[0].ToolCallID)
	}
	roundTrip, err := CanonicalToGeminiGenerateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Contents[1].Parts[0].FunctionCall.ID != "call_explicit" ||
		roundTrip.Contents[2].Parts[0].FunctionResponse.ID != "call_explicit" {
		t.Fatalf("round trip tool IDs = call:%q response:%q",
			roundTrip.Contents[1].Parts[0].FunctionCall.ID,
			roundTrip.Contents[2].Parts[0].FunctionResponse.ID,
		)
	}
}

func TestGeminiGenerateRequestAcceptsSystemInstructionJSONNames(t *testing.T) {
	for _, name := range []string{"system_instruction", "systemInstruction"} {
		t.Run(name, func(t *testing.T) {
			var req GeminiGenerateRequest
			body := `{"` + name + `":{"parts":[{"text":"be brief"}]},"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
			if err := json.Unmarshal([]byte(body), &req); err != nil {
				t.Fatal(err)
			}
			if req.SystemInstruction == nil ||
				len(req.SystemInstruction.Parts) != 1 ||
				req.SystemInstruction.Parts[0].Text != "be brief" {
				t.Fatalf("system instruction = %+v", req.SystemInstruction)
			}
		})
	}
}

func TestGeminiGenerateToCanonicalRequestCoalescesStreamedTextParts(t *testing.T) {
	req, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role: "model",
			Parts: []GeminiPart{
				{Text: "看起来"},
				{Text: "当前"},
				{Text: "模型"},
				{Text: "think", Thought: true},
				{Text: "ing", Thought: true},
			},
		}},
	}, "gemini-model", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 1 || len(req.Messages[0].Parts) != 2 {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if req.Messages[0].Parts[0].Type != "text" || req.Messages[0].Parts[0].Text != "看起来当前模型" {
		t.Fatalf("text part = %+v", req.Messages[0].Parts[0])
	}
	if req.Messages[0].Parts[1].Type != "reasoning" || req.Messages[0].Parts[1].Text != "thinking" {
		t.Fatalf("reasoning part = %+v", req.Messages[0].Parts[1])
	}

	openAI, err := CanonicalToOpenAIChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(openAI.Messages) != 1 ||
		string(openAI.Messages[0].Content) != `"看起来当前模型"` ||
		openAI.Messages[0].ReasoningContent != "thinking" {
		t.Fatalf("OpenAI message = %+v", openAI.Messages)
	}
}

func TestGeminiGenerateToCanonicalRequestAcceptsThinkingBudget(t *testing.T) {
	includeThoughts := true
	dynamicBudget := -1
	req, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role:  "user",
			Parts: []GeminiPart{{Text: "hi"}},
		}},
		GenerationConfig: &GeminiGenerationConfig{
			ThinkingConfig: &GeminiThinkingConfig{
				ThinkingBudget:  &dynamicBudget,
				IncludeThoughts: &includeThoughts,
			},
		},
	}, "public-model", true)
	if err != nil {
		t.Fatal(err)
	}
	if req.Reasoning == nil ||
		req.Reasoning.BudgetTokens != nil ||
		req.Reasoning.IncludeThoughts == nil ||
		!*req.Reasoning.IncludeThoughts {
		t.Fatalf("dynamic reasoning = %+v", req.Reasoning)
	}

	explicitBudget := 4096
	req, err = GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role:  "user",
			Parts: []GeminiPart{{Text: "hi"}},
		}},
		GenerationConfig: &GeminiGenerationConfig{
			ThinkingConfig: &GeminiThinkingConfig{ThinkingBudget: &explicitBudget},
		},
	}, "public-model", false)
	if err != nil {
		t.Fatal(err)
	}
	if req.Reasoning == nil ||
		req.Reasoning.BudgetTokens == nil ||
		*req.Reasoning.BudgetTokens != explicitBudget {
		t.Fatalf("explicit reasoning = %+v", req.Reasoning)
	}
}

func TestGeminiGenerateToCanonicalRequestRejectsMixedThinkingControls(t *testing.T) {
	budget := 1024
	_, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role:  "user",
			Parts: []GeminiPart{{Text: "hi"}},
		}},
		GenerationConfig: &GeminiGenerationConfig{
			ThinkingConfig: &GeminiThinkingConfig{
				ThinkingLevel:  "LOW",
				ThinkingBudget: &budget,
			},
		},
	}, "public-model", false)
	if err == nil {
		t.Fatal("expected mixed thinking controls to fail")
	}
}

func TestGeminiGenerateRequestRejectsDuplicateSystemInstructionNames(t *testing.T) {
	var req GeminiGenerateRequest
	err := json.Unmarshal([]byte(`{
		"system_instruction":{"parts":[{"text":"one"}]},
		"systemInstruction":{"parts":[{"text":"two"}]},
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`), &req)
	if err == nil {
		t.Fatal("expected duplicate system instruction error")
	}
}

func TestGeminiGenerateRejectsUnmatchedFunctionResponse(t *testing.T) {
	_, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role: "user",
			Parts: []GeminiPart{{
				FunctionResponse: &GeminiFunctionResponse{
					Name:     "lookup",
					Response: json.RawMessage(`{"result":"world"}`),
				},
			}},
		}},
	}, "gemini-model", false)
	if err == nil {
		t.Fatal("expected unmatched functionResponse error")
	}
}

func TestGeminiTextThoughtSignatureRoundTrip(t *testing.T) {
	canonical, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role: "model",
			Parts: []GeminiPart{
				{Text: "visible", ThoughtSignature: "sig-text"},
				{ThoughtSignature: "sig-empty"},
			},
		}},
	}, "gemini-model", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical.Messages) != 1 || len(canonical.Messages[0].Parts) != 2 {
		t.Fatalf("messages = %+v", canonical.Messages)
	}
	if canonical.Messages[0].Parts[0].Text != "visible" || canonical.Messages[0].Parts[0].Signature != "sig-text" {
		t.Fatalf("text part = %+v", canonical.Messages[0].Parts[0])
	}
	if canonical.Messages[0].Parts[1].Text != "" || canonical.Messages[0].Parts[1].Signature != "sig-empty" {
		t.Fatalf("signature-only part = %+v", canonical.Messages[0].Parts[1])
	}

	roundTrip, err := CanonicalToGeminiGenerateRequest(canonical)
	if err != nil {
		t.Fatal(err)
	}
	parts := roundTrip.Contents[0].Parts
	if len(parts) != 2 ||
		parts[0].Text != "visible" ||
		parts[0].ThoughtSignature != "sig-text" ||
		parts[1].Text != "" ||
		parts[1].ThoughtSignature != "sig-empty" {
		t.Fatalf("round trip parts = %+v", parts)
	}

	body, err := json.Marshal(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}
	contents := encoded["contents"].([]any)
	encodedParts := contents[0].(map[string]any)["parts"].([]any)
	signatureOnlyPart := encodedParts[1].(map[string]any)
	text, ok := signatureOnlyPart["text"]
	if !ok || text != "" {
		t.Fatalf("signature-only part must preserve explicit empty text: %s", body)
	}
}

func TestGeminiStructuredPartDoesNotSerializeEmptyText(t *testing.T) {
	body, err := json.Marshal(GeminiPart{
		FunctionCall:     &GeminiFunctionCall{Name: "search", Args: json.RawMessage(`{}`)},
		ThoughtSignature: "sig",
	})
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := encoded["text"]; ok {
		t.Fatalf("functionCall part must not also contain text: %s", body)
	}
}

func TestCanonicalToGeminiUsesFullJSONSchemaFields(t *testing.T) {
	toolSchema := json.RawMessage(`{
		"type":"object",
		"properties":{
			"filters":{
				"type":"object",
				"additionalProperties":{"type":"string"}
			}
		},
		"additionalProperties":false
	}`)
	responseSchema := json.RawMessage(`{
		"type":"object",
		"properties":{"answer":{"type":"string"}},
		"additionalProperties":false
	}`)

	req, err := CanonicalToGeminiGenerateRequest(CanonicalRequest{
		Messages: []CanonicalMessage{{
			Role:  "user",
			Parts: []CanonicalPart{{Type: "text", Text: "hello"}},
		}},
		Tools: []CanonicalTool{{
			Name:       "search",
			Parameters: toolSchema,
		}},
		ResponseFormat: &CanonicalResponseFormat{
			MimeType: "application/json",
			Schema:   responseSchema,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	declaration := req.Tools[0].FunctionDeclarations[0]
	if len(declaration.Parameters) != 0 {
		t.Fatalf("legacy parameters = %s", declaration.Parameters)
	}
	if string(declaration.ParametersJSONSchema) != string(toolSchema) {
		t.Fatalf("parametersJsonSchema = %s", declaration.ParametersJSONSchema)
	}
	if req.GenerationConfig == nil {
		t.Fatal("generationConfig is nil")
	}
	if len(req.GenerationConfig.ResponseSchema) != 0 {
		t.Fatalf("legacy responseSchema = %s", req.GenerationConfig.ResponseSchema)
	}
	if string(req.GenerationConfig.ResponseJSONSchema) != string(responseSchema) {
		t.Fatalf("responseJsonSchema = %s", req.GenerationConfig.ResponseJSONSchema)
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}
	tools := encoded["tools"].([]any)
	declarations := tools[0].(map[string]any)["functionDeclarations"].([]any)
	encodedDeclaration := declarations[0].(map[string]any)
	if _, ok := encodedDeclaration["parameters"]; ok {
		t.Fatalf("legacy parameters was serialized: %s", body)
	}
	if _, ok := encodedDeclaration["parametersJsonSchema"]; !ok {
		t.Fatalf("parametersJsonSchema was not serialized: %s", body)
	}
}

func TestGeminiRejectsBothSchemaRepresentations(t *testing.T) {
	_, err := GeminiGenerateToCanonicalRequest(GeminiGenerateRequest{
		Contents: []GeminiContent{{
			Role:  "user",
			Parts: []GeminiPart{{Text: "hello"}},
		}},
		Tools: []GeminiTool{{FunctionDeclarations: []GeminiFunctionDeclaration{{
			Name:                 "search",
			Parameters:           json.RawMessage(`{"type":"object"}`),
			ParametersJSONSchema: json.RawMessage(`{"type":"object"}`),
		}}}},
	}, "gemini-model", false)
	if err == nil {
		t.Fatal("expected mutually exclusive schema error")
	}
}
