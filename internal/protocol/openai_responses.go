package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
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
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type OpenAIResponseTextConfig struct {
	Format *OpenAIResponseTextFormat `json:"format,omitempty"`
}

type OpenAIResponseTextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
	Strict *bool           `json:"strict,omitempty"`
}

type OpenAIResponseInputItem struct {
	Type             string                     `json:"type,omitempty"`
	Role             string                     `json:"role,omitempty"`
	Content          json.RawMessage            `json:"content,omitempty"`
	Summary          []OpenAIResponseOutputPart `json:"summary,omitempty"`
	EncryptedContent string                     `json:"encrypted_content,omitempty"`
	ID               string                     `json:"id,omitempty"`
	CallID           string                     `json:"call_id,omitempty"`
	Name             string                     `json:"name,omitempty"`
	Arguments        string                     `json:"arguments,omitempty"`
	Output           string                     `json:"output,omitempty"`
}

type OpenAIResponseContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type OpenAIResponsesResponse struct {
	ID                string                            `json:"id"`
	Object            string                            `json:"object"`
	CreatedAt         int64                             `json:"created_at"`
	Status            string                            `json:"status"`
	IncompleteDetails *OpenAIResponsesIncompleteDetails `json:"incomplete_details,omitempty"`
	Error             *OpenAIResponsesError             `json:"error,omitempty"`
	Model             string                            `json:"model"`
	Output            []OpenAIResponseOutputItem        `json:"output"`
	Usage             *OpenAIResponsesUsage             `json:"usage,omitempty"`
}

type OpenAIResponsesIncompleteDetails struct {
	Reason string `json:"reason"`
}

type OpenAIResponsesError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Param   any    `json:"param,omitempty"`
}

type OpenAIResponseOutputItem struct {
	Type             string                     `json:"type"`
	ID               string                     `json:"id,omitempty"`
	Status           string                     `json:"status,omitempty"`
	Role             string                     `json:"role,omitempty"`
	Content          []OpenAIResponseOutputPart `json:"content,omitempty"`
	Summary          []OpenAIResponseOutputPart `json:"summary,omitempty"`
	EncryptedContent string                     `json:"encrypted_content,omitempty"`
	CallID           string                     `json:"call_id,omitempty"`
	Name             string                     `json:"name,omitempty"`
	Arguments        string                     `json:"arguments,omitempty"`
}

type OpenAIResponseOutputPart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

type OpenAIResponsesFunctionTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
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

func OpenAIResponsesToCanonicalRequest(req OpenAIResponsesRequest) (CanonicalRequest, error) {
	out := CanonicalRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
	}
	if req.Reasoning != nil && (strings.TrimSpace(req.Reasoning.Effort) != "" || strings.TrimSpace(req.Reasoning.Summary) != "") {
		out.Reasoning = &CanonicalReasoning{
			Effort:  req.Reasoning.Effort,
			Summary: req.Reasoning.Summary,
		}
	}
	if req.Text != nil && req.Text.Format != nil {
		var err error
		out.ResponseFormat, err = openAIResponsesTextFormatToCanonical(req.Text.Format)
		if err != nil {
			return CanonicalRequest{}, err
		}
	}
	tools, err := openAIResponsesToolsToCanonical(req.Tools)
	if err != nil {
		return CanonicalRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := openAIResponsesToolChoiceToCanonical(req.ToolChoice, tools)
	if err != nil {
		return CanonicalRequest{}, err
	}
	out.ToolChoice = toolChoice
	if strings.TrimSpace(req.Instructions) != "" {
		out.Messages = append(out.Messages, CanonicalMessage{
			Role:  "system",
			Parts: []CanonicalPart{{Type: "text", Text: req.Instructions}},
		})
	}
	messages, err := openAIResponsesInputToCanonical(req.Input)
	if err != nil {
		return CanonicalRequest{}, err
	}
	out.Messages = append(out.Messages, messages...)
	return out, nil
}

func CanonicalToOpenAIResponsesRequest(req CanonicalRequest) (OpenAIResponsesRequest, error) {
	if err := validateCanonicalToolChoice(req.ToolChoice, req.Tools); err != nil {
		return OpenAIResponsesRequest{}, err
	}
	if req.Reasoning != nil {
		if req.Reasoning.BudgetTokens != nil {
			return OpenAIResponsesRequest{}, fmt.Errorf("reasoning budget_tokens cannot be represented by OpenAI Responses")
		}
		mode := strings.ToLower(strings.TrimSpace(req.Reasoning.Mode))
		if mode != "" && mode != "adaptive" {
			return OpenAIResponsesRequest{}, fmt.Errorf("reasoning mode %q cannot be represented by OpenAI Responses", req.Reasoning.Mode)
		}
	}
	out := OpenAIResponsesRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
	}
	if req.Reasoning != nil && (strings.TrimSpace(req.Reasoning.Effort) != "" ||
		strings.TrimSpace(req.Reasoning.Summary) != "" ||
		(req.Reasoning.IncludeThoughts != nil && *req.Reasoning.IncludeThoughts)) {
		summary := req.Reasoning.Summary
		if summary == "" && req.Reasoning.IncludeThoughts != nil && *req.Reasoning.IncludeThoughts {
			summary = "auto"
		}
		out.Reasoning = &OpenAIReasoning{
			Effort:  strings.ToLower(req.Reasoning.Effort),
			Summary: summary,
		}
	}
	if req.ResponseFormat != nil {
		out.Text = &OpenAIResponseTextConfig{Format: canonicalResponseFormatToResponses(req.ResponseFormat)}
	}
	tools, err := canonicalToolsToOpenAIResponses(req.Tools)
	if err != nil {
		return OpenAIResponsesRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := canonicalToolChoiceToOpenAIResponses(req.ToolChoice)
	if err != nil {
		return OpenAIResponsesRequest{}, err
	}
	out.ToolChoice = toolChoice
	var items []OpenAIResponseInputItem
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Name) != "" {
			return OpenAIResponsesRequest{}, fmt.Errorf("message name cannot be represented by OpenAI Responses")
		}
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

func OpenAIResponsesResponseToCanonical(resp OpenAIResponsesResponse) (CanonicalResponse, error) {
	out := CanonicalResponse{
		ID:      resp.ID,
		Created: resp.CreatedAt,
		Model:   resp.Model,
		Role:    "assistant",
		Usage:   openAIResponsesUsageToCanonical(resp.Usage),
	}
	for _, item := range resp.Output {
		switch item.Type {
		case "reasoning":
			for _, part := range item.Summary {
				if part.Type == "summary_text" {
					out.Reasoning += part.Text
				}
			}
			for _, part := range item.Content {
				if part.Type == "reasoning_text" {
					out.Reasoning += part.Text
				}
			}
			if item.EncryptedContent != "" {
				out.Signature = item.EncryptedContent
			}
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					out.Text += part.Text
				case "refusal":
					out.Refusal += part.Refusal
				}
			}
		case "function_call":
			out.ToolCalls = append(out.ToolCalls, CanonicalToolCall{
				ID:        firstNonEmpty(item.CallID, item.ID),
				Name:      item.Name,
				Arguments: item.Arguments,
			})
		}
	}
	switch resp.Status {
	case "completed":
		if len(out.ToolCalls) > 0 {
			out.FinishReason = "tool_calls"
		} else {
			out.FinishReason = "stop"
		}
	case "incomplete":
		out.FinishReason = responsesIncompleteReasonToCanonical(resp.IncompleteDetails)
	case "failed", "cancelled":
		if resp.Error != nil {
			return CanonicalResponse{}, fmt.Errorf("Responses %s: %s", firstNonEmpty(resp.Error.Code, resp.Error.Type, resp.Status), resp.Error.Message)
		}
		return CanonicalResponse{}, fmt.Errorf("Responses response ended with status %q", resp.Status)
	case "":
		if len(out.ToolCalls) > 0 {
			out.FinishReason = "tool_calls"
		}
	default:
		return CanonicalResponse{}, fmt.Errorf("Responses response is not terminal: status %q", resp.Status)
	}
	return out, nil
}

func CanonicalToOpenAIResponsesResponse(resp CanonicalResponse) OpenAIResponsesResponse {
	out := OpenAIResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: resp.Created,
		Status:    canonicalFinishReasonToResponsesStatus(resp.FinishReason),
		Model:     resp.Model,
		Usage:     canonicalUsageToResponses(resp.Usage),
	}
	if out.Status == "incomplete" {
		out.IncompleteDetails = &OpenAIResponsesIncompleteDetails{
			Reason: canonicalFinishReasonToResponsesIncompleteReason(resp.FinishReason),
		}
	}
	if resp.Reasoning != "" || resp.Signature != "" {
		out.Output = append(out.Output, OpenAIResponseOutputItem{
			Type:             "reasoning",
			Status:           "completed",
			Summary:          []OpenAIResponseOutputPart{{Type: "summary_text", Text: resp.Reasoning}},
			EncryptedContent: resp.Signature,
		})
	}
	if resp.Text != "" || resp.Refusal != "" {
		part := OpenAIResponseOutputPart{Type: "output_text", Text: resp.Text}
		if resp.Refusal != "" {
			part = OpenAIResponseOutputPart{Type: "refusal", Refusal: resp.Refusal}
		}
		out.Output = append(out.Output, OpenAIResponseOutputItem{
			Type:    "message",
			Status:  canonicalFinishReasonToResponsesItemStatus(resp.FinishReason),
			Role:    resp.Role,
			Content: []OpenAIResponseOutputPart{part},
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

func openAIResponsesInputToCanonical(raw json.RawMessage) ([]CanonicalMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []CanonicalMessage{{Role: "user", Parts: []CanonicalPart{{Type: "text", Text: text}}}}, nil
	}
	var items []OpenAIResponseInputItem
	if err := decodeStrictJSON(raw, &items); err != nil {
		return nil, err
	}
	var out []CanonicalMessage
	for _, item := range items {
		switch item.Type {
		case "", "message":
			parts, err := openAIResponsesContentToCanonical(item.Content)
			if err != nil {
				return nil, err
			}
			out = append(out, CanonicalMessage{Role: item.Role, Parts: parts})
		case "function_call":
			args := json.RawMessage(item.Arguments)
			if len(bytes.TrimSpace(args)) == 0 {
				args = json.RawMessage(`{}`)
			}
			out = append(out, CanonicalMessage{Role: "assistant", Parts: []CanonicalPart{{
				Type:       "tool_call",
				ToolCallID: firstNonEmpty(item.CallID, item.ID),
				Name:       item.Name,
				Arguments:  args,
			}}})
		case "function_call_output":
			out = append(out, CanonicalMessage{Role: "tool", Parts: []CanonicalPart{{
				Type:       "tool_response",
				ToolCallID: item.CallID,
				Response:   json.RawMessage(quoteJSONString(item.Output)),
			}}})
		case "reasoning":
			parts := make([]CanonicalPart, 0, 1)
			reasoning := ""
			for _, part := range item.Summary {
				if part.Type == "summary_text" {
					reasoning += part.Text
				}
			}
			if reasoning != "" || item.EncryptedContent != "" {
				parts = append(parts, CanonicalPart{
					Type:      "reasoning",
					Text:      reasoning,
					Signature: item.EncryptedContent,
				})
			}
			if len(parts) > 0 {
				out = append(out, CanonicalMessage{Role: "assistant", Parts: parts})
			}
		default:
			return nil, fmt.Errorf("unsupported Responses input item type %q", item.Type)
		}
	}
	if err := populateCanonicalToolResponseNames(out); err != nil {
		return nil, err
	}
	return out, nil
}

func openAIResponsesContentToCanonical(raw json.RawMessage) ([]CanonicalPart, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []CanonicalPart{{Type: "text", Text: text}}, nil
	}
	var content []OpenAIResponseContentPart
	if err := decodeStrictJSON(raw, &content); err != nil {
		return nil, err
	}
	out := make([]CanonicalPart, 0, len(content))
	for _, part := range content {
		switch part.Type {
		case "input_text", "output_text":
			out = append(out, CanonicalPart{Type: "text", Text: part.Text})
		case "input_image":
			mimeType, data, err := parseOpenAIImageDataURL(part.ImageURL)
			if err != nil {
				return nil, err
			}
			out = append(out, CanonicalPart{Type: "image", MimeType: mimeType, Data: data})
		default:
			return nil, fmt.Errorf("unsupported Responses content type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalMessageToOpenAIResponsesInput(message CanonicalMessage) ([]OpenAIResponseInputItem, error) {
	var contentParts []CanonicalPart
	var before []OpenAIResponseInputItem
	var after []OpenAIResponseInputItem
	for _, part := range message.Parts {
		switch part.Type {
		case "text", "image":
			contentParts = append(contentParts, part)
		case "reasoning":
			before = append(before, OpenAIResponseInputItem{
				Type: "reasoning",
				Summary: []OpenAIResponseOutputPart{{
					Type: "summary_text",
					Text: part.Text,
				}},
				EncryptedContent: part.Signature,
			})
		case "tool_call":
			after = append(after, OpenAIResponseInputItem{
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
			after = append(after, OpenAIResponseInputItem{
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
		before = append(before, OpenAIResponseInputItem{
			Type:    "message",
			Role:    message.Role,
			Content: content,
		})
	}
	return append(before, after...), nil
}

func canonicalPartsToOpenAIResponsesContent(parts []CanonicalPart) (json.RawMessage, error) {
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

func canonicalText(parts []CanonicalPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Type == "text" {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func openAIResponsesToolsToCanonical(raw json.RawMessage) ([]CanonicalTool, error) {
	if !openAIToolsRequested(raw) {
		return nil, nil
	}
	var tools []OpenAIResponsesFunctionTool
	if err := decodeStrictJSON(raw, &tools); err != nil {
		return nil, errors.New("Responses tools must be an array of function tools")
	}
	out := make([]CanonicalTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("unsupported Responses tool type %q", tool.Type)
		}
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("Responses function tool name is required")
		}
		parameters := tool.Parameters
		if len(bytes.TrimSpace(parameters)) == 0 || bytes.Equal(bytes.TrimSpace(parameters), []byte("null")) {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, CanonicalTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
			Strict:      tool.Strict,
		})
	}
	return out, nil
}

func openAIResponsesToolChoiceToCanonical(raw json.RawMessage, tools []CanonicalTool) (*CanonicalToolChoice, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		switch text {
		case "", "auto":
			return nil, nil
		case "none":
			return &CanonicalToolChoice{Mode: "NONE"}, nil
		case "required":
			if len(tools) == 0 {
				return nil, errors.New("Responses tool_choice required needs at least one tool")
			}
			return &CanonicalToolChoice{Mode: "ANY"}, nil
		default:
			return nil, fmt.Errorf("unsupported Responses tool_choice %q", text)
		}
	}
	var choice struct {
		Type  string `json:"type"`
		Name  string `json:"name,omitempty"`
		Mode  string `json:"mode,omitempty"`
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name,omitempty"`
		} `json:"tools,omitempty"`
	}
	if err := decodeStrictJSON(raw, &choice); err != nil {
		return nil, errors.New("Responses tool_choice must be a string or tool choice object")
	}
	switch choice.Type {
	case "function":
		name := strings.TrimSpace(choice.Name)
		if name == "" {
			return nil, errors.New("Responses function tool_choice name is required")
		}
		if !canonicalToolExists(tools, name) {
			return nil, fmt.Errorf("Responses tool_choice references unknown function %q", name)
		}
		return &CanonicalToolChoice{Mode: "ANY", AllowedFunctionNames: []string{name}}, nil
	case "allowed_tools":
		if len(choice.Tools) == 0 {
			return nil, errors.New("Responses allowed_tools tool_choice requires tools")
		}
		names := make([]string, 0, len(choice.Tools))
		for _, tool := range choice.Tools {
			if tool.Type != "function" {
				return nil, fmt.Errorf("unsupported Responses allowed tool type %q", tool.Type)
			}
			name := strings.TrimSpace(tool.Name)
			if name == "" || !canonicalToolExists(tools, name) {
				return nil, fmt.Errorf("Responses allowed_tools references unknown function %q", name)
			}
			names = append(names, name)
		}
		if choice.Mode != "auto" && choice.Mode != "required" {
			return nil, fmt.Errorf("unsupported Responses allowed_tools mode %q", choice.Mode)
		}
		mode := "ANY"
		if choice.Mode == "auto" {
			mode = "AUTO"
		}
		return &CanonicalToolChoice{Mode: mode, AllowedFunctionNames: names}, nil
	default:
		return nil, fmt.Errorf("unsupported Responses tool_choice type %q", choice.Type)
	}
}

func canonicalToolsToOpenAIResponses(tools []CanonicalTool) (json.RawMessage, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]OpenAIResponsesFunctionTool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		parameters := tool.Parameters
		if len(bytes.TrimSpace(parameters)) == 0 || bytes.Equal(bytes.TrimSpace(parameters), []byte("null")) {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, OpenAIResponsesFunctionTool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
			Strict:      tool.Strict,
		})
	}
	return marshalJSONRaw(out)
}

func canonicalToolChoiceToOpenAIResponses(choice *CanonicalToolChoice) (json.RawMessage, error) {
	if choice == nil || strings.TrimSpace(choice.Mode) == "" {
		return nil, nil
	}
	switch choice.Mode {
	case "NONE":
		return marshalJSONRaw("none")
	case "AUTO", "ANY":
		if len(choice.AllowedFunctionNames) == 0 {
			if choice.Mode == "AUTO" {
				return marshalJSONRaw("auto")
			}
			return marshalJSONRaw("required")
		}
		if len(choice.AllowedFunctionNames) == 1 && choice.Mode == "ANY" {
			return marshalJSONRaw(map[string]any{
				"type": "function",
				"name": choice.AllowedFunctionNames[0],
			})
		}
		tools := make([]map[string]string, 0, len(choice.AllowedFunctionNames))
		for _, name := range choice.AllowedFunctionNames {
			tools = append(tools, map[string]string{"type": "function", "name": name})
		}
		mode := "required"
		if choice.Mode == "AUTO" {
			mode = "auto"
		}
		return marshalJSONRaw(map[string]any{
			"type":  "allowed_tools",
			"mode":  mode,
			"tools": tools,
		})
	default:
		return nil, fmt.Errorf("unsupported canonical tool choice mode %q", choice.Mode)
	}
}

func openAIResponsesTextFormatToCanonical(format *OpenAIResponseTextFormat) (*CanonicalResponseFormat, error) {
	if format == nil || format.Type == "" || format.Type == "text" {
		return nil, nil
	}
	if format.Type == "json_schema" {
		if len(bytes.TrimSpace(format.Schema)) == 0 {
			return nil, fmt.Errorf("Responses text.format.schema is required for json_schema")
		}
		return &CanonicalResponseFormat{
			MimeType: "application/json",
			Name:     format.Name,
			Schema:   format.Schema,
			Strict:   format.Strict,
		}, nil
	}
	if format.Type == "json_object" {
		return &CanonicalResponseFormat{MimeType: "application/json"}, nil
	}
	return nil, fmt.Errorf("unsupported Responses text format type %q", format.Type)
}

func canonicalResponseFormatToResponses(format *CanonicalResponseFormat) *OpenAIResponseTextFormat {
	if format == nil {
		return nil
	}
	if len(bytes.TrimSpace(format.Schema)) > 0 {
		return &OpenAIResponseTextFormat{
			Type:   "json_schema",
			Name:   firstNonEmpty(format.Name, "canonical_schema"),
			Schema: format.Schema,
			Strict: format.Strict,
		}
	}
	if format.MimeType == "application/json" {
		return &OpenAIResponseTextFormat{Type: "json_object"}
	}
	return &OpenAIResponseTextFormat{Type: format.MimeType}
}

func openAIResponsesUsageToCanonical(usage *OpenAIResponsesUsage) CanonicalUsage {
	if usage == nil {
		return CanonicalUsage{}
	}
	return CanonicalUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CachedTokens:     usage.InputTokensDetails.CachedTokens,
		ReasoningTokens:  usage.OutputTokensDetails.ReasoningTokens,
	}
}

func canonicalUsageToResponses(usage CanonicalUsage) *OpenAIResponsesUsage {
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
	case "", "stop", "tool_calls":
		return "completed"
	case "length", "content_filter":
		return "incomplete"
	default:
		return "completed"
	}
}

func canonicalFinishReasonToResponsesItemStatus(reason string) string {
	if canonicalFinishReasonToResponsesStatus(reason) == "incomplete" {
		return "incomplete"
	}
	return "completed"
}

func canonicalFinishReasonToResponsesIncompleteReason(reason string) string {
	if reason == "content_filter" {
		return "content_filter"
	}
	return "max_output_tokens"
}

func responsesIncompleteReasonToCanonical(details *OpenAIResponsesIncompleteDetails) string {
	if details != nil && details.Reason == "content_filter" {
		return "content_filter"
	}
	return "length"
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
