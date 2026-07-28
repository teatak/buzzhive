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
					FunctionCall:     &GeminiFunctionCall{Name: "lookup", Args: json.RawMessage(`{"q":"hello"}`)},
					ThoughtSignature: "sig",
				}},
			},
			{
				Role: "user",
				Parts: []GeminiPart{{
					FunctionResponse: &GeminiFunctionResponse{Name: "lookup", Response: json.RawMessage(`{"result":"world"}`)},
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
}
