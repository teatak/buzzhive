package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	OpenAIChat      = "openai"
	OpenAIResponses = "openai-responses"
	Gemini          = "gemini"
	Anthropic       = "anthropic"
)

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected exactly one JSON value")
		}
		return err
	}
	return nil
}

type CanonicalRequest struct {
	Model           string
	Messages        []CanonicalMessage
	Stream          bool
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	StopSequences   []string
	ResponseFormat  *CanonicalResponseFormat
	Reasoning       *CanonicalReasoning
	Tools           []CanonicalTool
	ToolChoice      *CanonicalToolChoice
	Extensions      map[string]json.RawMessage
}

type CanonicalMessage struct {
	Role  string
	Name  string
	Parts []CanonicalPart
}

type CanonicalPart struct {
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

type CanonicalTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Strict      *bool
}

type CanonicalToolChoice struct {
	Mode                 string
	AllowedFunctionNames []string
}

type CanonicalResponseFormat struct {
	MimeType string
	Name     string
	Schema   json.RawMessage
	Strict   *bool
}

type CanonicalReasoning struct {
	Mode            string
	Effort          string
	BudgetTokens    *int
	IncludeThoughts *bool
	Summary         string
}

type CanonicalToolCall struct {
	ID        string
	Name      string
	Arguments string
	Signature string
}

type CanonicalUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
	ReasoningTokens  int
}

func (u CanonicalUsage) IsZero() bool {
	return u.PromptTokens == 0 &&
		u.CompletionTokens == 0 &&
		u.TotalTokens == 0 &&
		u.CachedTokens == 0 &&
		u.ReasoningTokens == 0
}

type CanonicalResponse struct {
	ID           string
	Created      int64
	Model        string
	Role         string
	Text         string
	Refusal      string
	Reasoning    string
	Signature    string
	ToolCalls    []CanonicalToolCall
	FinishReason string
	Usage        CanonicalUsage
	Extensions   map[string]json.RawMessage
}

type CanonicalStreamEventType string

const (
	CanonicalStreamResponseStart      CanonicalStreamEventType = "response_start"
	CanonicalStreamMessageStart       CanonicalStreamEventType = "message_start"
	CanonicalStreamTextDelta          CanonicalStreamEventType = "text_delta"
	CanonicalStreamRefusalDelta       CanonicalStreamEventType = "refusal_delta"
	CanonicalStreamReasoningDelta     CanonicalStreamEventType = "reasoning_delta"
	CanonicalStreamToolCallStart      CanonicalStreamEventType = "tool_call_start"
	CanonicalStreamToolArgumentsDelta CanonicalStreamEventType = "tool_arguments_delta"
	CanonicalStreamToolCallDone       CanonicalStreamEventType = "tool_call_done"
	CanonicalStreamUsage              CanonicalStreamEventType = "usage"
	CanonicalStreamResponseDone       CanonicalStreamEventType = "response_done"
	CanonicalStreamError              CanonicalStreamEventType = "error"
)

type CanonicalStreamErrorData struct {
	Code    string
	Message string
}

type CanonicalStreamEvent struct {
	Type         CanonicalStreamEventType
	ID           string
	Created      int64
	Model        string
	Role         string
	Index        int
	CallID       string
	Name         string
	Delta        string
	Arguments    string
	Signature    string
	FinishReason string
	Usage        CanonicalUsage
	Error        *CanonicalStreamErrorData
	Extensions   map[string]json.RawMessage
}

func populateCanonicalToolResponseNames(messages []CanonicalMessage) error {
	names := make(map[string]string)
	for messageIndex := range messages {
		for partIndex := range messages[messageIndex].Parts {
			part := &messages[messageIndex].Parts[partIndex]
			if part.Type == "tool_call" && part.ToolCallID != "" && part.Name != "" {
				names[part.ToolCallID] = part.Name
				continue
			}
			if part.Type != "tool_response" || part.Name != "" {
				continue
			}
			part.Name = names[part.ToolCallID]
		}
	}
	return nil
}

func validateCanonicalToolChoice(choice *CanonicalToolChoice, tools []CanonicalTool) error {
	if choice == nil {
		return nil
	}
	switch choice.Mode {
	case "", "AUTO", "NONE", "ANY":
	default:
		return fmt.Errorf("unsupported canonical tool choice mode %q", choice.Mode)
	}
	if choice.Mode == "NONE" && len(choice.AllowedFunctionNames) > 0 {
		return fmt.Errorf("tool choice NONE cannot include allowed functions")
	}
	if choice.Mode == "ANY" && len(tools) == 0 {
		return fmt.Errorf("tool choice ANY requires at least one function tool")
	}
	seen := make(map[string]struct{}, len(choice.AllowedFunctionNames))
	for _, rawName := range choice.AllowedFunctionNames {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return fmt.Errorf("allowed function name is required")
		}
		if !canonicalToolExists(tools, name) {
			return fmt.Errorf("tool choice references unknown function %q", name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("tool choice contains duplicate function %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}
