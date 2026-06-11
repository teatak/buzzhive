package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type OpenAIResponsesRequest struct {
	Model           string                    `json:"model"`
	Input           json.RawMessage           `json:"input"`
	Instructions    string                    `json:"instructions,omitempty"`
	MaxOutputTokens *int                      `json:"max_output_tokens,omitempty"`
	Temperature     *float64                  `json:"temperature,omitempty"`
	TopP            *float64                  `json:"top_p,omitempty"`
	Tools           json.RawMessage           `json:"tools,omitempty"`
	ToolChoice      json.RawMessage           `json:"tool_choice,omitempty"`
	Reasoning       *OpenAIReasoning          `json:"reasoning,omitempty"`
	Text            *OpenAIResponseTextConfig `json:"text,omitempty"`
	Stream          bool                      `json:"stream,omitempty"`
}

type OpenAIReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type OpenAIResponseTextConfig struct {
	Format *OpenAIResponseTextFormat `json:"format,omitempty"`
}

type OpenAIResponseTextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

type OpenAIResponseInputItem struct {
	Type      string          `json:"type,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	ID        string          `json:"id,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Output    string          `json:"output,omitempty"`
}

type OpenAIResponseContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type OpenAIResponsesResponse struct {
	ID        string                     `json:"id"`
	Object    string                     `json:"object"`
	CreatedAt int64                      `json:"created_at"`
	Status    string                     `json:"status"`
	Model     string                     `json:"model"`
	Output    []OpenAIResponseOutputItem `json:"output"`
	Usage     *OpenAIResponsesUsage      `json:"usage,omitempty"`
}

type OpenAIResponseOutputItem struct {
	Type      string                     `json:"type"`
	ID        string                     `json:"id,omitempty"`
	Status    string                     `json:"status,omitempty"`
	Role      string                     `json:"role,omitempty"`
	Content   []OpenAIResponseOutputPart `json:"content,omitempty"`
	CallID    string                     `json:"call_id,omitempty"`
	Name      string                     `json:"name,omitempty"`
	Arguments string                     `json:"arguments,omitempty"`
}

type OpenAIResponseOutputPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type OpenAIResponsesUsage struct {
	InputTokens         int                                `json:"input_tokens"`
	OutputTokens        int                                `json:"output_tokens"`
	TotalTokens         int                                `json:"total_tokens"`
	InputTokensDetails  OpenAIResponsesInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokensDetails OpenAIResponsesOutputTokensDetails `json:"output_tokens_details,omitempty"`
}

type OpenAIResponsesInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type OpenAIResponsesOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

func OpenAIResponsesToCanonicalRequest(req OpenAIResponsesRequest) (ChatRequest, error) {
	out := ChatRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
	}
	if req.Reasoning != nil && strings.TrimSpace(req.Reasoning.Effort) != "" {
		out.ThinkingLevel = &req.Reasoning.Effort
	}
	if req.Text != nil && req.Text.Format != nil {
		out.ResponseFormat = openAIResponsesTextFormatToCanonical(req.Text.Format)
	}
	tools, err := openAIResponsesToolsToCanonical(req.Tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := openAIResponsesToolChoiceToCanonical(req.ToolChoice, tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.ToolChoice = toolChoice
	if strings.TrimSpace(req.Instructions) != "" {
		out.Messages = append(out.Messages, ChatMessage{
			Role:  "system",
			Parts: []ChatPart{{Type: "text", Text: req.Instructions}},
		})
	}
	messages, err := openAIResponsesInputToCanonical(req.Input)
	if err != nil {
		return ChatRequest{}, err
	}
	out.Messages = append(out.Messages, messages...)
	return out, nil
}

func CanonicalToOpenAIResponsesRequest(req ChatRequest) (OpenAIResponsesRequest, error) {
	out := OpenAIResponsesRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
	}
	if req.ThinkingLevel != nil && strings.TrimSpace(*req.ThinkingLevel) != "" {
		out.Reasoning = &OpenAIReasoning{Effort: *req.ThinkingLevel}
	}
	if req.ResponseFormat != nil {
		out.Text = &OpenAIResponseTextConfig{Format: canonicalResponseFormatToResponses(req.ResponseFormat)}
	}
	tools, err := canonicalToolsToOpenAI(req.Tools)
	if err != nil {
		return OpenAIResponsesRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := canonicalToolChoiceToOpenAI(req.ToolChoice)
	if err != nil {
		return OpenAIResponsesRequest{}, err
	}
	out.ToolChoice = toolChoice
	var items []OpenAIResponseInputItem
	for _, message := range req.Messages {
		if message.Role == "system" || message.Role == "developer" {
			text := canonicalText(message.Parts)
			if text != "" {
				if out.Instructions != "" {
					out.Instructions += "\n"
				}
				out.Instructions += text
			}
			continue
		}
		next, err := canonicalMessageToOpenAIResponsesInput(message)
		if err != nil {
			return OpenAIResponsesRequest{}, err
		}
		items = append(items, next...)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return OpenAIResponsesRequest{}, err
	}
	out.Input = raw
	return out, nil
}

func OpenAIResponsesResponseToCanonical(resp OpenAIResponsesResponse) ChatResponse {
	out := ChatResponse{
		ID:      resp.ID,
		Created: resp.CreatedAt,
		Model:   resp.Model,
		Role:    "assistant",
		Usage:   openAIResponsesUsageToCanonical(resp.Usage),
	}
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					out.Text += part.Text
				}
			}
		case "function_call":
			out.ToolCalls = append(out.ToolCalls, ChatToolCall{
				ID:        firstNonEmpty(item.CallID, item.ID),
				Name:      item.Name,
				Arguments: item.Arguments,
			})
		}
	}
	if len(out.ToolCalls) > 0 {
		out.FinishReason = "tool_calls"
	} else if resp.Status == "completed" {
		out.FinishReason = "stop"
	} else if resp.Status != "" {
		out.FinishReason = resp.Status
	}
	return out
}

func CanonicalToOpenAIResponsesResponse(resp ChatResponse) OpenAIResponsesResponse {
	out := OpenAIResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: resp.Created,
		Status:    canonicalFinishReasonToResponsesStatus(resp.FinishReason),
		Model:     resp.Model,
		Usage:     canonicalUsageToResponses(resp.Usage),
	}
	if resp.Text != "" {
		out.Output = append(out.Output, OpenAIResponseOutputItem{
			Type:   "message",
			Status: "completed",
			Role:   resp.Role,
			Content: []OpenAIResponseOutputPart{{
				Type: "output_text",
				Text: resp.Text,
			}},
		})
	}
	for _, call := range resp.ToolCalls {
		out.Output = append(out.Output, OpenAIResponseOutputItem{
			Type:      "function_call",
			ID:        call.ID,
			CallID:    call.ID,
			Status:    "completed",
			Name:      call.Name,
			Arguments: call.Arguments,
		})
	}
	return out
}

func openAIResponsesInputToCanonical(raw json.RawMessage) ([]ChatMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []ChatMessage{{Role: "user", Parts: []ChatPart{{Type: "text", Text: text}}}}, nil
	}
	var items []OpenAIResponseInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	var out []ChatMessage
	for _, item := range items {
		switch item.Type {
		case "", "message":
			parts, err := openAIResponsesContentToCanonical(item.Content)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatMessage{Role: item.Role, Parts: parts})
		case "function_call":
			args := json.RawMessage(item.Arguments)
			if len(bytes.TrimSpace(args)) == 0 {
				args = json.RawMessage(`{}`)
			}
			out = append(out, ChatMessage{Role: "assistant", Parts: []ChatPart{{
				Type:       "tool_call",
				ToolCallID: firstNonEmpty(item.CallID, item.ID),
				Name:       item.Name,
				Arguments:  args,
			}}})
		case "function_call_output":
			out = append(out, ChatMessage{Role: "tool", Parts: []ChatPart{{
				Type:       "tool_response",
				ToolCallID: item.CallID,
				Response:   json.RawMessage(quoteJSONString(item.Output)),
			}}})
		default:
			return nil, fmt.Errorf("unsupported Responses input item type %q", item.Type)
		}
	}
	return out, nil
}

func openAIResponsesContentToCanonical(raw json.RawMessage) ([]ChatPart, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []ChatPart{{Type: "text", Text: text}}, nil
	}
	var content []OpenAIResponseContentPart
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil, err
	}
	out := make([]ChatPart, 0, len(content))
	for _, part := range content {
		switch part.Type {
		case "input_text", "output_text":
			out = append(out, ChatPart{Type: "text", Text: part.Text})
		case "input_image":
			mimeType, data, err := parseOpenAIImageDataURL(part.ImageURL)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatPart{Type: "image", MimeType: mimeType, Data: data})
		default:
			return nil, fmt.Errorf("unsupported Responses content type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalMessageToOpenAIResponsesInput(message ChatMessage) ([]OpenAIResponseInputItem, error) {
	var contentParts []ChatPart
	var out []OpenAIResponseInputItem
	for _, part := range message.Parts {
		switch part.Type {
		case "text", "image":
			contentParts = append(contentParts, part)
		case "tool_call":
			out = append(out, OpenAIResponseInputItem{
				Type:      "function_call",
				CallID:    part.ToolCallID,
				Name:      part.Name,
				Arguments: string(part.Arguments),
			})
		case "tool_response":
			output := ""
			if len(bytes.TrimSpace(part.Response)) > 0 {
				output = string(bytes.TrimSpace(part.Response))
			}
			out = append(out, OpenAIResponseInputItem{
				Type:   "function_call_output",
				CallID: part.ToolCallID,
				Output: output,
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	if len(contentParts) > 0 {
		content, err := canonicalPartsToOpenAIResponsesContent(contentParts)
		if err != nil {
			return nil, err
		}
		out = append([]OpenAIResponseInputItem{{
			Type:    "message",
			Role:    message.Role,
			Content: content,
		}}, out...)
	}
	return out, nil
}

func canonicalPartsToOpenAIResponsesContent(parts []ChatPart) (json.RawMessage, error) {
	if len(parts) == 1 && parts[0].Type == "text" {
		return marshalJSONRaw(parts[0].Text)
	}
	out := make([]OpenAIResponseContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, OpenAIResponseContentPart{Type: "input_text", Text: part.Text})
		case "image":
			out = append(out, OpenAIResponseContentPart{Type: "input_image", ImageURL: dataURL(part.MimeType, part.Data), Detail: "auto"})
		default:
			return nil, fmt.Errorf("unsupported Responses content part type %q", part.Type)
		}
	}
	return marshalJSONRaw(out)
}

func canonicalText(parts []ChatPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Type == "text" {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func openAIResponsesToolsToCanonical(raw json.RawMessage) ([]ChatTool, error) {
	if !openAIToolsRequested(raw) {
		return nil, nil
	}
	return openAIToolsToCanonical(raw)
}

func openAIResponsesToolChoiceToCanonical(raw json.RawMessage, tools []ChatTool) (*ChatToolChoice, error) {
	return openAIToolChoiceToCanonical(raw, tools)
}

func openAIResponsesTextFormatToCanonical(format *OpenAIResponseTextFormat) *ChatResponseFormat {
	if format == nil || format.Type == "" || format.Type == "text" {
		return nil
	}
	if format.Type == "json_schema" {
		return &ChatResponseFormat{MimeType: "application/json", Schema: format.Schema}
	}
	if format.Type == "json_object" {
		return &ChatResponseFormat{MimeType: "application/json"}
	}
	return &ChatResponseFormat{MimeType: format.Type, Schema: format.Schema}
}

func canonicalResponseFormatToResponses(format *ChatResponseFormat) *OpenAIResponseTextFormat {
	if format == nil {
		return nil
	}
	if len(bytes.TrimSpace(format.Schema)) > 0 {
		return &OpenAIResponseTextFormat{Type: "json_schema", Name: "canonical_schema", Schema: format.Schema}
	}
	if format.MimeType == "application/json" {
		return &OpenAIResponseTextFormat{Type: "json_object"}
	}
	return &OpenAIResponseTextFormat{Type: format.MimeType}
}

func openAIResponsesUsageToCanonical(usage *OpenAIResponsesUsage) ChatUsage {
	if usage == nil {
		return ChatUsage{}
	}
	return ChatUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CachedTokens:     usage.InputTokensDetails.CachedTokens,
		ReasoningTokens:  usage.OutputTokensDetails.ReasoningTokens,
	}
}

func canonicalUsageToResponses(usage ChatUsage) *OpenAIResponsesUsage {
	return &OpenAIResponsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
		InputTokensDetails: OpenAIResponsesInputTokensDetails{
			CachedTokens: usage.CachedTokens,
		},
		OutputTokensDetails: OpenAIResponsesOutputTokensDetails{
			ReasoningTokens: usage.ReasoningTokens,
		},
	}
}

func canonicalFinishReasonToResponsesStatus(reason string) string {
	switch reason {
	case "", "stop":
		return "completed"
	default:
		return reason
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func quoteJSONString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
