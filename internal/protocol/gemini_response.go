package protocol

import (
	"fmt"
	"strings"
)

func GeminiToCanonicalChatResponse(resp GeminiGenerateResponse, model, id string, created int64, requestID string) ChatResponse {
	toolCalls := geminiResponseToolCalls(resp, requestID)
	finishReason := geminiFinishReasonToCanonical(geminiFinishReason(resp))
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return ChatResponse{
		ID:           id,
		Created:      created,
		Model:        model,
		Role:         "assistant",
		Text:         geminiResponseText(resp),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        geminiUsage(resp),
	}
}

func GeminiToCanonicalStreamEvent(resp GeminiGenerateResponse, requestID string) ChatStreamEvent {
	toolCalls := geminiResponseToolCalls(resp, requestID)
	finishReason := ""
	if reason := geminiFinishReason(resp); reason != "" {
		finishReason = geminiFinishReasonToCanonical(reason)
	}
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return ChatStreamEvent{
		Text:         geminiResponseText(resp),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        geminiUsage(resp),
	}
}

func CanonicalToGeminiGenerateResponse(resp ChatResponse) GeminiGenerateResponse {
	return GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			Content: GeminiContent{
				Role:  "model",
				Parts: canonicalResponsePartsToGemini(resp.Text, resp.ToolCalls),
			},
			FinishReason: canonicalFinishReasonToGemini(resp.FinishReason),
		}},
		UsageMetadata: canonicalUsageToGemini(resp.Usage),
	}
}

func CanonicalStreamEventToGeminiGenerateResponse(event ChatStreamEvent) GeminiGenerateResponse {
	return GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			Content: GeminiContent{
				Role:  "model",
				Parts: canonicalResponsePartsToGemini(event.Text, event.ToolCalls),
			},
			FinishReason: canonicalFinishReasonToGemini(event.FinishReason),
		}},
		UsageMetadata: canonicalUsageToGemini(event.Usage),
	}
}

func canonicalResponsePartsToGemini(text string, toolCalls []ChatToolCall) []GeminiPart {
	out := make([]GeminiPart, 0, 1+len(toolCalls))
	if text != "" {
		out = append(out, GeminiPart{Text: text})
	}
	for _, call := range toolCalls {
		out = append(out, GeminiPart{
			FunctionCall: &GeminiFunctionCall{
				Name: call.Name,
				Args: jsonRawObject(call.Arguments),
			},
			ThoughtSignature: call.Signature,
		})
	}
	return out
}

func canonicalUsageToGemini(usage ChatUsage) GeminiUsageMetadata {
	return GeminiUsageMetadata{
		PromptTokenCount:        usage.PromptTokens,
		CandidatesTokenCount:    usage.CompletionTokens,
		TotalTokenCount:         usage.TotalTokens,
		CachedContentTokenCount: usage.CachedTokens,
		ThoughtsTokenCount:      usage.ReasoningTokens,
	}
}

func geminiUsage(resp GeminiGenerateResponse) ChatUsage {
	return ChatUsage{
		PromptTokens:     resp.UsageMetadata.PromptTokenCount,
		CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		CachedTokens:     resp.UsageMetadata.CachedContentTokenCount,
		ReasoningTokens:  resp.UsageMetadata.ThoughtsTokenCount,
	}
}

func canonicalFinishReasonToGemini(reason string) string {
	switch reason {
	case "", "stop", "tool_calls":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	default:
		return strings.ToUpper(reason)
	}
}

func jsonRawObject(value string) []byte {
	value = strings.TrimSpace(value)
	if value == "" {
		return []byte("{}")
	}
	return []byte(value)
}

func geminiResponseToolCalls(resp GeminiGenerateResponse, requestID string) []ChatToolCall {
	if len(resp.Candidates) == 0 {
		return nil
	}
	var out []ChatToolCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
			continue
		}
		args := "{}"
		if len(part.FunctionCall.Args) > 0 && string(part.FunctionCall.Args) != "null" {
			args = string(part.FunctionCall.Args)
		}
		out = append(out, ChatToolCall{
			ID:        fmt.Sprintf("call_%s_%d", requestID, len(out)),
			Name:      part.FunctionCall.Name,
			Arguments: args,
			Signature: part.ThoughtSignature,
		})
	}
	return out
}

func geminiResponseText(resp GeminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func geminiFinishReason(resp GeminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	return resp.Candidates[0].FinishReason
}

func geminiFinishReasonToCanonical(reason string) string {
	switch reason {
	case "", "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "content_filter"
	default:
		return strings.ToLower(reason)
	}
}
