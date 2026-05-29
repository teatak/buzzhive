package buzzhive

import (
	"log"
	mathrand "math/rand"
	"strings"
	"time"
)

func (k *KeyState) Next(model string) (APIKey, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	for i := 0; i < len(k.keys); i++ {
		idx := (k.next + i) % len(k.keys)
		key := k.keys[idx]
		expires, cooling := k.exhausted[cooldownKey(model, key.Name)]
		if cooling && now.Before(expires) {
			continue
		}
		if cooling {
			delete(k.exhausted, cooldownKey(model, key.Name))
		}
		k.next = (idx + 1) % len(k.keys)
		return key, true
	}
	return APIKey{}, false
}

func (k *KeyState) MarkExhausted(model string, key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.exhausted[cooldownKey(model, key.Name)] = time.Now().Add(k.cooldown + time.Duration(mathrand.Intn(500))*time.Millisecond)
	delete(k.errors, cooldownKey(model, key.Name))
}

func (k *KeyState) MarkError(model string, key APIKey, status int, message string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(message) > 500 {
		message = message[:500]
	}
	k.errors[cooldownKey(model, key.Name)] = KeyError{
		Key:       key.Name,
		Model:     model,
		Status:    status,
		Message:   strings.TrimSpace(message),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func (k *KeyState) Remove(key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	nextKeys := k.keys[:0]
	for _, item := range k.keys {
		if item.ID != key.ID {
			nextKeys = append(nextKeys, item)
		}
	}
	k.keys = nextKeys
	if len(k.keys) == 0 {
		k.next = 0
	} else if k.next >= len(k.keys) {
		k.next = k.next % len(k.keys)
	}
	suffix := "::" + key.Name
	for item := range k.exhausted {
		if strings.HasSuffix(item, suffix) {
			delete(k.exhausted, item)
		}
	}
}

func (k *KeyState) ClearError(key APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for id, item := range k.errors {
		if item.Key == key.Name {
			delete(k.errors, id)
		}
	}
}

func (k *KeyState) Flush() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.exhausted = make(map[string]time.Time)
}

func (k *KeyState) Replace(keys []APIKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys = keys
	k.next = 0
	k.exhausted = make(map[string]time.Time)
	k.errors = make(map[string]KeyError)
}

func (k *KeyState) SnapshotExhausted() map[string]string {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	out := make(map[string]string)
	for key, expires := range k.exhausted {
		if now.After(expires) {
			delete(k.exhausted, key)
			continue
		}
		out[key] = expires.Format(time.RFC3339)
	}
	return out
}

func (k *KeyState) SnapshotErrors() map[string]KeyError {
	k.mu.Lock()
	defer k.mu.Unlock()

	out := make(map[string]KeyError, len(k.errors))
	for key, item := range k.errors {
		out[key] = item
	}
	return out
}

func (s *Server) refreshKeyStateStats() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.Exhausted = s.keyState.SnapshotExhausted()
	s.stats.KeyErrors = s.keyState.SnapshotErrors()
}

func (s *Server) disableAPIKey(key APIKey, status int, errorCode, errorMessage string, errorBody []byte) {
	if err := s.store.DisableAPIKey(key.ID, status, errorCode, errorMessage, string(errorBody)); err != nil {
		log.Printf("disable api key %s after %d %s: %v", key.Name, status, errorCode, err)
		return
	}
	s.keyState.Remove(key)
	log.Printf("disabled api key %s after %d %s", key.Name, status, errorCode)
}
func cooldownKey(model, keyName string) string {
	return model + "::" + keyName
}
