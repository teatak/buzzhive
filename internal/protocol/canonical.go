package protocol

import "encoding/json"

const (
	OpenAIChat      = "openai"
	OpenAIResponses = "openai-responses"
	Gemini          = "gemini"
	Anthropic       = "anthropic"
)

type CanonicalRequest struct {
	Model           string
	System          []Part
	Messages        []Message
	Tools           []Tool
	ToolChoice      *ToolChoice
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	Stream          bool
	ReasoningEffort *string
	Metadata        map[string]any
	Raw             []byte
}

type Message struct {
	Role       string
	Name       string
	ToolCallID string
	Parts      []Part
}

type Part struct {
	Type       string
	Text       string
	Image      *Image
	ToolCall   *ToolCall
	ToolResult *ToolResult
}

type Image struct {
	MediaType string
	Data      []byte
	URL       string
}

type Tool struct {
	Type     string
	Function FunctionTool
}

type FunctionTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolChoice struct {
	Mode         string
	FunctionName string
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolResult struct {
	ID      string
	Name    string
	Content json.RawMessage
}

type CanonicalResponse struct {
	ID           string
	Model        string
	Message      Message
	FinishReason string
	Usage        Usage
	Raw          []byte
}

type StreamEvent struct {
	Type          string
	TextDelta     string
	ToolCallDelta *ToolCallDelta
	Usage         Usage
	Error         error
	Done          bool
	Raw           []byte
}

type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
	ReasoningTokens  int64
	Raw              json.RawMessage
}
