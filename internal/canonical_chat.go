package buzzhive

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type canonicalChatRequest struct {
	Model           string
	Messages        []canonicalMessage
	Stream          bool
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	StopSequences   []string
	Tools           []canonicalTool
	ToolChoice      *canonicalToolChoice
}

type canonicalMessage struct {
	Role  string
	Parts []canonicalPart
}

type canonicalPart struct {
	Type       string
	Text       string
	MimeType   string
	Data       string
	ToolCallID string
	Name       string
	Arguments  json.RawMessage
	Response   json.RawMessage
	Signature  string
}

type canonicalTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type canonicalToolChoice struct {
	Mode                 string
	AllowedFunctionNames []string
}

type canonicalToolCall struct {
	ID        string
	Name      string
	Arguments string
	Signature string
}

type canonicalUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type canonicalChatResponse struct {
	ID           string
	Created      int64
	Model        string
	Role         string
	Text         string
	ToolCalls    []canonicalToolCall
	FinishReason string
	Usage        canonicalUsage
}

type canonicalStreamEvent struct {
	Text         string
	ToolCalls    []canonicalToolCall
	FinishReason string
}

func openAIToCanonicalChatRequest(req openAIChatRequest) (canonicalChatRequest, error) {
	maxTokens := req.MaxTokens
	if maxTokens == nil {
		maxTokens = req.MaxOutputTokens
	}
	out := canonicalChatRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: maxTokens,
		StopSequences:   openAIStopSequences(req.Stop),
	}
	tools, err := openAIToolsToCanonical(req.Tools)
	if err != nil {
		return canonicalChatRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := openAIToolChoiceToCanonical(req.ToolChoice, tools)
	if err != nil {
		return canonicalChatRequest{}, err
	}
	out.ToolChoice = toolChoice
	hasConversationMessage := false
	toolCallNames := make(map[string]string)
	for _, message := range req.Messages {
		switch message.Role {
		case "system", "developer", "assistant", "user":
			parts, err := openAIMessageToCanonicalParts(message, toolCallNames)
			if err != nil {
				return canonicalChatRequest{}, err
			}
			if canonicalPartsEmpty(parts) {
				continue
			}
			out.Messages = append(out.Messages, canonicalMessage{
				Role:  message.Role,
				Parts: parts,
			})
			if message.Role == "assistant" || message.Role == "user" {
				hasConversationMessage = true
			}
		case "tool":
			parts, err := openAIToolMessageToCanonicalParts(message, toolCallNames)
			if err != nil {
				return canonicalChatRequest{}, err
			}
			if canonicalPartsEmpty(parts) {
				continue
			}
			out.Messages = append(out.Messages, canonicalMessage{
				Role:  message.Role,
				Parts: parts,
			})
		default:
			return canonicalChatRequest{}, fmt.Errorf("unsupported message role %q", message.Role)
		}
	}
	if !hasConversationMessage {
		return canonicalChatRequest{}, errors.New("messages must contain at least one user or assistant message")
	}
	return out, nil
}

func canonicalPartsEmpty(parts []canonicalPart) bool {
	for _, part := range parts {
		switch part.Type {
		case "text":
			if strings.TrimSpace(part.Text) != "" {
				return false
			}
		case "image":
			if part.Data != "" {
				return false
			}
		case "tool_call", "tool_response":
			if strings.TrimSpace(part.Name) != "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func canonicalToGeminiGenerateRequest(req canonicalChatRequest) (geminiGenerateRequest, error) {
	out := geminiGenerateRequest{
		GenerationConfig: canonicalToGeminiGenerationConfig(req),
		ToolConfig:       canonicalToolChoiceToGeminiToolConfig(req.ToolChoice),
	}
	tools, err := canonicalToolsToGeminiTools(req.Tools)
	if err != nil {
		return geminiGenerateRequest{}, err
	}
	out.Tools = tools
	var systemParts []geminiPart
	for _, message := range req.Messages {
		parts, err := canonicalPartsToGeminiParts(message.Parts)
		if err != nil {
			return geminiGenerateRequest{}, err
		}
		switch message.Role {
		case "system", "developer":
			systemParts = append(systemParts, parts...)
		case "assistant":
			out.Contents = append(out.Contents, geminiContent{Role: "model", Parts: parts})
		case "tool":
			out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: parts})
		case "user":
			out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: parts})
		default:
			return geminiGenerateRequest{}, fmt.Errorf("unsupported canonical message role %q", message.Role)
		}
	}
	if len(systemParts) > 0 {
		out.SystemInstruction = &geminiContent{Parts: systemParts}
	}
	if len(out.Contents) == 0 {
		return geminiGenerateRequest{}, errors.New("messages must contain at least one user or assistant message")
	}
	return out, nil
}

func openAIMessageToCanonicalParts(message openAIMessage, toolCallNames map[string]string) ([]canonicalPart, error) {
	parts, err := openAIMessageParts(message.Content)
	if err != nil {
		return nil, err
	}
	if message.Role != "assistant" || len(message.ToolCalls) == 0 {
		return parts, nil
	}
	for _, toolCall := range message.ToolCalls {
		if toolCall.Type != "function" {
			return nil, fmt.Errorf("unsupported tool call type %q", toolCall.Type)
		}
		if strings.TrimSpace(toolCall.ID) == "" {
			return nil, errors.New("tool call id is required")
		}
		if strings.TrimSpace(toolCall.Function.Name) == "" {
			return nil, errors.New("tool call function name is required")
		}
		args, err := normalizeOpenAIToolCallArguments(toolCall.Function.Arguments)
		if err != nil {
			return nil, err
		}
		toolCallNames[toolCall.ID] = toolCall.Function.Name
		parts = append(parts, canonicalPart{
			Type:       "tool_call",
			ToolCallID: toolCall.ID,
			Name:       toolCall.Function.Name,
			Arguments:  args,
		})
	}
	return parts, nil
}

func openAIToolMessageToCanonicalParts(message openAIMessage, toolCallNames map[string]string) ([]canonicalPart, error) {
	if strings.TrimSpace(message.ToolCallID) == "" {
		return nil, errors.New("tool message tool_call_id is required")
	}
	name := toolCallNames[message.ToolCallID]
	if name == "" {
		return nil, fmt.Errorf("tool message references unknown tool_call_id %q", message.ToolCallID)
	}
	response, err := openAIToolContentToGeminiResponse(message.Content)
	if err != nil {
		return nil, err
	}
	return []canonicalPart{{
		Type:       "tool_response",
		ToolCallID: message.ToolCallID,
		Name:       name,
		Response:   response,
	}}, nil
}

func normalizeOpenAIToolCallArguments(value string) (json.RawMessage, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return json.RawMessage(`{}`), nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err != nil {
		return nil, errors.New("tool call function arguments must be a JSON object")
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func openAIToolContentToGeminiResponse(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return json.RawMessage(`{"result":null}`), nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if object := jsonObjectFromText(text); object != nil {
			return object, nil
		}
		return marshalJSONRaw(map[string]any{"result": text})
	}
	if object := jsonObjectFromText(string(raw)); object != nil {
		return object, nil
	}
	return nil, errors.New("tool message content must be a string or JSON object")
}

func jsonObjectFromText(value string) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err != nil {
		return nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return json.RawMessage(raw)
}

func marshalJSONRaw(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func openAIToolsToCanonical(raw json.RawMessage) ([]canonicalTool, error) {
	if !openAIToolsRequested(raw) {
		return nil, nil
	}
	var tools []openAITool
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, errors.New("tools must be an array")
	}
	out := make([]canonicalTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("unsupported tool type %q", tool.Type)
		}
		if strings.TrimSpace(tool.Function.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		parameters := tool.Function.Parameters
		if len(parameters) == 0 || strings.TrimSpace(string(parameters)) == "null" {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, canonicalTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  parameters,
		})
	}
	return out, nil
}

func openAIToolsRequested(raw json.RawMessage) bool {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return len(items) > 0
	}
	return true
}

func openAIToolChoiceToCanonical(raw json.RawMessage, tools []canonicalTool) (*canonicalToolChoice, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		switch text {
		case "", "auto":
			return nil, nil
		case "none":
			return &canonicalToolChoice{Mode: "NONE"}, nil
		case "required":
			if len(tools) == 0 {
				return nil, errors.New("tool_choice required needs at least one tool")
			}
			return &canonicalToolChoice{Mode: "ANY"}, nil
		default:
			return nil, fmt.Errorf("unsupported tool_choice %q", text)
		}
	}
	var choice struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, errors.New("tool_choice must be a string or function object")
	}
	if choice.Type != "function" {
		return nil, fmt.Errorf("unsupported tool_choice type %q", choice.Type)
	}
	name := strings.TrimSpace(choice.Function.Name)
	if name == "" {
		return nil, errors.New("tool_choice function name is required")
	}
	if !canonicalToolExists(tools, name) {
		return nil, fmt.Errorf("tool_choice references unknown function %q", name)
	}
	return &canonicalToolChoice{Mode: "ANY", AllowedFunctionNames: []string{name}}, nil
}

func canonicalToolExists(tools []canonicalTool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIFunctionTool `json:"function"`
}

type openAIFunctionTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func canonicalToolsToGeminiTools(tools []canonicalTool) ([]geminiTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	declarations := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}
	return []geminiTool{{FunctionDeclarations: declarations}}, nil
}

func canonicalToolChoiceToGeminiToolConfig(choice *canonicalToolChoice) *geminiToolConfig {
	if choice == nil {
		return nil
	}
	return &geminiToolConfig{
		FunctionCallingConfig: &geminiFunctionCallingConfig{
			Mode:                 choice.Mode,
			AllowedFunctionNames: choice.AllowedFunctionNames,
		},
	}
}

func canonicalPartsToGeminiParts(parts []canonicalPart) ([]geminiPart, error) {
	out := make([]geminiPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, geminiPart{Text: part.Text})
		case "image":
			out = append(out, geminiPart{
				InlineData: &geminiInlineData{
					MimeType: part.MimeType,
					Data:     part.Data,
				},
			})
		case "tool_call":
			out = append(out, geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: part.Name,
					Args: part.Arguments,
				},
				ThoughtSignature: part.Signature,
			})
		case "tool_response":
			out = append(out, geminiPart{
				FunctionResponse: &geminiFunctionResponse{
					Name:     part.Name,
					Response: part.Response,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalToGeminiGenerationConfig(req canonicalChatRequest) *geminiGenerationConfig {
	cfg := &geminiGenerationConfig{
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
		StopSequences:   req.StopSequences,
	}
	if cfg.Temperature == nil && cfg.TopP == nil && cfg.MaxOutputTokens == nil && len(cfg.StopSequences) == 0 {
		return nil
	}
	return cfg
}

func geminiToCanonicalChatResponse(resp geminiGenerateResponse, model, requestID string, startedAt time.Time) canonicalChatResponse {
	toolCalls := geminiResponseToolCalls(resp, requestID)
	finishReason := openAIFinishReason(geminiFinishReason(resp))
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return canonicalChatResponse{
		ID:           "chatcmpl-" + requestID,
		Created:      startedAt.Unix(),
		Model:        model,
		Role:         "assistant",
		Text:         geminiResponseText(resp),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage: canonicalUsage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}
}

func canonicalToOpenAIChatResponse(resp canonicalChatResponse) openAIChatResponse {
	finishReason := resp.FinishReason
	content := resp.Text
	message := &openAIMessageOut{Role: resp.Role, Content: &content}
	if len(resp.ToolCalls) > 0 {
		message.Content = nil
		message.ToolCalls = canonicalToolCallsToOpenAIToolCalls(resp.ToolCalls)
	}
	return openAIChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.Created,
		Model:   resp.Model,
		Choices: []openAIChoice{{
			Index:        0,
			Message:      message,
			FinishReason: &finishReason,
		}},
		Usage: openAIUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func geminiResponseToolCalls(resp geminiGenerateResponse, requestID string) []canonicalToolCall {
	if len(resp.Candidates) == 0 {
		return nil
	}
	var out []canonicalToolCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
			continue
		}
		args := "{}"
		if len(part.FunctionCall.Args) > 0 && string(part.FunctionCall.Args) != "null" {
			args = string(part.FunctionCall.Args)
		}
		out = append(out, canonicalToolCall{
			ID:        fmt.Sprintf("call_%s_%d", requestID, len(out)),
			Name:      part.FunctionCall.Name,
			Arguments: args,
			Signature: part.ThoughtSignature,
		})
	}
	return out
}

func canonicalToolCallsToOpenAIToolCalls(toolCalls []canonicalToolCall) []openAIToolCall {
	out := make([]openAIToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		out = append(out, openAIToolCall{
			ID:   call.ID,
			Type: "function",
			Function: openAIToolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}
	return out
}

func canonicalToolCallsToOpenAIStreamToolCalls(toolCalls []canonicalToolCall) []openAIToolCall {
	out := make([]openAIToolCall, 0, len(toolCalls))
	for i, call := range toolCalls {
		index := i
		out = append(out, openAIToolCall{
			Index: &index,
			ID:    call.ID,
			Type:  "function",
			Function: openAIToolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}
	return out
}

func geminiToCanonicalStreamEvent(resp geminiGenerateResponse, requestID string) canonicalStreamEvent {
	toolCalls := geminiResponseToolCalls(resp, requestID)
	finishReason := ""
	if reason := geminiFinishReason(resp); reason != "" {
		finishReason = openAIFinishReason(reason)
	}
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return canonicalStreamEvent{
		Text:         geminiResponseText(resp),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
	}
}

func canonicalToOpenAIStreamChunk(event canonicalStreamEvent, id string, created int64, model string) openAIChatResponse {
	choice := openAIChoice{Index: 0, Delta: &openAIStreamDelta{}}
	if event.Text != "" {
		choice.Delta.Content = event.Text
	}
	if len(event.ToolCalls) > 0 {
		choice.Delta.ToolCalls = canonicalToolCallsToOpenAIStreamToolCalls(event.ToolCalls)
	}
	if event.FinishReason != "" {
		choice.FinishReason = &event.FinishReason
	}
	return openAIChatResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []openAIChoice{choice},
	}
}
