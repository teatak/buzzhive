package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type GeminiGenerateRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	Tools             []GeminiTool            `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type GeminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type GeminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type GeminiToolConfig struct {
	FunctionCallingConfig *GeminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type GeminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type GeminiGenerationConfig struct {
	Temperature      *float64              `json:"temperature,omitempty"`
	TopP             *float64              `json:"topP,omitempty"`
	MaxOutputTokens  *int                  `json:"maxOutputTokens,omitempty"`
	StopSequences    []string              `json:"stopSequences,omitempty"`
	ResponseMimeType string                `json:"responseMimeType,omitempty"`
	ResponseSchema   json.RawMessage       `json:"responseSchema,omitempty"`
	ThinkingConfig   *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiThinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

type GeminiGenerateResponse struct {
	Candidates    []GeminiCandidate   `json:"candidates"`
	UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
}

type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
}

func GeminiGenerateToCanonicalRequest(req GeminiGenerateRequest, model string, stream bool) (ChatRequest, error) {
	out := ChatRequest{
		Model:  model,
		Stream: stream,
	}
	if req.GenerationConfig != nil {
		out.Temperature = req.GenerationConfig.Temperature
		out.TopP = req.GenerationConfig.TopP
		out.MaxOutputTokens = req.GenerationConfig.MaxOutputTokens
		out.StopSequences = req.GenerationConfig.StopSequences
		if req.GenerationConfig.ResponseMimeType != "" {
			out.ResponseFormat = &ChatResponseFormat{
				MimeType: req.GenerationConfig.ResponseMimeType,
				Schema:   req.GenerationConfig.ResponseSchema,
			}
		}
		if req.GenerationConfig.ThinkingConfig != nil && strings.TrimSpace(req.GenerationConfig.ThinkingConfig.ThinkingLevel) != "" {
			out.ThinkingLevel = &req.GenerationConfig.ThinkingConfig.ThinkingLevel
		}
	}
	tools, err := geminiToolsToCanonical(req.Tools)
	if err != nil {
		return ChatRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice = geminiToolConfigToCanonical(req.ToolConfig)
	if req.SystemInstruction != nil {
		parts, err := geminiPartsToCanonical(req.SystemInstruction.Parts, 0)
		if err != nil {
			return ChatRequest{}, err
		}
		if !canonicalPartsEmpty(parts) {
			out.Messages = append(out.Messages, ChatMessage{Role: "system", Parts: parts})
		}
	}
	for messageIndex, content := range req.Contents {
		parts, err := geminiPartsToCanonical(content.Parts, messageIndex)
		if err != nil {
			return ChatRequest{}, err
		}
		if canonicalPartsEmpty(parts) {
			continue
		}
		role, err := geminiRoleToCanonical(content.Role, parts)
		if err != nil {
			return ChatRequest{}, err
		}
		out.Messages = append(out.Messages, ChatMessage{Role: role, Parts: parts})
	}
	return out, nil
}

func CanonicalToGeminiGenerateRequest(req ChatRequest) (GeminiGenerateRequest, error) {
	out := GeminiGenerateRequest{
		GenerationConfig: canonicalToGeminiGenerationConfig(req),
		ToolConfig:       canonicalToolChoiceToGeminiToolConfig(req.ToolChoice),
	}
	tools, err := canonicalToolsToGeminiTools(req.Tools)
	if err != nil {
		return GeminiGenerateRequest{}, err
	}
	out.Tools = tools
	var systemParts []GeminiPart
	for _, message := range req.Messages {
		parts, err := canonicalPartsToGeminiParts(message.Parts)
		if err != nil {
			return GeminiGenerateRequest{}, err
		}
		switch message.Role {
		case "system", "developer":
			systemParts = append(systemParts, parts...)
		case "assistant":
			out.Contents = append(out.Contents, GeminiContent{Role: "model", Parts: parts})
		case "tool":
			out.Contents = append(out.Contents, GeminiContent{Role: "user", Parts: parts})
		case "user":
			out.Contents = append(out.Contents, GeminiContent{Role: "user", Parts: parts})
		default:
			return GeminiGenerateRequest{}, fmt.Errorf("unsupported canonical message role %q", message.Role)
		}
	}
	if len(systemParts) > 0 {
		out.SystemInstruction = &GeminiContent{Parts: systemParts}
	}
	if len(out.Contents) == 0 {
		return GeminiGenerateRequest{}, errors.New("messages must contain at least one user or assistant message")
	}
	return out, nil
}

func geminiToolsToCanonical(tools []GeminiTool) ([]ChatTool, error) {
	var out []ChatTool
	for _, tool := range tools {
		for _, declaration := range tool.FunctionDeclarations {
			if strings.TrimSpace(declaration.Name) == "" {
				return nil, errors.New("function declaration name is required")
			}
			parameters := declaration.Parameters
			if len(strings.TrimSpace(string(parameters))) == 0 || strings.TrimSpace(string(parameters)) == "null" {
				parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			out = append(out, ChatTool{
				Name:        declaration.Name,
				Description: declaration.Description,
				Parameters:  parameters,
			})
		}
	}
	return out, nil
}

func geminiToolConfigToCanonical(config *GeminiToolConfig) *ChatToolChoice {
	if config == nil || config.FunctionCallingConfig == nil {
		return nil
	}
	mode := strings.TrimSpace(config.FunctionCallingConfig.Mode)
	if mode == "" || mode == "AUTO" {
		return nil
	}
	switch mode {
	case "NONE":
		return &ChatToolChoice{Mode: "NONE"}
	case "ANY":
		return &ChatToolChoice{
			Mode:                 "ANY",
			AllowedFunctionNames: config.FunctionCallingConfig.AllowedFunctionNames,
		}
	default:
		return &ChatToolChoice{Mode: mode, AllowedFunctionNames: config.FunctionCallingConfig.AllowedFunctionNames}
	}
}

func geminiPartsToCanonical(parts []GeminiPart, messageIndex int) ([]ChatPart, error) {
	out := make([]ChatPart, 0, len(parts))
	for partIndex, part := range parts {
		switch {
		case part.Text != "":
			out = append(out, ChatPart{Type: "text", Text: part.Text})
		case part.InlineData != nil:
			partType := "image"
			if strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "audio/") {
				partType = "audio"
			}
			out = append(out, ChatPart{
				Type:     partType,
				MimeType: part.InlineData.MimeType,
				Data:     part.InlineData.Data,
			})
		case part.FunctionCall != nil:
			id := fmt.Sprintf("call_%d_%d", messageIndex, partIndex)
			out = append(out, ChatPart{
				Type:       "tool_call",
				ToolCallID: id,
				Name:       part.FunctionCall.Name,
				Arguments:  part.FunctionCall.Args,
				Signature:  part.ThoughtSignature,
			})
		case part.FunctionResponse != nil:
			out = append(out, ChatPart{
				Type:       "tool_response",
				ToolCallID: "call_" + strings.TrimSpace(part.FunctionResponse.Name),
				Name:       part.FunctionResponse.Name,
				Response:   part.FunctionResponse.Response,
			})
		default:
			return nil, errors.New("unsupported empty Gemini part")
		}
	}
	return out, nil
}

func geminiRoleToCanonical(role string, parts []ChatPart) (string, error) {
	if len(parts) > 0 && allCanonicalPartsAreToolResponses(parts) {
		return "tool", nil
	}
	switch role {
	case "", "user":
		return "user", nil
	case "model":
		return "assistant", nil
	default:
		return "", fmt.Errorf("unsupported Gemini role %q", role)
	}
}

func allCanonicalPartsAreToolResponses(parts []ChatPart) bool {
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part.Type != "tool_response" {
			return false
		}
	}
	return true
}

func canonicalToolsToGeminiTools(tools []ChatTool) ([]GeminiTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	declarations := make([]GeminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		declarations = append(declarations, GeminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}
	return []GeminiTool{{FunctionDeclarations: declarations}}, nil
}

func canonicalToolChoiceToGeminiToolConfig(choice *ChatToolChoice) *GeminiToolConfig {
	if choice == nil {
		return nil
	}
	return &GeminiToolConfig{
		FunctionCallingConfig: &GeminiFunctionCallingConfig{
			Mode:                 choice.Mode,
			AllowedFunctionNames: choice.AllowedFunctionNames,
		},
	}
}

func canonicalPartsToGeminiParts(parts []ChatPart) ([]GeminiPart, error) {
	out := make([]GeminiPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, GeminiPart{Text: part.Text})
		case "image", "audio":
			out = append(out, GeminiPart{
				InlineData: &GeminiInlineData{
					MimeType: part.MimeType,
					Data:     part.Data,
				},
			})
		case "tool_call":
			out = append(out, GeminiPart{
				FunctionCall: &GeminiFunctionCall{
					Name: part.Name,
					Args: part.Arguments,
				},
				ThoughtSignature: part.Signature,
			})
		case "tool_response":
			out = append(out, GeminiPart{
				FunctionResponse: &GeminiFunctionResponse{
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

func canonicalToGeminiGenerationConfig(req ChatRequest) *GeminiGenerationConfig {
	cfg := &GeminiGenerationConfig{
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		MaxOutputTokens:  req.MaxOutputTokens,
		StopSequences:    req.StopSequences,
		ResponseMimeType: "",
	}
	if req.ThinkingLevel != nil {
		cfg.ThinkingConfig = &GeminiThinkingConfig{ThinkingLevel: *req.ThinkingLevel}
	}
	if req.ResponseFormat != nil {
		cfg.ResponseMimeType = req.ResponseFormat.MimeType
		cfg.ResponseSchema = req.ResponseFormat.Schema
	}
	if cfg.Temperature == nil && cfg.TopP == nil && cfg.MaxOutputTokens == nil && len(cfg.StopSequences) == 0 && cfg.ResponseMimeType == "" && len(cfg.ResponseSchema) == 0 && cfg.ThinkingConfig == nil {
		return nil
	}
	return cfg
}
