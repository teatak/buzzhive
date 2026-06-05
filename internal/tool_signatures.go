package buzzhive

func (s *Server) rememberToolSignatures(toolCalls []canonicalToolCall) {
	if len(toolCalls) == 0 {
		return
	}
	s.toolSigMu.Lock()
	defer s.toolSigMu.Unlock()
	if s.toolSigs == nil {
		s.toolSigs = make(map[string]string)
	}
	for _, call := range toolCalls {
		if call.ID != "" && call.Signature != "" {
			s.toolSigs[call.ID] = call.Signature
		}
	}
}

func (s *Server) applyToolSignatures(req *canonicalChatRequest) {
	s.toolSigMu.Lock()
	defer s.toolSigMu.Unlock()
	if len(s.toolSigs) == 0 {
		return
	}
	for messageIndex := range req.Messages {
		for partIndex := range req.Messages[messageIndex].Parts {
			part := &req.Messages[messageIndex].Parts[partIndex]
			if part.Type != "tool_call" || part.Signature != "" || part.ToolCallID == "" {
				continue
			}
			part.Signature = s.toolSigs[part.ToolCallID]
		}
	}
}
