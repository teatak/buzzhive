package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type AnthropicMessagesRequest struct {
	Model         string               `json:"model"`
	System        any                  `json:"system,omitempty"`
	Messages      []AnthropicMessage   `json:"messages"`
	MaxTokens     *int                 `json:"max_tokens,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice `json:"tool_choice,omitempty"`
	Thinking      *AnthropicThinking   `json:"thinking,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

func (m *AnthropicMessage) UnmarshalJSON(raw []byte) error {
	var aux struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &aux); err != nil {
		return err
	}
	m.Role = aux.Role
	var text string
	if err := json.Unmarshal(aux.Content, &text); err == nil {
		m.Content = []AnthropicContent{{Type: "text", Text: text}}
		return nil
	}
	var content []AnthropicContent
	if err := json.Unmarshal(aux.Content, &content); err != nil {
		return err
	}
	m.Content = content
	return nil
}

type AnthropicContent struct {
	Type      string           `json:"type"`
	Text      string           `json:"text,omitempty"`
	Source    *AnthropicSource `json:"source,omitempty"`
	ID        string           `json:"id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Input     json.RawMessage  `json:"input,omitempty"`
	ToolUseID string           `json:"tool_use_id,omitempty"`
	Content   any              `json:"content,omitempty"`
}

type AnthropicSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type AnthropicThinking struct {
	Type string `json:"type"`
}

type AnthropicMessagesResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Model        string             `json:"model"`
	Content      []AnthropicContent `json:"content"`
	StopReason   string             `json:"stop_reason,omitempty"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage     `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

func AnthropicMessagesToCanonicalRequest(req AnthropicMessagesRequest) (ChatRequest, error) {
	out := ChatRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxTokens,
		StopSequences:   req.StopSequences,
	}
	if req.Thinking != nil && strings.TrimSpace(req.Thinking.Type) != "" {
		out.ThinkingLevel = &req.Thinking.Type
	}
	tools, err := anthropicToolsToCanonical(req.Tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice = anthropicToolChoiceToCanonical(req.ToolChoice)
	systemParts, err := anthropicSystemToCanonicalParts(req.System)
	if err != nil {
		return ChatRequest{}, err
	}
	if !canonicalPartsEmpty(systemParts) {
		out.Messages = append(out.Messages, ChatMessage{Role: "system", Parts: systemParts})
	}
	for _, message := range req.Messages {
		parts, err := anthropicContentToCanonical(message.Content)
		if err != nil {
			return ChatRequest{}, err
		}
		if canonicalPartsEmpty(parts) {
			continue
		}
		out.Messages = append(out.Messages, ChatMessage{
			Role:  anthropicRoleToCanonical(message.Role, parts),
			Parts: parts,
		})
	}
	return out, nil
}

func CanonicalToAnthropicMessagesRequest(req ChatRequest) (AnthropicMessagesRequest, error) {
	out := AnthropicMessagesRequest{
		Model:         req.Model,
		MaxTokens:     req.MaxOutputTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
	}
	if req.ThinkingLevel != nil && strings.TrimSpace(*req.ThinkingLevel) != "" {
		out.Thinking = &AnthropicThinking{Type: *req.ThinkingLevel}
	}
	tools, err := canonicalToolsToAnthropic(req.Tools)
	if err != nil {
		return AnthropicMessagesRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice = canonicalToolChoiceToAnthropic(req.ToolChoice)
	for _, message := range req.Messages {
		content, err := canonicalPartsToAnthropicContent(message.Parts)
		if err != nil {
			return AnthropicMessagesRequest{}, err
		}
		if len(content) == 0 {
			continue
		}
		if message.Role == "system" || message.Role == "developer" {
			out.System = content
			continue
		}
		out.Messages = append(out.Messages, AnthropicMessage{
			Role:    canonicalRoleToAnthropic(message.Role),
			Content: content,
		})
	}
	return out, nil
}

func AnthropicMessagesResponseToCanonical(resp AnthropicMessagesResponse) ChatResponse {
	parts, _ := anthropicContentToCanonical(resp.Content)
	out := ChatResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Role:         anthropicRoleToCanonical(resp.Role, parts),
		FinishReason: anthropicStopReasonToCanonical(resp.StopReason),
		Usage: ChatUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
			CachedTokens:     resp.Usage.CacheReadInputTokens,
		},
	}
	for _, part := range parts {
		switch part.Type {
		case "text":
			out.Text += part.Text
		case "tool_call":
			out.ToolCalls = append(out.ToolCalls, ChatToolCall{
				ID:        part.ToolCallID,
				Name:      part.Name,
				Arguments: string(part.Arguments),
				Signature: part.Signature,
			})
		}
	}
	if len(out.ToolCalls) > 0 {
		out.FinishReason = "tool_calls"
	}
	return out
}

func CanonicalToAnthropicMessagesResponse(resp ChatResponse) AnthropicMessagesResponse {
	return AnthropicMessagesResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       canonicalRoleToAnthropic(resp.Role),
		Model:      resp.Model,
		Content:    canonicalResponseToAnthropicContent(resp.Text, resp.ToolCalls),
		StopReason: canonicalFinishReasonToAnthropic(resp.FinishReason),
		Usage: AnthropicUsage{
			InputTokens:          resp.Usage.PromptTokens,
			OutputTokens:         resp.Usage.CompletionTokens,
			CacheReadInputTokens: resp.Usage.CachedTokens,
		},
	}
}

func anthropicSystemToCanonicalParts(value any) ([]ChatPart, error) {
	switch system := value.(type) {
	case nil:
		return nil, nil
	case string:
		return []ChatPart{{Type: "text", Text: system}}, nil
	case []AnthropicContent:
		return anthropicContentToCanonical(system)
	case []any:
		raw, err := json.Marshal(system)
		if err != nil {
			return nil, err
		}
		var content []AnthropicContent
		if err := json.Unmarshal(raw, &content); err != nil {
			return nil, err
		}
		return anthropicContentToCanonical(content)
	default:
		return nil, errors.New("unsupported Anthropic system format")
	}
}

func anthropicContentToCanonical(content []AnthropicContent) ([]ChatPart, error) {
	out := make([]ChatPart, 0, len(content))
	for _, part := range content {
		switch part.Type {
		case "text":
			out = append(out, ChatPart{Type: "text", Text: part.Text})
		case "image":
			if part.Source == nil || part.Source.Type != "base64" {
				return nil, errors.New("only Anthropic base64 images are supported")
			}
			out = append(out, ChatPart{Type: "image", MimeType: part.Source.MediaType, Data: part.Source.Data})
		case "tool_use":
			args := part.Input
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			out = append(out, ChatPart{
				Type:       "tool_call",
				ToolCallID: part.ID,
				Name:       part.Name,
				Arguments:  args,
			})
		case "tool_result":
			raw, err := marshalJSONRaw(part.Content)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatPart{
				Type:       "tool_response",
				ToolCallID: part.ToolUseID,
				Response:   raw,
			})
		default:
			return nil, fmt.Errorf("unsupported Anthropic content type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalPartsToAnthropicContent(parts []ChatPart) ([]AnthropicContent, error) {
	out := make([]AnthropicContent, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, AnthropicContent{Type: "text", Text: part.Text})
		case "image":
			out = append(out, AnthropicContent{
				Type: "image",
				Source: &AnthropicSource{
					Type:      "base64",
					MediaType: part.MimeType,
					Data:      part.Data,
				},
			})
		case "tool_call":
			out = append(out, AnthropicContent{
				Type:  "tool_use",
				ID:    part.ToolCallID,
				Name:  part.Name,
				Input: part.Arguments,
			})
		case "tool_response":
			out = append(out, AnthropicContent{
				Type:      "tool_result",
				ToolUseID: part.ToolCallID,
				Content:   json.RawMessage(part.Response),
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalResponseToAnthropicContent(text string, toolCalls []ChatToolCall) []AnthropicContent {
	var out []AnthropicContent
	if text != "" {
		out = append(out, AnthropicContent{Type: "text", Text: text})
	}
	for _, call := range toolCalls {
		out = append(out, AnthropicContent{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Name,
			Input: json.RawMessage(call.Arguments),
		})
	}
	return out
}

func anthropicToolsToCanonical(tools []AnthropicTool) ([]ChatTool, error) {
	out := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("Anthropic tool name is required")
		}
		schema := tool.InputSchema
		if len(strings.TrimSpace(string(schema))) == 0 || strings.TrimSpace(string(schema)) == "null" {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, ChatTool{Name: tool.Name, Description: tool.Description, Parameters: schema})
	}
	return out, nil
}

func canonicalToolsToAnthropic(tools []ChatTool) ([]AnthropicTool, error) {
	out := make([]AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		schema := tool.Parameters
		if len(strings.TrimSpace(string(schema))) == 0 || strings.TrimSpace(string(schema)) == "null" {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, AnthropicTool{Name: tool.Name, Description: tool.Description, InputSchema: schema})
	}
	return out, nil
}

func anthropicToolChoiceToCanonical(choice *AnthropicToolChoice) *ChatToolChoice {
	if choice == nil {
		return nil
	}
	switch choice.Type {
	case "", "auto":
		return nil
	case "none":
		return &ChatToolChoice{Mode: "NONE"}
	case "any":
		return &ChatToolChoice{Mode: "ANY"}
	case "tool":
		return &ChatToolChoice{Mode: "ANY", AllowedFunctionNames: []string{choice.Name}}
	default:
		return &ChatToolChoice{Mode: strings.ToUpper(choice.Type)}
	}
}

func canonicalToolChoiceToAnthropic(choice *ChatToolChoice) *AnthropicToolChoice {
	if choice == nil {
		return nil
	}
	switch choice.Mode {
	case "NONE":
		return &AnthropicToolChoice{Type: "none"}
	case "ANY":
		if len(choice.AllowedFunctionNames) == 1 {
			return &AnthropicToolChoice{Type: "tool", Name: choice.AllowedFunctionNames[0]}
		}
		return &AnthropicToolChoice{Type: "any"}
	default:
		return &AnthropicToolChoice{Type: strings.ToLower(choice.Mode)}
	}
}

func anthropicRoleToCanonical(role string, parts []ChatPart) string {
	if allCanonicalPartsAreToolResponses(parts) {
		return "tool"
	}
	if role == "assistant" {
		return "assistant"
	}
	return "user"
}

func canonicalRoleToAnthropic(role string) string {
	if role == "assistant" {
		return "assistant"
	}
	return "user"
}

func anthropicStopReasonToCanonical(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return reason
	}
}

func canonicalFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "", "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return reason
	}
}
