package protocol

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

type OpenAIChoice struct {
	Index        int                `json:"index"`
	Message      *OpenAIMessageOut  `json:"message,omitempty"`
	Delta        *OpenAIStreamDelta `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason"`
}

type OpenAIMessageOut struct {
	Role      string           `json:"role"`
	Content   *string          `json:"content"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIStreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens            int                            `json:"prompt_tokens"`
	CompletionTokens        int                            `json:"completion_tokens"`
	TotalTokens             int                            `json:"total_tokens"`
	PromptTokensDetails     *OpenAIPromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAICompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type OpenAIPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type OpenAICompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

func OpenAIChatResponseToCanonical(resp OpenAIChatResponse) ChatResponse {
	out := ChatResponse{
		ID:      resp.ID,
		Created: resp.Created,
		Model:   resp.Model,
		Usage:   openAIUsageToCanonical(resp.Usage),
	}
	if len(resp.Choices) == 0 {
		return out
	}
	choice := resp.Choices[0]
	if choice.FinishReason != nil {
		out.FinishReason = *choice.FinishReason
	}
	if choice.Message == nil {
		return out
	}
	out.Role = choice.Message.Role
	if choice.Message.Content != nil {
		out.Text = *choice.Message.Content
	}
	out.ToolCalls = openAIToolCallsToCanonical(choice.Message.ToolCalls)
	return out
}

func OpenAIChatStreamChunkToCanonical(chunk OpenAIChatResponse) ChatStreamEvent {
	out := ChatStreamEvent{Usage: openAIUsageToCanonical(chunk.Usage)}
	if len(chunk.Choices) == 0 {
		return out
	}
	choice := chunk.Choices[0]
	if choice.FinishReason != nil {
		out.FinishReason = *choice.FinishReason
	}
	if choice.Delta == nil {
		return out
	}
	out.Text = choice.Delta.Content
	out.ToolCalls = openAIToolCallsToCanonical(choice.Delta.ToolCalls)
	return out
}

func CanonicalToOpenAIChatResponse(resp ChatResponse) OpenAIChatResponse {
	finishReason := resp.FinishReason
	content := resp.Text
	message := &OpenAIMessageOut{Role: resp.Role, Content: &content}
	if len(resp.ToolCalls) > 0 {
		message.Content = nil
		message.ToolCalls = canonicalToolCallsToOpenAIToolCalls(resp.ToolCalls)
	}
	return OpenAIChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.Created,
		Model:   resp.Model,
		Choices: []OpenAIChoice{{
			Index:        0,
			Message:      message,
			FinishReason: &finishReason,
		}},
		Usage: canonicalUsageToOpenAIUsage(resp.Usage),
	}
}

func openAIUsageToCanonical(usage *OpenAIUsage) ChatUsage {
	if usage == nil {
		return ChatUsage{}
	}
	out := ChatUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
	if usage.PromptTokensDetails != nil {
		out.CachedTokens = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokensDetails != nil {
		out.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
	}
	return out
}

func openAIToolCallsToCanonical(toolCalls []OpenAIToolCall) []ChatToolCall {
	out := make([]ChatToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		out = append(out, ChatToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return out
}

func OpenAIChatRoleStreamChunk(id string, created int64, model string) OpenAIChatResponse {
	return OpenAIChatResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []OpenAIChoice{{Index: 0, Delta: &OpenAIStreamDelta{Role: "assistant"}}},
	}
}

func CanonicalToOpenAIStreamChunk(event ChatStreamEvent, id string, created int64, model string, includeUsage bool) OpenAIChatResponse {
	choice := OpenAIChoice{Index: 0, Delta: &OpenAIStreamDelta{}}
	if event.Text != "" {
		choice.Delta.Content = event.Text
	}
	if len(event.ToolCalls) > 0 {
		choice.Delta.ToolCalls = canonicalToolCallsToOpenAIStreamToolCalls(event.ToolCalls)
	}
	if event.FinishReason != "" {
		choice.FinishReason = &event.FinishReason
	}
	resp := OpenAIChatResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []OpenAIChoice{choice},
	}
	if includeUsage && !canonicalUsageIsZero(event.Usage) {
		resp.Usage = canonicalUsageToOpenAIUsage(event.Usage)
	}
	return resp
}

func canonicalUsageIsZero(usage ChatUsage) bool {
	return usage.PromptTokens == 0 &&
		usage.CompletionTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.CachedTokens == 0 &&
		usage.ReasoningTokens == 0
}

func canonicalUsageToOpenAIUsage(usage ChatUsage) *OpenAIUsage {
	out := &OpenAIUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
	if usage.CachedTokens != 0 {
		out.PromptTokensDetails = &OpenAIPromptTokensDetails{CachedTokens: usage.CachedTokens}
	}
	if usage.ReasoningTokens != 0 {
		out.CompletionTokensDetails = &OpenAICompletionTokensDetails{ReasoningTokens: usage.ReasoningTokens}
	}
	return out
}

func canonicalToolCallsToOpenAIToolCalls(toolCalls []ChatToolCall) []OpenAIToolCall {
	out := make([]OpenAIToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		out = append(out, OpenAIToolCall{
			ID:   call.ID,
			Type: "function",
			Function: OpenAIToolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}
	return out
}

func canonicalToolCallsToOpenAIStreamToolCalls(toolCalls []ChatToolCall) []OpenAIToolCall {
	out := make([]OpenAIToolCall, 0, len(toolCalls))
	for i, call := range toolCalls {
		index := i
		out = append(out, OpenAIToolCall{
			Index: &index,
			ID:    call.ID,
			Type:  "function",
			Function: OpenAIToolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}
	return out
}
