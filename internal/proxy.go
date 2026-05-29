package buzzhive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) proxy(w http.ResponseWriter, r *http.Request, body []byte, user AuthToken, originalModel string) {
	models := []string{originalModel}
	if originalModel == "auto" || originalModel == "gemini-auto" {
		models = s.cfg.Models.Auto
	}

	var lastErrBody []byte
	var lastStatus int
	var chain []string
	attempts := 0
	requestID := randomHex(8)

	for _, model := range models {
		for attempts < s.cfg.Retry.MaxAttempts {
			key, ok := s.keyState.Next(model)
			if !ok {
				chain = append(chain, model+":all-keys-cooling")
				break
			}

			attempts++
			attemptStartedAt := time.Now()
			resp, err := s.forward(r, body, originalModel, model, key)
			if err != nil {
				s.recordModelUsage(user, key, model, 0, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, "", err.Error(), "")
				chain = append(chain, fmt.Sprintf("%s(%s):%s", model, key.Name, err.Error()))
				s.keyState.MarkError(model, key, 0, err.Error())
				s.refreshKeyStateStats()
				continue
			}

			var errorCode, errorMessage string
			disableKey := false
			if resp.StatusCode >= 400 {
				lastStatus = resp.StatusCode
				lastErrBody = drain(resp.Body, 64*1024)
				errorCode, errorMessage = parseUpstreamError(lastErrBody)
				disableKey = shouldDisableAPIKey(resp.StatusCode, errorCode, errorMessage, lastErrBody)
				s.recordModelUsage(user, key, model, resp.StatusCode, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, errorCode, errorMessage, string(lastErrBody))
				if resp.StatusCode == http.StatusTooManyRequests {
					s.keyState.MarkExhausted(model, key)
				} else if disableKey {
					s.keyState.MarkError(model, key, resp.StatusCode, string(lastErrBody))
					s.disableAPIKey(key, resp.StatusCode, errorCode, errorMessage, lastErrBody)
				} else {
					s.keyState.MarkError(model, key, resp.StatusCode, string(lastErrBody))
				}
				s.refreshKeyStateStats()
			} else {
				s.recordModelUsage(user, key, model, resp.StatusCode, time.Since(attemptStartedAt), requestID, attempts, attemptStartedAt, "", "", "")
			}

			if shouldRetry(resp.StatusCode) {
				resp.Body.Close()
				chain = append(chain, fmt.Sprintf("%s(%s):%d", model, key.Name, resp.StatusCode))
				continue
			}
			if disableKey {
				resp.Body.Close()
				chain = append(chain, fmt.Sprintf("%s(%s):disabled:%d", model, key.Name, resp.StatusCode))
				continue
			}
			if resp.StatusCode >= 400 {
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(lastErrBody))
			} else {
				s.keyState.ClearError(key)
				s.refreshKeyStateStats()
			}

			defer resp.Body.Close()
			copyResponseHeaders(w.Header(), resp.Header)
			setCORS(w.Header())
			w.Header().Set("X-Proxy-Debug", strings.Join(chain, " -> "))
			w.Header().Set("X-Proxy-Key", key.Name)
			w.WriteHeader(resp.StatusCode)
			startedAt := time.Now()
			io.Copy(w, resp.Body)
			s.recordUsage(user, key, model, resp.StatusCode, time.Since(startedAt))
			return
		}
	}

	if lastStatus == 0 {
		lastStatus = http.StatusTooManyRequests
	}
	reason := "all keys or models failed"
	if attempts >= s.cfg.Retry.MaxAttempts {
		reason = "retry max_attempts reached"
		chain = append(chain, fmt.Sprintf("max-attempts:%d", s.cfg.Retry.MaxAttempts))
	}
	w.Header().Set("X-Proxy-Debug", strings.Join(chain, " -> "))
	writeJSON(w, lastStatus, map[string]any{
		"error":        reason,
		"attempts":     attempts,
		"max_attempts": s.cfg.Retry.MaxAttempts,
		"chain":        chain,
		"body":         string(lastErrBody),
	})
}

func (s *Server) forward(r *http.Request, body []byte, originalModel, currentModel string, key APIKey) (*http.Response, error) {
	upstream := *s.upstream
	upstream.Path = r.URL.Path
	upstream.RawQuery = r.URL.RawQuery
	if currentModel != originalModel {
		upstream.Path = strings.Replace(upstream.Path, "/models/"+originalModel, "/models/"+currentModel, 1)
	}
	q := upstream.Query()
	q.Set("key", key.Key)
	q.Del("model")
	upstream.RawQuery = q.Encode()

	ctx, cancel := context.WithCancel(r.Context())
	req, err := http.NewRequestWithContext(ctx, r.Method, upstream.String(), bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header = cleanHeaders(r.Header)
	resp, err := s.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}
