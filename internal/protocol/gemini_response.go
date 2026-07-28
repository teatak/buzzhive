package protocol

import (
	"fmt"
	"strings"
)

func GeminiToCanonicalResponse(resp GeminiGenerateResponse, model, id string, created int64, requestID string) CanonicalResponse {
	toolCalls := geminiResponseToolCalls(resp, requestID)
	finishReason := geminiFinishReasonToCanonical(geminiFinishReason(resp))
	if len(toolCalls) > 0 && finishReason == "stop" {
		finishReason = "tool_calls"
	}
	out := CanonicalResponse{
		ID:           id,
		Created:      created,
		Model:        model,
		Role:         "assistant",
		Text:         geminiResponseText(resp),
		Reasoning:    geminiResponseReasoning(resp),
		Signature:    geminiResponseSignature(resp),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        geminiUsage(resp),
	}
	if len(resp.Candidates) == 0 && resp.PromptFeedback != nil && strings.TrimSpace(resp.PromptFeedback.BlockReason) != "" {
		out.Refusal = firstNonEmpty(resp.PromptFeedback.BlockReasonMessage, "Gemini blocked the prompt: "+resp.PromptFeedback.BlockReason)
		out.FinishReason = "content_filter"
	}
	return out
}

func GeminiToCanonicalStreamEvents(resp GeminiGenerateResponse, requestID string, toolOffset int) []CanonicalStreamEvent {
	out := make([]CanonicalStreamEvent, 0, 6)
	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		toolIndex := 0
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall == nil && (part.Text != "" || part.ThoughtSignature != "") {
				eventType := CanonicalStreamTextDelta
				if part.Thought {
					eventType = CanonicalStreamReasoningDelta
				}
				out = append(out, CanonicalStreamEvent{
					Type:      eventType,
					Delta:     part.Text,
					Signature: part.ThoughtSignature,
				})
			}
			if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
				continue
			}
			index := toolOffset + toolIndex
			callID := strings.TrimSpace(part.FunctionCall.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%s_%d", requestID, index)
			}
			arguments := string(part.FunctionCall.Args)
			if strings.TrimSpace(arguments) == "" || arguments == "null" {
				arguments = "{}"
			}
			out = append(out,
				CanonicalStreamEvent{
					Type:      CanonicalStreamToolCallStart,
					Index:     index,
					CallID:    callID,
					Name:      part.FunctionCall.Name,
					Signature: part.ThoughtSignature,
				},
				CanonicalStreamEvent{
					Type:   CanonicalStreamToolArgumentsDelta,
					Index:  index,
					CallID: callID,
					Name:   part.FunctionCall.Name,
					Delta:  arguments,
				},
				CanonicalStreamEvent{
					Type:      CanonicalStreamToolCallDone,
					Index:     index,
					CallID:    callID,
					Name:      part.FunctionCall.Name,
					Arguments: arguments,
					Signature: part.ThoughtSignature,
				},
			)
			toolIndex++
		}
		if candidate.FinishReason != "" {
			finishReason := geminiFinishReasonToCanonical(candidate.FinishReason)
			if toolIndex > 0 && finishReason == "stop" {
				finishReason = "tool_calls"
			}
			out = append(out, CanonicalStreamEvent{
				Type:         CanonicalStreamResponseDone,
				FinishReason: finishReason,
			})
		}
	}
	if len(resp.Candidates) == 0 && resp.PromptFeedback != nil && strings.TrimSpace(resp.PromptFeedback.BlockReason) != "" {
		out = append(out,
			CanonicalStreamEvent{
				Type:  CanonicalStreamRefusalDelta,
				Delta: firstNonEmpty(resp.PromptFeedback.BlockReasonMessage, "Gemini blocked the prompt: "+resp.PromptFeedback.BlockReason),
			},
			CanonicalStreamEvent{
				Type:         CanonicalStreamResponseDone,
				FinishReason: "content_filter",
			},
		)
	}
	if usage := geminiUsage(resp); !usage.IsZero() {
		usageEvent := CanonicalStreamEvent{Type: CanonicalStreamUsage, Usage: usage}
		for i, event := range out {
			if event.Type == CanonicalStreamResponseDone {
				out = append(out[:i], append([]CanonicalStreamEvent{usageEvent}, out[i:]...)...)
				return out
			}
		}
		out = append(out, usageEvent)
	}
	return out
}

func CanonicalToGeminiGenerateResponse(resp CanonicalResponse) GeminiGenerateResponse {
	return GeminiGenerateResponse{
		Candidates: []GeminiCandidate{{
			Content: GeminiContent{
				Role:  "model",
				Parts: canonicalResponsePartsToGemini(resp.Reasoning, resp.Signature, firstNonEmpty(resp.Refusal, resp.Text), resp.ToolCalls),
			},
			FinishReason: canonicalFinishReasonToGemini(resp.FinishReason),
		}},
		UsageMetadata: canonicalUsageToGemini(resp.Usage),
	}
}

func CanonicalStreamEventToGeminiGenerateResponse(event CanonicalStreamEvent) (GeminiGenerateResponse, bool) {
	out := GeminiGenerateResponse{}
	switch event.Type {
	case CanonicalStreamReasoningDelta:
		out.Candidates = []GeminiCandidate{{
			Content: GeminiContent{
				Role: "model",
				Parts: []GeminiPart{{
					Text:             event.Delta,
					Thought:          true,
					ThoughtSignature: event.Signature,
				}},
			},
		}}
	case CanonicalStreamTextDelta, CanonicalStreamRefusalDelta:
		out.Candidates = []GeminiCandidate{{
			Content: GeminiContent{
				Role: "model",
				Parts: []GeminiPart{{
					Text:             event.Delta,
					ThoughtSignature: event.Signature,
				}},
			},
		}}
	case CanonicalStreamToolCallDone:
		out.Candidates = []GeminiCandidate{{
			Content: GeminiContent{
				Role: "model",
				Parts: []GeminiPart{{
					FunctionCall: &GeminiFunctionCall{
						ID:   event.CallID,
						Name: event.Name,
						Args: jsonRawObject(event.Arguments),
					},
					ThoughtSignature: event.Signature,
				}},
			},
		}}
	case CanonicalStreamUsage:
		out.UsageMetadata = canonicalUsageToGemini(event.Usage)
	case CanonicalStreamResponseDone:
		out.Candidates = []GeminiCandidate{{
			Content:      GeminiContent{Role: "model"},
			FinishReason: canonicalFinishReasonToGemini(event.FinishReason),
		}}
	default:
		return GeminiGenerateResponse{}, false
	}
	return out, true
}

func canonicalResponsePartsToGemini(reasoning string, signature string, text string, toolCalls []CanonicalToolCall) []GeminiPart {
	out := make([]GeminiPart, 0, 2+len(toolCalls))
	if reasoning != "" {
		out = append(out, GeminiPart{Text: reasoning, Thought: true, ThoughtSignature: signature})
	}
	if text != "" {
		out = append(out, GeminiPart{Text: text})
	}
	for _, call := range toolCalls {
		out = append(out, GeminiPart{
			FunctionCall: &GeminiFunctionCall{
				ID:   call.ID,
				Name: call.Name,
				Args: jsonRawObject(call.Arguments),
			},
			ThoughtSignature: call.Signature,
		})
	}
	return out
}

func canonicalUsageToGemini(usage CanonicalUsage) GeminiUsageMetadata {
	return GeminiUsageMetadata{
		PromptTokenCount:        usage.PromptTokens,
		CandidatesTokenCount:    usage.CompletionTokens,
		TotalTokenCount:         usage.TotalTokens,
		CachedContentTokenCount: usage.CachedTokens,
		ThoughtsTokenCount:      usage.ReasoningTokens,
	}
}

func geminiUsage(resp GeminiGenerateResponse) CanonicalUsage {
	return CanonicalUsage{
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

func geminiResponseToolCalls(resp GeminiGenerateResponse, requestID string) []CanonicalToolCall {
	if len(resp.Candidates) == 0 {
		return nil
	}
	var out []CanonicalToolCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
			continue
		}
		args := "{}"
		if len(part.FunctionCall.Args) > 0 && string(part.FunctionCall.Args) != "null" {
			args = string(part.FunctionCall.Args)
		}
		callID := strings.TrimSpace(part.FunctionCall.ID)
		if callID == "" {
			callID = fmt.Sprintf("call_%s_%d", requestID, len(out))
		}
		out = append(out, CanonicalToolCall{
			ID:        callID,
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
		if !part.Thought {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func geminiResponseReasoning(resp GeminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Thought {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func geminiResponseSignature(resp GeminiGenerateResponse) string {
	if len(resp.Candidates) == 0 {
		return ""
	}
	for i := len(resp.Candidates[0].Content.Parts) - 1; i >= 0; i-- {
		if signature := resp.Candidates[0].Content.Parts[i].ThoughtSignature; signature != "" {
			return signature
		}
	}
	return ""
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
