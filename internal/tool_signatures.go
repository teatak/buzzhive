package buzzhive

import (
	"encoding/json"
	"strings"

	"github.com/teatak/buzzhive/internal/protocol"
)

func (s *Server) rememberToolSignatures(toolCalls []protocol.ChatToolCall) {
	if len(toolCalls) == 0 {
		return
	}
	s.toolSigMu.Lock()
	defer s.toolSigMu.Unlock()
	if s.toolSigs == nil {
		s.toolSigs = make(map[string]string)
	}
	for _, call := range toolCalls {
		if call.Signature == "" {
			continue
		}
		if id := strings.TrimSpace(call.ID); id != "" {
			s.toolSigs[toolSignatureIDKey(id)] = call.Signature
		}
		if name := strings.TrimSpace(call.Name); name != "" {
			s.toolSigs[toolSignatureFunctionKey(name, call.Arguments)] = call.Signature
		}
	}
}

func (s *Server) applyToolSignatures(req *protocol.ChatRequest) {
	s.toolSigMu.Lock()
	defer s.toolSigMu.Unlock()
	if len(s.toolSigs) == 0 {
		return
	}
	for messageIndex := range req.Messages {
		for partIndex := range req.Messages[messageIndex].Parts {
			part := &req.Messages[messageIndex].Parts[partIndex]
			if part.Type != "tool_call" || part.Signature != "" {
				continue
			}
			if id := strings.TrimSpace(part.ToolCallID); id != "" {
				if signature := s.toolSigs[toolSignatureIDKey(id)]; signature != "" {
					part.Signature = signature
					continue
				}
			}
			if name := strings.TrimSpace(part.Name); name != "" {
				part.Signature = s.toolSigs[toolSignatureFunctionKey(name, string(part.Arguments))]
			}
		}
	}
}

func toolSignatureIDKey(id string) string {
	return "id:" + strings.TrimSpace(id)
}

func toolSignatureFunctionKey(name, args string) string {
	return "fn:" + strings.TrimSpace(name) + ":" + normalizeToolSignatureArgs(args)
}

func normalizeToolSignatureArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return string(normalized)
}
