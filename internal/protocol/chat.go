package protocol

import "encoding/json"

type ChatRequest struct {
	Model           string
	Messages        []ChatMessage
	Stream          bool
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	StopSequences   []string
	ResponseFormat  *ChatResponseFormat
	ThinkingLevel   *string
	Tools           []ChatTool
	ToolChoice      *ChatToolChoice
}

type ChatMessage struct {
	Role  string
	Parts []ChatPart
}

type ChatPart struct {
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

type ChatTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ChatToolChoice struct {
	Mode                 string
	AllowedFunctionNames []string
}

type ChatResponseFormat struct {
	MimeType string
	Schema   json.RawMessage
}

type ChatToolCall struct {
	ID        string
	Name      string
	Arguments string
	Signature string
}

type ChatUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
	ReasoningTokens  int
}

type ChatResponse struct {
	ID           string
	Created      int64
	Model        string
	Role         string
	Text         string
	ToolCalls    []ChatToolCall
	FinishReason string
	Usage        ChatUsage
}

type ChatStreamEvent struct {
	Text         string
	ToolCalls    []ChatToolCall
	FinishReason string
	Usage        ChatUsage
}
