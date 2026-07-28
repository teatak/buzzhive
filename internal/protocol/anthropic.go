package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type AnthropicMessagesRequest struct {
	Model         string                 `json:"model"`
	System        any                    `json:"system,omitempty"`
	Messages      []AnthropicMessage     `json:"messages"`
	MaxTokens     *int                   `json:"max_tokens,omitempty"`
	Temperature   *float64               `json:"temperature,omitempty"`
	TopP          *float64               `json:"top_p,omitempty"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool        `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice   `json:"tool_choice,omitempty"`
	Thinking      *AnthropicThinking     `json:"thinking,omitempty"`
	OutputConfig  *AnthropicOutputConfig `json:"output_config,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
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
	Thinking  string           `json:"thinking,omitempty"`
	Signature string           `json:"signature,omitempty"`
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
	Type         string `json:"type"`
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type AnthropicOutputConfig struct {
	Effort string                     `json:"effort,omitempty"`
	Format *AnthropicJSONOutputFormat `json:"format,omitempty"`
}

type AnthropicJSONOutputFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

type AnthropicMessagesResponse struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"`
	Role         string                `json:"role"`
	Model        string                `json:"model"`
	Content      []AnthropicContent    `json:"content"`
	StopReason   string                `json:"stop_reason,omitempty"`
	StopSequence string                `json:"stop_sequence,omitempty"`
	StopDetails  *AnthropicStopDetails `json:"stop_details,omitempty"`
	Usage        AnthropicUsage        `json:"usage,omitempty"`
}

type AnthropicStopDetails struct {
	Type        string `json:"type"`
	Category    string `json:"category,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

type AnthropicUsage struct {
	InputTokens              int                           `json:"input_tokens,omitempty"`
	OutputTokens             int                           `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int                           `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                           `json:"cache_read_input_tokens,omitempty"`
	OutputTokensDetails      *AnthropicOutputTokensDetails `json:"output_tokens_details,omitempty"`
}

type AnthropicOutputTokensDetails struct {
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
}

func AnthropicMessagesToCanonicalRequest(req AnthropicMessagesRequest) (CanonicalRequest, error) {
	if req.MaxTokens == nil {
		return CanonicalRequest{}, errors.New("Anthropic max_tokens is required")
	}
	if *req.MaxTokens < 0 {
		return CanonicalRequest{}, errors.New("Anthropic max_tokens must not be negative")
	}
	if len(req.Messages) == 0 {
		return CanonicalRequest{}, errors.New("Anthropic messages must contain at least one message")
	}
	out := CanonicalRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxTokens,
		StopSequences:   req.StopSequences,
	}
	if req.Thinking != nil || (req.OutputConfig != nil && strings.TrimSpace(req.OutputConfig.Effort) != "") {
		out.Reasoning = &CanonicalReasoning{}
		if req.Thinking != nil {
			out.Reasoning.Mode = req.Thinking.Type
			out.Reasoning.BudgetTokens = req.Thinking.BudgetTokens
			switch req.Thinking.Display {
			case "summarized":
				value := true
				out.Reasoning.IncludeThoughts = &value
			case "omitted":
				value := false
				out.Reasoning.IncludeThoughts = &value
			}
		}
		if req.OutputConfig != nil {
			out.Reasoning.Effort = req.OutputConfig.Effort
		}
	}
	if req.OutputConfig != nil && req.OutputConfig.Format != nil {
		if req.OutputConfig.Format.Type != "json_schema" {
			return CanonicalRequest{}, fmt.Errorf("unsupported Anthropic output format type %q", req.OutputConfig.Format.Type)
		}
		if len(strings.TrimSpace(string(req.OutputConfig.Format.Schema))) == 0 {
			return CanonicalRequest{}, errors.New("Anthropic output_config.format.schema is required")
		}
		out.ResponseFormat = &CanonicalResponseFormat{
			MimeType: "application/json",
			Schema:   req.OutputConfig.Format.Schema,
		}
	}
	tools, err := anthropicToolsToCanonical(req.Tools)
	if err != nil {
		return CanonicalRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice = anthropicToolChoiceToCanonical(req.ToolChoice)
	if err := validateCanonicalToolChoice(out.ToolChoice, out.Tools); err != nil {
		return CanonicalRequest{}, err
	}
	systemParts, err := anthropicSystemToCanonicalParts(req.System)
	if err != nil {
		return CanonicalRequest{}, err
	}
	if !canonicalPartsEmpty(systemParts) {
		out.Messages = append(out.Messages, CanonicalMessage{Role: "system", Parts: systemParts})
	}
	for _, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return CanonicalRequest{}, fmt.Errorf("unsupported Anthropic message role %q", message.Role)
		}
		parts, err := anthropicContentToCanonical(message.Content)
		if err != nil {
			return CanonicalRequest{}, err
		}
		if canonicalPartsEmpty(parts) {
			continue
		}
		out.Messages = append(out.Messages, CanonicalMessage{
			Role:  anthropicRoleToCanonical(message.Role, parts),
			Parts: parts,
		})
	}
	if err := populateCanonicalToolResponseNames(out.Messages); err != nil {
		return CanonicalRequest{}, err
	}
	return out, nil
}

func CanonicalToAnthropicMessagesRequest(req CanonicalRequest) (AnthropicMessagesRequest, error) {
	if req.MaxOutputTokens == nil || *req.MaxOutputTokens < 0 {
		return AnthropicMessagesRequest{}, errors.New("max_output_tokens is required and must not be negative when converting to Anthropic Messages")
	}
	if err := validateCanonicalToolChoice(req.ToolChoice, req.Tools); err != nil {
		return AnthropicMessagesRequest{}, err
	}
	out := AnthropicMessagesRequest{
		Model:         req.Model,
		MaxTokens:     req.MaxOutputTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
		Stream:        req.Stream,
	}
	if req.Reasoning != nil {
		mode := strings.ToLower(strings.TrimSpace(req.Reasoning.Mode))
		if req.Reasoning.BudgetTokens != nil {
			if mode == "" {
				mode = "enabled"
			}
			if mode != "enabled" {
				return AnthropicMessagesRequest{}, errors.New("Anthropic budget_tokens requires reasoning mode enabled")
			}
		}
		switch mode {
		case "", "adaptive", "enabled", "disabled":
		default:
			return AnthropicMessagesRequest{}, fmt.Errorf("unsupported Anthropic reasoning mode %q", req.Reasoning.Mode)
		}
		summary := strings.ToLower(strings.TrimSpace(req.Reasoning.Summary))
		if summary != "" && summary != "auto" {
			return AnthropicMessagesRequest{}, fmt.Errorf("reasoning summary %q cannot be represented by Anthropic Messages", req.Reasoning.Summary)
		}
		effort := strings.ToLower(strings.TrimSpace(req.Reasoning.Effort))
		switch effort {
		case "", "low", "medium", "high", "xhigh", "max":
		default:
			return AnthropicMessagesRequest{}, fmt.Errorf("reasoning effort %q cannot be represented by Anthropic Messages", req.Reasoning.Effort)
		}
		if mode != "" || req.Reasoning.BudgetTokens != nil || req.Reasoning.IncludeThoughts != nil || summary == "auto" {
			if mode == "" {
				mode = "adaptive"
			}
			out.Thinking = &AnthropicThinking{
				Type:         mode,
				BudgetTokens: req.Reasoning.BudgetTokens,
			}
			if req.Reasoning.IncludeThoughts != nil {
				if *req.Reasoning.IncludeThoughts {
					out.Thinking.Display = "summarized"
				} else {
					out.Thinking.Display = "omitted"
				}
			} else if summary == "auto" {
				out.Thinking.Display = "summarized"
			}
		}
		if effort != "" {
			out.OutputConfig = &AnthropicOutputConfig{Effort: effort}
		}
	}
	if req.ResponseFormat != nil {
		if strings.TrimSpace(req.ResponseFormat.MimeType) != "application/json" {
			return AnthropicMessagesRequest{}, fmt.Errorf("response mime type %q cannot be represented by Anthropic Messages", req.ResponseFormat.MimeType)
		}
		if len(strings.TrimSpace(string(req.ResponseFormat.Schema))) == 0 {
			return AnthropicMessagesRequest{}, errors.New("Anthropic structured output requires a JSON schema")
		}
		if out.OutputConfig == nil {
			out.OutputConfig = &AnthropicOutputConfig{}
		}
		out.OutputConfig.Format = &AnthropicJSONOutputFormat{
			Type:   "json_schema",
			Schema: req.ResponseFormat.Schema,
		}
	}
	tools, err := canonicalToolsToAnthropic(req.Tools)
	if err != nil {
		return AnthropicMessagesRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice, err = canonicalToolChoiceToAnthropic(req.ToolChoice)
	if err != nil {
		return AnthropicMessagesRequest{}, err
	}
	if err := populateCanonicalToolResponseNames(req.Messages); err != nil {
		return AnthropicMessagesRequest{}, err
	}
	var systemContent []AnthropicContent
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Name) != "" {
			return AnthropicMessagesRequest{}, errors.New("message name cannot be represented by Anthropic Messages")
		}
		content, err := canonicalPartsToAnthropicContent(message.Parts)
		if err != nil {
			return AnthropicMessagesRequest{}, err
		}
		if len(content) == 0 {
			continue
		}
		if message.Role == "system" || message.Role == "developer" {
			systemContent = append(systemContent, content...)
			continue
		}
		out.Messages = append(out.Messages, AnthropicMessage{
			Role:    canonicalRoleToAnthropic(message.Role),
			Content: content,
		})
	}
	if len(systemContent) > 0 {
		out.System = systemContent
	}
	return out, nil
}

func AnthropicMessagesResponseToCanonical(resp AnthropicMessagesResponse) (CanonicalResponse, error) {
	parts, err := anthropicContentToCanonical(resp.Content)
	if err != nil {
		return CanonicalResponse{}, err
	}
	promptTokens := resp.Usage.InputTokens +
		resp.Usage.CacheCreationInputTokens +
		resp.Usage.CacheReadInputTokens
	out := CanonicalResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Role:         anthropicRoleToCanonical(resp.Role, parts),
		FinishReason: AnthropicStopReasonToCanonical(resp.StopReason),
		Usage: CanonicalUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      promptTokens + resp.Usage.OutputTokens,
			CachedTokens:     resp.Usage.CacheReadInputTokens,
		},
	}
	if resp.StopReason == "refusal" {
		out.Refusal = "Anthropic refused the request"
		if resp.StopDetails != nil {
			out.Refusal = firstNonEmpty(resp.StopDetails.Explanation, resp.StopDetails.Category, out.Refusal)
		}
	}
	if resp.Usage.OutputTokensDetails != nil {
		out.Usage.ReasoningTokens = resp.Usage.OutputTokensDetails.ThinkingTokens
	}
	for _, part := range parts {
		switch part.Type {
		case "text":
			out.Text += part.Text
		case "reasoning":
			out.Reasoning += part.Text
			out.Signature = part.Signature
		case "tool_call":
			out.ToolCalls = append(out.ToolCalls, CanonicalToolCall{
				ID:        part.ToolCallID,
				Name:      part.Name,
				Arguments: string(part.Arguments),
				Signature: part.Signature,
			})
		}
	}
	if len(out.ToolCalls) > 0 && (resp.StopReason == "" || resp.StopReason == "tool_use") {
		out.FinishReason = "tool_calls"
	}
	return out, nil
}

func CanonicalToAnthropicMessagesResponse(resp CanonicalResponse) AnthropicMessagesResponse {
	inputTokens := resp.Usage.PromptTokens - resp.Usage.CachedTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	out := AnthropicMessagesResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       canonicalRoleToAnthropic(resp.Role),
		Model:      resp.Model,
		Content:    canonicalResponseToAnthropicContent(resp.Reasoning, resp.Signature, resp.Text, resp.ToolCalls),
		StopReason: CanonicalFinishReasonToAnthropic(resp.FinishReason),
		Usage: AnthropicUsage{
			InputTokens:          inputTokens,
			OutputTokens:         resp.Usage.CompletionTokens,
			CacheReadInputTokens: resp.Usage.CachedTokens,
			OutputTokensDetails: &AnthropicOutputTokensDetails{
				ThinkingTokens: resp.Usage.ReasoningTokens,
			},
		},
	}
	if resp.Refusal != "" {
		out.Content = nil
		out.StopReason = "refusal"
		out.StopDetails = &AnthropicStopDetails{
			Type:        "refusal",
			Explanation: resp.Refusal,
		}
	}
	return out
}

func anthropicSystemToCanonicalParts(value any) ([]CanonicalPart, error) {
	switch system := value.(type) {
	case nil:
		return nil, nil
	case string:
		return []CanonicalPart{{Type: "text", Text: system}}, nil
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

func anthropicContentToCanonical(content []AnthropicContent) ([]CanonicalPart, error) {
	out := make([]CanonicalPart, 0, len(content))
	for _, part := range content {
		switch part.Type {
		case "text":
			out = append(out, CanonicalPart{Type: "text", Text: part.Text})
		case "thinking":
			out = append(out, CanonicalPart{
				Type:      "reasoning",
				Text:      part.Thinking,
				Signature: part.Signature,
			})
		case "image":
			if part.Source == nil || part.Source.Type != "base64" {
				return nil, errors.New("only Anthropic base64 images are supported")
			}
			out = append(out, CanonicalPart{Type: "image", MimeType: part.Source.MediaType, Data: part.Source.Data})
		case "tool_use":
			args := part.Input
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			out = append(out, CanonicalPart{
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
			out = append(out, CanonicalPart{
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

func canonicalPartsToAnthropicContent(parts []CanonicalPart) ([]AnthropicContent, error) {
	out := make([]AnthropicContent, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, AnthropicContent{Type: "text", Text: part.Text})
		case "reasoning":
			out = append(out, AnthropicContent{
				Type:      "thinking",
				Thinking:  part.Text,
				Signature: part.Signature,
			})
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
			content, err := canonicalToolResponseToAnthropic(part.Response)
			if err != nil {
				return nil, err
			}
			out = append(out, AnthropicContent{
				Type:      "tool_result",
				ToolUseID: part.ToolCallID,
				Content:   content,
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalToolResponseToAnthropic(raw json.RawMessage) (string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	if !json.Valid(raw) {
		return "", errors.New("tool response must contain valid JSON")
	}
	return value, nil
}

func canonicalResponseToAnthropicContent(reasoning string, signature string, text string, toolCalls []CanonicalToolCall) []AnthropicContent {
	var out []AnthropicContent
	if reasoning != "" || signature != "" {
		out = append(out, AnthropicContent{
			Type:      "thinking",
			Thinking:  reasoning,
			Signature: signature,
		})
	}
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

func anthropicToolsToCanonical(tools []AnthropicTool) ([]CanonicalTool, error) {
	out := make([]CanonicalTool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("Anthropic tool name is required")
		}
		schema := tool.InputSchema
		if len(strings.TrimSpace(string(schema))) == 0 || strings.TrimSpace(string(schema)) == "null" {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, CanonicalTool{Name: tool.Name, Description: tool.Description, Parameters: schema})
	}
	return out, nil
}

func canonicalToolsToAnthropic(tools []CanonicalTool) ([]AnthropicTool, error) {
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

func anthropicToolChoiceToCanonical(choice *AnthropicToolChoice) *CanonicalToolChoice {
	if choice == nil {
		return nil
	}
	switch choice.Type {
	case "", "auto":
		return nil
	case "none":
		return &CanonicalToolChoice{Mode: "NONE"}
	case "any":
		return &CanonicalToolChoice{Mode: "ANY"}
	case "tool":
		return &CanonicalToolChoice{Mode: "ANY", AllowedFunctionNames: []string{choice.Name}}
	default:
		return &CanonicalToolChoice{Mode: strings.ToUpper(choice.Type)}
	}
}

func canonicalToolChoiceToAnthropic(choice *CanonicalToolChoice) (*AnthropicToolChoice, error) {
	if choice == nil {
		return nil, nil
	}
	switch choice.Mode {
	case "NONE":
		return &AnthropicToolChoice{Type: "none"}, nil
	case "AUTO":
		if len(choice.AllowedFunctionNames) > 0 {
			return nil, errors.New("Anthropic Messages cannot restrict automatic tool choice to a function subset")
		}
		return &AnthropicToolChoice{Type: "auto"}, nil
	case "ANY":
		if len(choice.AllowedFunctionNames) == 1 {
			return &AnthropicToolChoice{Type: "tool", Name: choice.AllowedFunctionNames[0]}, nil
		}
		if len(choice.AllowedFunctionNames) > 1 {
			return nil, errors.New("Anthropic Messages cannot require a function subset with more than one function")
		}
		return &AnthropicToolChoice{Type: "any"}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical tool choice mode %q", choice.Mode)
	}
}

func anthropicRoleToCanonical(role string, parts []CanonicalPart) string {
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

func AnthropicStopReasonToCanonical(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence", "pause_turn":
		return "stop"
	case "max_tokens", "model_context_window_exceeded":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "refusal":
		return "content_filter"
	default:
		return reason
	}
}

func CanonicalFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "", "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "refusal"
	default:
		return reason
	}
}
