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
	Thought          bool                    `json:"thought,omitempty"`
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

func (p GeminiPart) MarshalJSON() ([]byte, error) {
	type wirePart struct {
		Text             *string                 `json:"text,omitempty"`
		Thought          bool                    `json:"thought,omitempty"`
		InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
		FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
		FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
		ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
	}

	var text *string
	hasStructuredData := p.InlineData != nil || p.FunctionCall != nil || p.FunctionResponse != nil
	if p.Text != "" || (!hasStructuredData && p.ThoughtSignature != "") {
		text = &p.Text
	}
	if text == nil && !hasStructuredData {
		return nil, errors.New("Gemini part must contain text, inlineData, functionCall, or functionResponse")
	}
	return json.Marshal(wirePart{
		Text:             text,
		Thought:          p.Thought,
		InlineData:       p.InlineData,
		FunctionCall:     p.FunctionCall,
		FunctionResponse: p.FunctionResponse,
		ThoughtSignature: p.ThoughtSignature,
	})
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description,omitempty"`
	Parameters           json.RawMessage `json:"parameters,omitempty"`
	ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

type GeminiFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type GeminiFunctionResponse struct {
	ID       string          `json:"id,omitempty"`
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
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	MaxOutputTokens    *int                  `json:"maxOutputTokens,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	ResponseSchema     json.RawMessage       `json:"responseSchema,omitempty"`
	ResponseJSONSchema json.RawMessage       `json:"responseJsonSchema,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiThinkingConfig struct {
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
	IncludeThoughts *bool  `json:"includeThoughts,omitempty"`
}

type GeminiGenerateResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  GeminiUsageMetadata   `json:"usageMetadata"`
}

type GeminiPromptFeedback struct {
	BlockReason        string               `json:"blockReason,omitempty"`
	BlockReasonMessage string               `json:"blockReasonMessage,omitempty"`
	SafetyRatings      []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiSafetyRating struct {
	Category    string `json:"category,omitempty"`
	Probability string `json:"probability,omitempty"`
	Blocked     bool   `json:"blocked,omitempty"`
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

func GeminiGenerateToCanonicalRequest(req GeminiGenerateRequest, model string, stream bool) (CanonicalRequest, error) {
	out := CanonicalRequest{
		Model:  model,
		Stream: stream,
	}
	toolCalls := newGeminiToolCallResolver()
	if req.GenerationConfig != nil {
		out.Temperature = req.GenerationConfig.Temperature
		out.TopP = req.GenerationConfig.TopP
		out.MaxOutputTokens = req.GenerationConfig.MaxOutputTokens
		out.StopSequences = req.GenerationConfig.StopSequences
		if req.GenerationConfig.ResponseMimeType != "" {
			responseSchema, err := selectGeminiSchema(
				req.GenerationConfig.ResponseSchema,
				req.GenerationConfig.ResponseJSONSchema,
				"responseSchema",
				"responseJsonSchema",
			)
			if err != nil {
				return CanonicalRequest{}, err
			}
			out.ResponseFormat = &CanonicalResponseFormat{
				MimeType: req.GenerationConfig.ResponseMimeType,
				Schema:   responseSchema,
			}
		}
		if req.GenerationConfig.ThinkingConfig != nil {
			thinking := req.GenerationConfig.ThinkingConfig
			if strings.TrimSpace(thinking.ThinkingLevel) != "" || thinking.IncludeThoughts != nil {
				out.Reasoning = &CanonicalReasoning{
					Effort:          thinking.ThinkingLevel,
					IncludeThoughts: thinking.IncludeThoughts,
				}
			}
		}
	}
	tools, err := geminiToolsToCanonical(req.Tools)
	if err != nil {
		return CanonicalRequest{}, err
	}
	out.Tools = tools
	out.ToolChoice = geminiToolConfigToCanonical(req.ToolConfig)
	if err := validateCanonicalToolChoice(out.ToolChoice, out.Tools); err != nil {
		return CanonicalRequest{}, err
	}
	if req.SystemInstruction != nil {
		parts, err := geminiPartsToCanonical(req.SystemInstruction.Parts, -1, toolCalls)
		if err != nil {
			return CanonicalRequest{}, err
		}
		if !canonicalPartsEmpty(parts) {
			out.Messages = append(out.Messages, CanonicalMessage{Role: "system", Parts: parts})
		}
	}
	for messageIndex, content := range req.Contents {
		parts, err := geminiPartsToCanonical(content.Parts, messageIndex, toolCalls)
		if err != nil {
			return CanonicalRequest{}, err
		}
		if canonicalPartsEmpty(parts) {
			continue
		}
		role, err := geminiRoleToCanonical(content.Role, parts)
		if err != nil {
			return CanonicalRequest{}, err
		}
		out.Messages = append(out.Messages, CanonicalMessage{Role: role, Parts: parts})
	}
	return out, nil
}

func CanonicalToGeminiGenerateRequest(req CanonicalRequest) (GeminiGenerateRequest, error) {
	if err := validateCanonicalToolChoice(req.ToolChoice, req.Tools); err != nil {
		return GeminiGenerateRequest{}, err
	}
	if err := populateCanonicalToolResponseNames(req.Messages); err != nil {
		return GeminiGenerateRequest{}, err
	}
	generationConfig, err := canonicalToGeminiGenerationConfig(req)
	if err != nil {
		return GeminiGenerateRequest{}, err
	}
	out := GeminiGenerateRequest{
		GenerationConfig: generationConfig,
		ToolConfig:       canonicalToolChoiceToGeminiToolConfig(req.ToolChoice),
	}
	tools, err := canonicalToolsToGeminiTools(req.Tools)
	if err != nil {
		return GeminiGenerateRequest{}, err
	}
	out.Tools = tools
	var systemParts []GeminiPart
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Name) != "" {
			return GeminiGenerateRequest{}, errors.New("message name cannot be represented by Gemini")
		}
		parts, err := canonicalPartsToGeminiParts(message.Parts)
		if err != nil {
			return GeminiGenerateRequest{}, err
		}
		if len(parts) == 0 {
			continue
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

func geminiToolsToCanonical(tools []GeminiTool) ([]CanonicalTool, error) {
	var out []CanonicalTool
	for _, tool := range tools {
		for _, declaration := range tool.FunctionDeclarations {
			if strings.TrimSpace(declaration.Name) == "" {
				return nil, errors.New("function declaration name is required")
			}
			parameters, err := selectGeminiSchema(
				declaration.Parameters,
				declaration.ParametersJSONSchema,
				"parameters",
				"parametersJsonSchema",
			)
			if err != nil {
				return nil, fmt.Errorf("function declaration %q: %w", declaration.Name, err)
			}
			if len(strings.TrimSpace(string(parameters))) == 0 || strings.TrimSpace(string(parameters)) == "null" {
				parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			out = append(out, CanonicalTool{
				Name:        declaration.Name,
				Description: declaration.Description,
				Parameters:  parameters,
			})
		}
	}
	return out, nil
}

func selectGeminiSchema(legacy, jsonSchema json.RawMessage, legacyName, jsonSchemaName string) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(legacy))) > 0 && len(strings.TrimSpace(string(jsonSchema))) > 0 {
		return nil, fmt.Errorf("%s and %s are mutually exclusive", legacyName, jsonSchemaName)
	}
	if len(strings.TrimSpace(string(jsonSchema))) > 0 {
		return jsonSchema, nil
	}
	return legacy, nil
}

func geminiToolConfigToCanonical(config *GeminiToolConfig) *CanonicalToolChoice {
	if config == nil || config.FunctionCallingConfig == nil {
		return nil
	}
	mode := strings.TrimSpace(config.FunctionCallingConfig.Mode)
	if mode == "" || (mode == "AUTO" && len(config.FunctionCallingConfig.AllowedFunctionNames) == 0) {
		return nil
	}
	switch mode {
	case "AUTO":
		return &CanonicalToolChoice{
			Mode:                 "AUTO",
			AllowedFunctionNames: config.FunctionCallingConfig.AllowedFunctionNames,
		}
	case "NONE":
		return &CanonicalToolChoice{Mode: "NONE"}
	case "ANY":
		return &CanonicalToolChoice{
			Mode:                 "ANY",
			AllowedFunctionNames: config.FunctionCallingConfig.AllowedFunctionNames,
		}
	default:
		return &CanonicalToolChoice{Mode: mode, AllowedFunctionNames: config.FunctionCallingConfig.AllowedFunctionNames}
	}
}

type geminiToolCallResolver struct {
	pending map[string][]string
}

func newGeminiToolCallResolver() *geminiToolCallResolver {
	return &geminiToolCallResolver{pending: make(map[string][]string)}
}

func (r *geminiToolCallResolver) add(id string, name string, messageIndex int, partIndex int) string {
	id = strings.TrimSpace(id)
	if id == "" {
		id = fmt.Sprintf("call_%d_%d", messageIndex, partIndex)
	}
	r.pending[name] = append(r.pending[name], id)
	return id
}

func (r *geminiToolCallResolver) take(id string, name string) (string, bool) {
	id = strings.TrimSpace(id)
	ids := r.pending[name]
	if len(ids) == 0 {
		return "", false
	}
	index := 0
	if id != "" {
		index = -1
		for i, pendingID := range ids {
			if pendingID == id {
				index = i
				break
			}
		}
		if index < 0 {
			return "", false
		}
	}
	resolved := ids[index]
	ids = append(ids[:index], ids[index+1:]...)
	if len(ids) == 0 {
		delete(r.pending, name)
	} else {
		r.pending[name] = ids
	}
	return resolved, true
}

func geminiPartsToCanonical(parts []GeminiPart, messageIndex int, toolCalls *geminiToolCallResolver) ([]CanonicalPart, error) {
	out := make([]CanonicalPart, 0, len(parts))
	for partIndex, part := range parts {
		switch {
		case part.FunctionCall != nil:
			name := strings.TrimSpace(part.FunctionCall.Name)
			if name == "" {
				return nil, errors.New("Gemini functionCall name is required")
			}
			id := toolCalls.add(part.FunctionCall.ID, name, messageIndex, partIndex)
			out = append(out, CanonicalPart{
				Type:       "tool_call",
				ToolCallID: id,
				Name:       name,
				Arguments:  part.FunctionCall.Args,
				Signature:  part.ThoughtSignature,
			})
		case part.FunctionResponse != nil:
			name := strings.TrimSpace(part.FunctionResponse.Name)
			if name == "" {
				return nil, errors.New("Gemini functionResponse name is required")
			}
			id, ok := toolCalls.take(part.FunctionResponse.ID, name)
			if !ok {
				return nil, fmt.Errorf("Gemini functionResponse %q id %q has no matching functionCall", name, part.FunctionResponse.ID)
			}
			out = append(out, CanonicalPart{
				Type:       "tool_response",
				ToolCallID: id,
				Name:       name,
				Response:   part.FunctionResponse.Response,
			})
		case part.InlineData != nil:
			if part.Thought {
				return nil, errors.New("Gemini thought image/audio parts are not supported")
			}
			partType := "image"
			if strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "audio/") {
				partType = "audio"
			}
			out = append(out, CanonicalPart{
				Type:      partType,
				MimeType:  part.InlineData.MimeType,
				Data:      part.InlineData.Data,
				Signature: part.ThoughtSignature,
			})
		case part.Text != "" || part.ThoughtSignature != "":
			partType := "text"
			if part.Thought {
				partType = "reasoning"
			}
			out = append(out, CanonicalPart{
				Type:      partType,
				Text:      part.Text,
				Signature: part.ThoughtSignature,
			})
		default:
			return nil, errors.New("unsupported empty Gemini part")
		}
	}
	return out, nil
}

func geminiRoleToCanonical(role string, parts []CanonicalPart) (string, error) {
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

func allCanonicalPartsAreToolResponses(parts []CanonicalPart) bool {
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

func canonicalToolsToGeminiTools(tools []CanonicalTool) ([]GeminiTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	declarations := make([]GeminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return nil, errors.New("function tool name is required")
		}
		declarations = append(declarations, GeminiFunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJSONSchema: tool.Parameters,
		})
	}
	return []GeminiTool{{FunctionDeclarations: declarations}}, nil
}

func canonicalToolChoiceToGeminiToolConfig(choice *CanonicalToolChoice) *GeminiToolConfig {
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

func canonicalPartsToGeminiParts(parts []CanonicalPart) ([]GeminiPart, error) {
	out := make([]GeminiPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == "" && part.Signature == "" {
				continue
			}
			out = append(out, GeminiPart{
				Text:             part.Text,
				ThoughtSignature: part.Signature,
			})
		case "reasoning":
			if part.Text == "" && part.Signature == "" {
				continue
			}
			out = append(out, GeminiPart{
				Text:             part.Text,
				Thought:          true,
				ThoughtSignature: part.Signature,
			})
		case "image", "audio":
			out = append(out, GeminiPart{
				InlineData: &GeminiInlineData{
					MimeType: part.MimeType,
					Data:     part.Data,
				},
				ThoughtSignature: part.Signature,
			})
		case "tool_call":
			out = append(out, GeminiPart{
				FunctionCall: &GeminiFunctionCall{
					ID:   part.ToolCallID,
					Name: part.Name,
					Args: part.Arguments,
				},
				ThoughtSignature: part.Signature,
			})
		case "tool_response":
			if strings.TrimSpace(part.Name) == "" {
				return nil, fmt.Errorf("tool_response %q has no matching function name", part.ToolCallID)
			}
			response, err := canonicalToolResponseToGemini(part.Response)
			if err != nil {
				return nil, fmt.Errorf("tool_response %q: %w", part.ToolCallID, err)
			}
			out = append(out, GeminiPart{
				FunctionResponse: &GeminiFunctionResponse{
					ID:       part.ToolCallID,
					Name:     part.Name,
					Response: response,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported canonical part type %q", part.Type)
		}
	}
	return out, nil
}

func canonicalToolResponseToGemini(raw json.RawMessage) (json.RawMessage, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return json.RawMessage(`{"result":null}`), nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err == nil && object != nil {
		return json.Marshal(object)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, errors.New("response must contain valid JSON")
	}
	return json.Marshal(map[string]any{"result": decoded})
}

func canonicalToGeminiGenerationConfig(req CanonicalRequest) (*GeminiGenerationConfig, error) {
	includeThoughts := true
	cfg := &GeminiGenerationConfig{
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		MaxOutputTokens:  req.MaxOutputTokens,
		StopSequences:    req.StopSequences,
		ResponseMimeType: "",
		ThinkingConfig: &GeminiThinkingConfig{
			IncludeThoughts: &includeThoughts,
		},
	}
	if req.Reasoning != nil {
		if req.Reasoning.BudgetTokens != nil {
			return nil, errors.New("reasoning budget_tokens cannot be represented by Gemini thinkingLevel")
		}
		mode := strings.ToLower(strings.TrimSpace(req.Reasoning.Mode))
		if mode != "" && mode != "adaptive" {
			return nil, fmt.Errorf("reasoning mode %q cannot be represented by Gemini", req.Reasoning.Mode)
		}
		summary := strings.ToLower(strings.TrimSpace(req.Reasoning.Summary))
		if summary != "" && summary != "auto" {
			return nil, fmt.Errorf("reasoning summary %q cannot be represented by Gemini", req.Reasoning.Summary)
		}
		level := strings.ToUpper(strings.TrimSpace(req.Reasoning.Effort))
		switch level {
		case "", "MINIMAL", "LOW", "MEDIUM", "HIGH":
		default:
			return nil, fmt.Errorf("reasoning effort %q cannot be represented by Gemini thinkingLevel", req.Reasoning.Effort)
		}
		cfg.ThinkingConfig.ThinkingLevel = level
		if req.Reasoning.IncludeThoughts != nil {
			cfg.ThinkingConfig.IncludeThoughts = req.Reasoning.IncludeThoughts
		}
	}
	if req.ResponseFormat != nil {
		cfg.ResponseMimeType = req.ResponseFormat.MimeType
		cfg.ResponseJSONSchema = req.ResponseFormat.Schema
	}
	if cfg.Temperature == nil && cfg.TopP == nil && cfg.MaxOutputTokens == nil && len(cfg.StopSequences) == 0 && cfg.ResponseMimeType == "" && len(cfg.ResponseJSONSchema) == 0 && cfg.ThinkingConfig == nil {
		return nil, nil
	}
	return cfg, nil
}
