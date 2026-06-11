package buzzhive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func writeSSEJSON(w io.Writer, flusher http.Flusher, event string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	fmt.Fprintf(w, "data: %s\n\n", raw)
	if flusher != nil {
		flusher.Flush()
	}
}

func writeResponsesStreamStart(w io.Writer, flusher http.Flusher, id string, created int64, model string) {
	writeSSEJSON(w, flusher, "response.created", map[string]any{
		"type":            "response.created",
		"sequence_number": 1,
		"response": map[string]any{
			"id":         id,
			"object":     "response",
			"created_at": created,
			"status":     "in_progress",
			"model":      model,
			"output":     []any{},
		},
	})
	writeSSEJSON(w, flusher, "response.output_item.added", map[string]any{
		"type":            "response.output_item.added",
		"sequence_number": 2,
		"output_index":    0,
		"item": map[string]any{
			"id":      id + "_msg",
			"type":    "message",
			"status":  "in_progress",
			"role":    "assistant",
			"content": []any{},
		},
	})
	writeSSEJSON(w, flusher, "response.content_part.added", map[string]any{
		"type":            "response.content_part.added",
		"sequence_number": 3,
		"item_id":         id + "_msg",
		"output_index":    0,
		"content_index":   0,
		"part": map[string]any{
			"type": "output_text",
			"text": "",
		},
	})
}

func writeResponsesStreamDelta(w io.Writer, flusher http.Flusher, id string, sequence int, text string) {
	if text == "" {
		return
	}
	writeSSEJSON(w, flusher, "response.output_text.delta", map[string]any{
		"type":            "response.output_text.delta",
		"sequence_number": sequence,
		"item_id":         id + "_msg",
		"output_index":    0,
		"content_index":   0,
		"delta":           text,
	})
}

func writeResponsesStreamDone(w io.Writer, flusher http.Flusher, id string, created int64, model string, text string, usage protocol.ChatUsage, sequence int) {
	writeSSEJSON(w, flusher, "response.output_text.done", map[string]any{
		"type":            "response.output_text.done",
		"sequence_number": sequence,
		"item_id":         id + "_msg",
		"output_index":    0,
		"content_index":   0,
		"text":            text,
	})
	resp := protocol.CanonicalToOpenAIResponsesResponse(protocol.ChatResponse{
		ID:           id,
		Created:      created,
		Model:        model,
		Role:         "assistant",
		Text:         text,
		FinishReason: "stop",
		Usage:        usage,
	})
	writeSSEJSON(w, flusher, "response.completed", map[string]any{
		"type":            "response.completed",
		"sequence_number": sequence + 1,
		"response":        resp,
	})
}

func writeAnthropicStreamStart(w io.Writer, flusher http.Flusher, id string, model string) {
	writeSSEJSON(w, flusher, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            id,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens": 0,
			},
		},
	})
	writeSSEJSON(w, flusher, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})
}

func writeAnthropicTextDelta(w io.Writer, flusher http.Flusher, text string) {
	if text == "" {
		return
	}
	writeSSEJSON(w, flusher, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})
}

func writeAnthropicStreamDone(w io.Writer, flusher http.Flusher, usage protocol.ChatUsage) {
	writeSSEJSON(w, flusher, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})
	writeSSEJSON(w, flusher, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"input_tokens":                usage.PromptTokens,
			"output_tokens":               usage.CompletionTokens,
			"cache_read_input_tokens":     usage.CachedTokens,
			"cache_creation_input_tokens": 0,
		},
	})
	writeSSEJSON(w, flusher, "message_stop", map[string]any{
		"type": "message_stop",
	})
}

func writeGeminiStreamEvent(w io.Writer, flusher http.Flusher, event protocol.ChatStreamEvent) {
	resp := protocol.CanonicalStreamEventToGeminiGenerateResponse(event)
	writeSSEJSON(w, flusher, "", resp)
}

func readOpenAIChatStreamAsCanonical(r io.Reader, onEvent func(protocol.ChatStreamEvent)) protocol.ChatUsage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var usage protocol.ChatUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk protocol.OpenAIChatResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		event := protocol.OpenAIChatStreamChunkToCanonical(chunk)
		if !canonicalUsageZero(event.Usage) {
			usage = event.Usage
		}
		onEvent(event)
	}
	return usage
}

func readGeminiStreamAsCanonical(r io.Reader, requestID string, onEvent func(protocol.ChatStreamEvent)) protocol.ChatUsage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var usage protocol.ChatUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var resp protocol.GeminiGenerateResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			continue
		}
		event := protocol.GeminiToCanonicalStreamEvent(resp, requestID)
		if !canonicalUsageZero(event.Usage) {
			usage = event.Usage
		}
		onEvent(event)
	}
	return usage
}

func readResponsesStreamAsCanonical(r io.Reader, onEvent func(protocol.ChatStreamEvent)) protocol.ChatUsage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var usage protocol.ChatUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type     string                           `json:"type"`
			Delta    string                           `json:"delta"`
			Response protocol.OpenAIResponsesResponse `json:"response"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "response.output_text.delta":
			onEvent(protocol.ChatStreamEvent{Text: event.Delta})
		case "response.completed":
			canonical := protocol.OpenAIResponsesResponseToCanonical(event.Response)
			usage = canonical.Usage
			onEvent(protocol.ChatStreamEvent{FinishReason: canonical.FinishReason, Usage: canonical.Usage})
		}
	}
	return usage
}

func readAnthropicStreamAsCanonical(r io.Reader, onEvent func(protocol.ChatStreamEvent)) protocol.ChatUsage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var usage protocol.ChatUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage protocol.AnthropicUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				onEvent(protocol.ChatStreamEvent{Text: event.Delta.Text})
			}
		case "message_delta":
			usage = protocol.ChatUsage{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
				CachedTokens:     event.Usage.CacheReadInputTokens,
			}
			onEvent(protocol.ChatStreamEvent{FinishReason: "stop", Usage: usage})
		}
	}
	return usage
}

func canonicalUsageZero(usage protocol.ChatUsage) bool {
	return usage.PromptTokens == 0 &&
		usage.CompletionTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.CachedTokens == 0 &&
		usage.ReasoningTokens == 0
}
