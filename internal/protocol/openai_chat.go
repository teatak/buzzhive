package protocol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedOpenAIContent = errors.New("only text, data URL image, and input_audio chat content are supported in this version")

type OpenAIChatRequest struct {
	Model            string               `json:"model"`
	Messages         []OpenAIMessage      `json:"messages"`
	Stream           bool                 `json:"stream"`
	N                *int                 `json:"n,omitempty"`
	Tools            json.RawMessage      `json:"tools,omitempty"`
	ToolChoice       json.RawMessage      `json:"tool_choice,omitempty"`
	Temperature      *float64             `json:"temperature,omitempty"`
	TopP             *float64             `json:"top_p,omitempty"`
	MaxTokens        *int                 `json:"max_tokens,omitempty"`
	MaxOutputTokens  *int                 `json:"max_completion_tokens,omitempty"`
	Stop             any                  `json:"stop,omitempty"`
	PresencePenalty  *float64             `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64             `json:"frequency_penalty,omitempty"`
	LogitBias        json.RawMessage      `json:"logit_bias,omitempty"`
	Logprobs         *bool                `json:"logprobs,omitempty"`
	TopLogprobs      *int                 `json:"top_logprobs,omitempty"`
	Seed             *int64               `json:"seed,omitempty"`
	User             string               `json:"user,omitempty"`
	Metadata         json.RawMessage      `json:"metadata,omitempty"`
	ReasoningEffort  *string              `json:"reasoning_effort,omitempty"`
	ResponseFormat   json.RawMessage      `json:"response_format,omitempty"`
	StreamOptions    *OpenAIStreamOptions `json:"stream_options,omitempty"`
}

type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIToolCall struct {
	Index    *int                   `json:"index,omitempty"`
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIFunctionTool `json:"function"`
}

type OpenAIFunctionTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func CanonicalToOpenAIChatRequest(req ChatRequest) (OpenAIChatRequest, error) {
	out := OpenAIChatRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
		Stop:            canonicalStopSequencesToOpenAI(req.StopSequences),
		ReasoningEffort: req.ThinkingLevel,
	}
	tools, err := canonicalToolsToOpenAI(req.Tools)
	if err != nil {
		return OpenAIChatRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := canonicalToolChoiceToOpenAI(req.ToolChoice)
	if err != nil {
		return OpenAIChatRequest{}, err
	}
	out.ToolChoice = toolChoice
	responseFormat, err := canonicalResponseFormatToOpenAI(req.ResponseFormat)
	if err != nil {
		return OpenAIChatRequest{}, err
	}
	out.ResponseFormat = responseFormat
	for _, message := range req.Messages {
		messages, err := canonicalMessageToOpenAI(message)
		if err != nil {
			return OpenAIChatRequest{}, err
		}
		out.Messages = append(out.Messages, messages...)
	}
	return out, nil
}

func OpenAIChatToCanonical(req OpenAIChatRequest) (ChatRequest, error) {
	maxTokens := req.MaxTokens
	if maxTokens == nil {
		maxTokens = req.MaxOutputTokens
	}
	out := ChatRequest{
		Model:           req.Model,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: maxTokens,
		StopSequences:   openAIStopSequences(req.Stop),
	}
	tools, err := openAIToolsToCanonical(req.Tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.Tools = tools
	toolChoice, err := openAIToolChoiceToCanonical(req.ToolChoice, tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.ToolChoice = toolChoice
	responseFormat, err := openAIResponseFormatToCanonical(req.ResponseFormat)
	if err != nil {
		return ChatRequest{}, err
	}
	out.ResponseFormat = responseFormat
	hasConversationMessage := false
	toolCallNames := make(map[string]string)
	for _, message := range req.Messages {
		switch message.Role {
		case "system", "developer", "assistant", "user":
			parts, err := openAIMessageToCanonicalParts(message, toolCallNames)
			if err != nil {
				return ChatRequest{}, err
			}
			if canonicalPartsEmpty(parts) {
				continue
			}
			out.Messages = append(out.Messages, ChatMessage{
				Role:  message.Role,
				Parts: parts,
			})
			if message.Role == "assistant" || message.Role == "user" {
				hasConversationMessage = true
			}
		case "tool":
			parts, err := openAIToolMessageToCanonicalParts(message, toolCallNames)
			if err != nil {
				return ChatRequest{}, err
			}
			if canonicalPartsEmpty(parts) {
				continue
			}
			out.Messages = append(out.Messages, ChatMessage{
				Role:  message.Role,
				Parts: parts,
			})
		default:
			return ChatRequest{}, fmt.Errorf("unsupported message role %q", message.Role)
		}
	}
	if !hasConversationMessage {
		return ChatRequest{}, errors.New("messages must contain at least one user or assistant message")
	}
	return out, nil
}

func canonicalMessageToOpenAI(message ChatMessage) ([]OpenAIMessage, error) {
	var contentParts []ChatPart
	var toolCalls []OpenAIToolCall
	var toolMessages []OpenAIMessage
	for _, part := range message.Parts {
		switch part.Type {
		case "text", "image", "audio":
			contentParts = append(contentParts, part)
		case "tool_call":
			args := strings.TrimSpace(string(part.Arguments))
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   part.ToolCallID,
				Type: "function",
				Function: OpenAIToolCallFunction{
					Name:      part.Name,
					Arguments: args,
				},
			})
		case "tool_response":
			content, err := canonicalToolResponseToOpenAIContent(part.Response)
			if err != nil {
				return nil, err
			}
			toolMessages = append(toolMessages, OpenAIMessage{
				Role:       "tool",
				ToolCallID: part.ToolCallID,
				Content:    content,
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	if len(toolMessages) > 0 {
		if len(contentParts) > 0 || len(toolCalls) > 0 || message.Role != "tool" {
			return nil, errors.New("tool_response parts must be standalone tool messages")
		}
		return toolMessages, nil
	}
	content, err := canonicalContentPartsToOpenAI(contentParts)
	if err != nil {
		return nil, err
	}
	out := OpenAIMessage{
		Role:      canonicalRoleToOpenAI(message.Role),
		Content:   content,
		ToolCalls: toolCalls,
	}
	if out.Role == "assistant" && len(toolCalls) > 0 && len(contentParts) == 0 {
		out.Content = json.RawMessage("null")
	}
	return []OpenAIMessage{out}, nil
}

func canonicalRoleToOpenAI(role string) string {
	if role == "developer" {
		return "developer"
	}
	return role
}

func canonicalContentPartsToOpenAI(parts []ChatPart) (json.RawMessage, error) {
	if len(parts) == 0 {
		return json.RawMessage("null"), nil
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return marshalJSONRaw(parts[0].Text)
	}
	out := make([]openAIContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, openAIContentPart{Type: "text", Text: part.Text})
		case "image":
			item := openAIContentPart{Type: "image_url"}
			item.ImageURL.URL = dataURL(part.MimeType, part.Data)
			out = append(out, item)
		case "audio":
			format, err := openAIAudioFormatFromMimeType(part.MimeType)
			if err != nil {
				return nil, err
			}
			item := openAIContentPart{Type: "input_audio"}
			item.InputAudio.Data = part.Data
			item.InputAudio.Format = format
			out = append(out, item)
		default:
			return nil, fmt.Errorf("unsupported canonical content part type %q", part.Type)
		}
	}
	return marshalJSONRaw(out)
}

func canonicalToolResponseToOpenAIContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return marshalJSONRaw("")
	}
	if json.Valid(raw) {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return marshalJSONRaw(text)
		}
		return marshalJSONRaw(string(bytes.TrimSpace(raw)))
	}
	return nil, errors.New("tool_response content must be valid JSON")
}

func canonicalToolsToOpenAI(tools []ChatTool) (json.RawMessage, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]OpenAITool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		parameters := tool.Parameters
		if len(bytes.TrimSpace(parameters)) == 0 || bytes.Equal(bytes.TrimSpace(parameters), []byte("null")) {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, OpenAITool{
			Type: "function",
			Function: OpenAIFunctionTool{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}
	return marshalJSONRaw(out)
}

func canonicalToolChoiceToOpenAI(choice *ChatToolChoice) (json.RawMessage, error) {
	if choice == nil || strings.TrimSpace(choice.Mode) == "" {
		return nil, nil
	}
	switch choice.Mode {
	case "NONE":
		return marshalJSONRaw("none")
	case "ANY":
		if len(choice.AllowedFunctionNames) == 0 {
			return marshalJSONRaw("required")
		}
		if len(choice.AllowedFunctionNames) == 1 {
			return marshalJSONRaw(map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": choice.AllowedFunctionNames[0],
				},
			})
		}
		tools := make([]map[string]any, 0, len(choice.AllowedFunctionNames))
		for _, name := range choice.AllowedFunctionNames {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			})
		}
		return marshalJSONRaw(map[string]any{
			"type": "allowed_tools",
			"allowed_tools": map[string]any{
				"mode":  "required",
				"tools": tools,
			},
		})
	default:
		return nil, fmt.Errorf("unsupported canonical tool choice mode %q", choice.Mode)
	}
}

func canonicalResponseFormatToOpenAI(format *ChatResponseFormat) (json.RawMessage, error) {
	if format == nil {
		return nil, nil
	}
	if strings.TrimSpace(format.MimeType) != "application/json" {
		return nil, fmt.Errorf("unsupported response mime type %q", format.MimeType)
	}
	if len(bytes.TrimSpace(format.Schema)) == 0 {
		return marshalJSONRaw(map[string]string{"type": "json_object"})
	}
	return marshalJSONRaw(map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "canonical_schema",
			"schema": json.RawMessage(format.Schema),
		},
	})
}

func canonicalStopSequencesToOpenAI(stop []string) any {
	switch len(stop) {
	case 0:
		return nil
	case 1:
		return stop[0]
	default:
		return stop
	}
}

func dataURL(mimeType, data string) string {
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return "data:" + mimeType + ";base64," + data
}

func canonicalPartsEmpty(parts []ChatPart) bool {
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
			if strings.TrimSpace(part.Name) != "" || strings.TrimSpace(part.ToolCallID) != "" || len(bytes.TrimSpace(part.Response)) > 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func openAIMessageToCanonicalParts(message OpenAIMessage, toolCallNames map[string]string) ([]ChatPart, error) {
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
		parts = append(parts, ChatPart{
			Type:       "tool_call",
			ToolCallID: toolCall.ID,
			Name:       toolCall.Function.Name,
			Arguments:  args,
		})
	}
	return parts, nil
}

func openAIToolMessageToCanonicalParts(message OpenAIMessage, toolCallNames map[string]string) ([]ChatPart, error) {
	if strings.TrimSpace(message.ToolCallID) == "" {
		return nil, errors.New("tool message tool_call_id is required")
	}
	name := toolCallNames[message.ToolCallID]
	if name == "" {
		return nil, fmt.Errorf("tool message references unknown tool_call_id %q", message.ToolCallID)
	}
	response, err := openAIToolContentToCanonicalResponse(message.Content)
	if err != nil {
		return nil, err
	}
	return []ChatPart{{
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

func openAIToolContentToCanonicalResponse(raw json.RawMessage) (json.RawMessage, error) {
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

func openAIToolsToCanonical(raw json.RawMessage) ([]ChatTool, error) {
	if !openAIToolsRequested(raw) {
		return nil, nil
	}
	var tools []OpenAITool
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, errors.New("tools must be an array")
	}
	out := make([]ChatTool, 0, len(tools))
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
		out = append(out, ChatTool{
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

func openAIToolChoiceToCanonical(raw json.RawMessage, tools []ChatTool) (*ChatToolChoice, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		switch text {
		case "", "auto":
			return nil, nil
		case "none":
			return &ChatToolChoice{Mode: "NONE"}, nil
		case "required":
			if len(tools) == 0 {
				return nil, errors.New("tool_choice required needs at least one tool")
			}
			return &ChatToolChoice{Mode: "ANY"}, nil
		default:
			return nil, fmt.Errorf("unsupported tool_choice %q", text)
		}
	}
	var choice struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
		AllowedTools struct {
			Mode  string `json:"mode"`
			Tools []struct {
				Type     string `json:"type"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		} `json:"allowed_tools"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, errors.New("tool_choice must be a string or function object")
	}
	if choice.Type == "allowed_tools" {
		if len(choice.AllowedTools.Tools) == 0 {
			return nil, errors.New("tool_choice allowed_tools.tools is required")
		}
		names := make([]string, 0, len(choice.AllowedTools.Tools))
		for _, tool := range choice.AllowedTools.Tools {
			if tool.Type != "function" {
				return nil, fmt.Errorf("unsupported allowed_tools tool type %q", tool.Type)
			}
			name := strings.TrimSpace(tool.Function.Name)
			if name == "" {
				return nil, errors.New("allowed_tools function name is required")
			}
			if !canonicalToolExists(tools, name) {
				return nil, fmt.Errorf("allowed_tools references unknown function %q", name)
			}
			names = append(names, name)
		}
		mode := strings.TrimSpace(choice.AllowedTools.Mode)
		if mode == "" || mode == "auto" || mode == "required" {
			return &ChatToolChoice{Mode: "ANY", AllowedFunctionNames: names}, nil
		}
		return nil, fmt.Errorf("unsupported allowed_tools mode %q", mode)
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
	return &ChatToolChoice{Mode: "ANY", AllowedFunctionNames: []string{name}}, nil
}

func openAIResponseFormatToCanonical(raw json.RawMessage) (*ChatResponseFormat, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var format struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(raw, &format); err != nil {
		return nil, errors.New("response_format must be an object")
	}
	switch format.Type {
	case "", "text":
		return nil, nil
	case "json_object":
		return &ChatResponseFormat{MimeType: "application/json"}, nil
	case "json_schema":
		if len(format.JSONSchema.Schema) == 0 || strings.TrimSpace(string(format.JSONSchema.Schema)) == "null" {
			return nil, errors.New("response_format json_schema.schema is required")
		}
		return &ChatResponseFormat{
			MimeType: "application/json",
			Schema:   format.JSONSchema.Schema,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported response_format type %q", format.Type)
	}
}

func canonicalToolExists(tools []ChatTool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func openAIStopSequences(value any) []string {
	switch stop := value.(type) {
	case string:
		return []string{stop}
	case []any:
		out := make([]string, 0, len(stop))
		for _, item := range stop {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func openAIMessageParts(raw json.RawMessage) ([]ChatPart, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []ChatPart{{Type: "text", Text: text}}, nil
	}
	var parts []openAIContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, ErrUnsupportedOpenAIContent
	}
	out := make([]ChatPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text":
			out = append(out, ChatPart{Type: "text", Text: part.Text})
		case "image_url":
			mimeType, data, err := parseOpenAIImageDataURL(part.ImageURL.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatPart{Type: "image", MimeType: mimeType, Data: data})
		case "input_audio":
			mimeType, data, err := parseOpenAIInputAudio(part.InputAudio.Data, part.InputAudio.Format)
			if err != nil {
				return nil, err
			}
			out = append(out, ChatPart{Type: "audio", MimeType: mimeType, Data: data})
		default:
			return nil, ErrUnsupportedOpenAIContent
		}
	}
	return out, nil
}

type openAIContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
	InputAudio struct {
		Data   string `json:"data"`
		Format string `json:"format"`
	} `json:"input_audio"`
}

func parseOpenAIImageDataURL(value string) (string, string, error) {
	const prefix = "data:"
	if !strings.HasPrefix(value, prefix) {
		return "", "", ErrUnsupportedOpenAIContent
	}
	metaAndData := strings.SplitN(strings.TrimPrefix(value, prefix), ",", 2)
	if len(metaAndData) != 2 {
		return "", "", ErrUnsupportedOpenAIContent
	}
	meta := metaAndData[0]
	data := metaAndData[1]
	if !strings.HasSuffix(meta, ";base64") || data == "" {
		return "", "", ErrUnsupportedOpenAIContent
	}
	mimeType := strings.TrimSuffix(meta, ";base64")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return "", "", ErrUnsupportedOpenAIContent
	}
	return mimeType, data, nil
}

func parseOpenAIInputAudio(data string, format string) (string, string, error) {
	data = strings.TrimSpace(data)
	format = strings.ToLower(strings.TrimSpace(format))
	if data == "" || format == "" {
		return "", "", ErrUnsupportedOpenAIContent
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return "", "", ErrUnsupportedOpenAIContent
	}
	mimeType, ok := openAIAudioMimeTypes[format]
	if !ok {
		return "", "", ErrUnsupportedOpenAIContent
	}
	return mimeType, data, nil
}

func openAIAudioFormatFromMimeType(mimeType string) (string, error) {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	for format, candidate := range openAIAudioMimeTypes {
		if candidate == mimeType {
			return format, nil
		}
	}
	return "", ErrUnsupportedOpenAIContent
}

var openAIAudioMimeTypes = map[string]string{
	"wav":  "audio/wav",
	"mp3":  "audio/mpeg",
	"mpeg": "audio/mpeg",
	"mpga": "audio/mpeg",
	"webm": "audio/webm",
	"ogg":  "audio/ogg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"aac":  "audio/aac",
}
