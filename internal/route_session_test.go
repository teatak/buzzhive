package buzzhive

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testProviderFunc func(context.Context, ProviderRequest, APIKey) (*http.Response, error)

func (f testProviderFunc) Forward(ctx context.Context, req ProviderRequest, key APIKey) (*http.Response, error) {
	return f(ctx, req, key)
}

func TestProviderTargetLoopKeepsRouteSessionSticky(t *testing.T) {
	var routedModels []string
	srv := &Server{
		providers: map[string]Provider{
			providerGemini: testProviderFunc(func(_ context.Context, req ProviderRequest, _ APIKey) (*http.Response, error) {
				routedModels = append(routedModels, req.Model)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			}),
		},
		routeSessions: make(map[string]RouteSession),
		keyState: &KeyState{
			keys: []APIKey{
				{Name: "k1", Key: "secret-1", ProviderName: providerGemini},
				{Name: "k2", Key: "secret-2", ProviderName: providerGemini},
			},
			cooldown:     time.Minute,
			rpdCooldown:  time.Hour,
			exhausted:    make(map[string]time.Time),
			cooldownHits: make(map[string]int),
			rpdLike:      make(map[string]bool),
			errors:       make(map[string]KeyError),
		},
	}
	srv.cfg.Retry.MaxAttempts = 4

	user := AuthToken{ID: 7, Name: "alice"}
	targetA := RouteTarget{ID: 1, ProviderName: providerGemini, UpstreamModel: "upstream-a"}
	targetB := RouteTarget{ID: 2, ProviderName: providerGemini, UpstreamModel: "upstream-b"}
	buildReq := func(target RouteTarget) ProviderRequest {
		return ProviderRequest{ProviderName: target.ProviderName, Model: target.UpstreamModel}
	}

	first := srv.doProviderTargetLoop(context.Background(), user, "mimo-v2.5", []RouteTarget{targetA, targetB}, buildReq)
	if !first.OK {
		t.Fatalf("first result = %+v", first)
	}
	second := srv.doProviderTargetLoop(context.Background(), user, "mimo-v2.5", []RouteTarget{targetB, targetA}, buildReq)
	if !second.OK {
		t.Fatalf("second result = %+v", second)
	}

	if len(routedModels) != 2 || routedModels[0] != "upstream-a" || routedModels[1] != "upstream-a" {
		t.Fatalf("routed models = %+v, want sticky upstream-a twice", routedModels)
	}
}

func TestProviderAttemptLoopCapsAttemptsByMatchingKeyCount(t *testing.T) {
	attempts := 0
	srv := &Server{
		providers: map[string]Provider{
			providerOpenAI: testProviderFunc(func(_ context.Context, _ ProviderRequest, _ APIKey) (*http.Response, error) {
				attempts++
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Too Many Requests"}}`)),
				}, nil
			}),
		},
		keyState: &KeyState{
			keys: []APIKey{
				{Name: "only", Key: "secret", ProviderName: providerOpenAI},
			},
			cooldown:     time.Minute,
			rpdCooldown:  time.Hour,
			exhausted:    make(map[string]time.Time),
			cooldownHits: make(map[string]int),
			rpdLike:      make(map[string]bool),
			errors:       make(map[string]KeyError),
		},
	}
	srv.cfg.Retry.MaxAttempts = 8

	target := RouteTarget{ProviderName: providerOpenAI, UpstreamModel: "gpt-oss"}
	result := srv.doProviderAttemptLoop(context.Background(), AuthToken{}, "public-model", target, ProviderRequest{})

	if attempts != 1 || result.Attempts != 1 {
		t.Fatalf("attempts = provider:%d result:%d, want 1", attempts, result.Attempts)
	}
}

func TestRotateRouteTargetsWeighted(t *testing.T) {
	srv := &Server{routeNext: make(map[string]int)}
	targets := []RouteTarget{
		{ID: 1, ModelName: "mimo-v2.5", SelectionPolicy: "weighted", UpstreamModel: "a", Weight: 2},
		{ID: 2, ModelName: "mimo-v2.5", SelectionPolicy: "weighted", UpstreamModel: "b", Weight: 1},
	}

	first := srv.rotateRouteTargets("mimo-v2.5", targets)
	second := srv.rotateRouteTargets("mimo-v2.5", targets)
	third := srv.rotateRouteTargets("mimo-v2.5", targets)

	got := []string{first[0].UpstreamModel, second[0].UpstreamModel, third[0].UpstreamModel}
	want := []string{"a", "a", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("weighted rotation = %v, want %v", got, want)
		}
	}
}
