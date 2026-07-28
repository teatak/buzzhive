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
	Role             string           `json:"role"`
	Content          *string          `json:"content"`
	Refusal          *string          `json:"refusal,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIStreamDelta struct {
	Role             string           `json:"role,omitempty"`
	Content          string           `json:"content,omitempty"`
	Refusal          string           `json:"refusal,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens            int                            `json:"prompt_tokens"`
	CompletionTokens        int                            `json:"completion_tokens"`
	TotalTokens             int                            `json:"total_tokens"`
	PromptCacheHitTokens    int                            `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens   int                            `json:"prompt_cache_miss_tokens,omitempty"`
	PromptTokensDetails     *OpenAIPromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAICompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type OpenAIPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type OpenAICompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

func OpenAIChatResponseToCanonical(resp OpenAIChatResponse) CanonicalResponse {
	out := CanonicalResponse{
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
	if choice.Message.Refusal != nil {
		out.Refusal = *choice.Message.Refusal
	}
	out.Reasoning = choice.Message.ReasoningContent
	out.ToolCalls = openAIToolCallsToCanonical(choice.Message.ToolCalls)
	return out
}

func OpenAIChatStreamChunkToCanonical(chunk OpenAIChatResponse) []CanonicalStreamEvent {
	out := make([]CanonicalStreamEvent, 0, 4)
	usage := openAIUsageToCanonical(chunk.Usage)
	for _, choice := range chunk.Choices {
		if choice.Delta != nil {
			if choice.Delta.Role != "" {
				out = append(out, CanonicalStreamEvent{
					Type:  CanonicalStreamMessageStart,
					Role:  choice.Delta.Role,
					Index: choice.Index,
				})
			}
			if choice.Delta.Content != "" {
				out = append(out, CanonicalStreamEvent{
					Type:  CanonicalStreamTextDelta,
					Index: choice.Index,
					Delta: choice.Delta.Content,
				})
			}
			if choice.Delta.Refusal != "" {
				out = append(out, CanonicalStreamEvent{
					Type:  CanonicalStreamRefusalDelta,
					Index: choice.Index,
					Delta: choice.Delta.Refusal,
				})
			}
			if choice.Delta.ReasoningContent != "" {
				out = append(out, CanonicalStreamEvent{
					Type:  CanonicalStreamReasoningDelta,
					Index: choice.Index,
					Delta: choice.Delta.ReasoningContent,
				})
			}
			for _, call := range choice.Delta.ToolCalls {
				index := choice.Index
				if call.Index != nil {
					index = *call.Index
				}
				if call.ID != "" || call.Function.Name != "" {
					out = append(out, CanonicalStreamEvent{
						Type:   CanonicalStreamToolCallStart,
						Index:  index,
						CallID: call.ID,
						Name:   call.Function.Name,
					})
				}
				if call.Function.Arguments != "" {
					out = append(out, CanonicalStreamEvent{
						Type:   CanonicalStreamToolArgumentsDelta,
						Index:  index,
						CallID: call.ID,
						Name:   call.Function.Name,
						Delta:  call.Function.Arguments,
					})
				}
			}
		}
		if choice.FinishReason != nil {
			out = append(out, CanonicalStreamEvent{
				Type:         CanonicalStreamResponseDone,
				Index:        choice.Index,
				FinishReason: *choice.FinishReason,
			})
		}
	}
	if !usage.IsZero() {
		out = append(out, CanonicalStreamEvent{Type: CanonicalStreamUsage, Usage: usage})
	}
	return out
}

func CanonicalToOpenAIChatResponse(resp CanonicalResponse) OpenAIChatResponse {
	finishReason := resp.FinishReason
	content := resp.Text
	message := &OpenAIMessageOut{
		Role:             resp.Role,
		Content:          &content,
		ReasoningContent: resp.Reasoning,
	}
	if resp.Refusal != "" {
		message.Content = nil
		message.Refusal = &resp.Refusal
	}
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

func openAIUsageToCanonical(usage *OpenAIUsage) CanonicalUsage {
	if usage == nil {
		return CanonicalUsage{}
	}
	out := CanonicalUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CachedTokens:     usage.PromptCacheHitTokens,
	}
	if usage.PromptTokensDetails != nil {
		out.CachedTokens = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokensDetails != nil {
		out.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
	}
	return out
}

func openAIToolCallsToCanonical(toolCalls []OpenAIToolCall) []CanonicalToolCall {
	out := make([]CanonicalToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		out = append(out, CanonicalToolCall{
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

func CanonicalToOpenAIStreamChunk(event CanonicalStreamEvent, id string, created int64, model string, includeUsage bool) (OpenAIChatResponse, bool) {
	resp := OpenAIChatResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
	}
	choice := OpenAIChoice{Index: event.Index, Delta: &OpenAIStreamDelta{}}
	switch event.Type {
	case CanonicalStreamMessageStart:
		choice.Delta.Role = event.Role
	case CanonicalStreamTextDelta:
		choice.Delta.Content = event.Delta
	case CanonicalStreamRefusalDelta:
		choice.Delta.Refusal = event.Delta
	case CanonicalStreamReasoningDelta:
		choice.Delta.ReasoningContent = event.Delta
	case CanonicalStreamToolCallStart:
		index := event.Index
		choice.Delta.ToolCalls = []OpenAIToolCall{{
			Index: &index,
			ID:    event.CallID,
			Type:  "function",
			Function: OpenAIToolCallFunction{
				Name: event.Name,
			},
		}}
	case CanonicalStreamToolArgumentsDelta:
		index := event.Index
		choice.Delta.ToolCalls = []OpenAIToolCall{{
			Index: &index,
			Function: OpenAIToolCallFunction{
				Arguments: event.Delta,
			},
		}}
	case CanonicalStreamResponseDone:
		choice.FinishReason = &event.FinishReason
	case CanonicalStreamUsage:
		if !includeUsage || event.Usage.IsZero() {
			return OpenAIChatResponse{}, false
		}
		resp.Usage = canonicalUsageToOpenAIUsage(event.Usage)
		return resp, true
	default:
		return OpenAIChatResponse{}, false
	}
	resp.Choices = []OpenAIChoice{choice}
	if includeUsage && !event.Usage.IsZero() {
		resp.Usage = canonicalUsageToOpenAIUsage(event.Usage)
	}
	return resp, true
}

func canonicalUsageToOpenAIUsage(usage CanonicalUsage) *OpenAIUsage {
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

func canonicalToolCallsToOpenAIToolCalls(toolCalls []CanonicalToolCall) []OpenAIToolCall {
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
