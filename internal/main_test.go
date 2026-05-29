package buzzhive

import (
	"net/http"
	"testing"
)

func TestShouldDisableAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		errorCode    string
		errorMessage string
		body         string
		want         bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: true},
		{name: "forbidden", status: http.StatusForbidden, want: true},
		{name: "invalid api key reason", status: http.StatusBadRequest, body: `{"error":{"status":"INVALID_ARGUMENT","message":"API key not valid. Please pass a valid API key.","details":[{"reason":"API_KEY_INVALID"}]}}`, want: true},
		{name: "invalid api key message", status: http.StatusBadRequest, errorMessage: "API key not valid. Please pass a valid API key.", want: true},
		{name: "ordinary bad request", status: http.StatusBadRequest, errorMessage: "invalid request body", want: false},
		{name: "too many requests", status: http.StatusTooManyRequests, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldDisableAPIKey(tt.status, tt.errorCode, tt.errorMessage, []byte(tt.body))
			if got != tt.want {
				t.Fatalf("shouldDisableAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "too many requests", status: http.StatusTooManyRequests, want: true},
		{name: "not found", status: http.StatusNotFound, want: false},
		{name: "bad request", status: http.StatusBadRequest, want: false},
		{name: "server error", status: http.StatusInternalServerError, want: true},
		{name: "bad gateway", status: http.StatusBadGateway, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetry(tt.status)
			if got != tt.want {
				t.Fatalf("shouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
