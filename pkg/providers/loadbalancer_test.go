package providers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeAdaptor struct {
	doRequestFunc  func(ctx context.Context, req *http.Request) ([]byte, error)
	doResponseFunc func(body []byte, info *RelayInfo) (*UnifiedResponse, error)
}

func (a *fakeAdaptor) Init(info *RelayInfo) error {
	_ = info
	return nil
}

func (a *fakeAdaptor) GetRequestURL(info *RelayInfo) (string, error) {
	_ = info
	return "https://example.com/v1/chat/completions", nil
}

func (a *fakeAdaptor) SetupRequestHeader(req *http.Request, info *RelayInfo) error {
	_ = info
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *fakeAdaptor) ConvertRequest(unified *UnifiedRequest, info *RelayInfo) ([]byte, error) {
	_ = unified
	_ = info
	return []byte(`{"ok":true}`), nil
}

func (a *fakeAdaptor) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	if a.doRequestFunc == nil {
		return []byte("ok"), nil
	}
	return a.doRequestFunc(ctx, req)
}

func (a *fakeAdaptor) DoResponse(body []byte, info *RelayInfo) (*UnifiedResponse, error) {
	if a.doResponseFunc != nil {
		return a.doResponseFunc(body, info)
	}
	return &UnifiedResponse{
		Content:      string(body),
		FinishReason: "stop",
	}, nil
}

func (a *fakeAdaptor) DoStreamResponse(ctx context.Context, reader io.Reader, handler StreamHandler, info *RelayInfo) error {
	_ = ctx
	_ = reader
	_ = handler
	_ = info
	return nil
}

func (a *fakeAdaptor) GetModelList() ([]string, error) {
	return nil, nil
}

func newFakeClient(doRequestFunc func(ctx context.Context, req *http.Request) ([]byte, error)) *Client {
	return &Client{
		adaptor: &fakeAdaptor{doRequestFunc: doRequestFunc},
		info: &RelayInfo{
			ProviderName: "fake",
			APIKey:       "fake-key",
			Model:        "fake-model",
		},
	}
}

func TestLoadBalancerChatSkipsCooldownProvider(t *testing.T) {
	lb := NewLoadBalancer()
	lb.cooldown = NewCooldownTracker()

	primaryCalls := 0
	fallbackCalls := 0
	if err := lb.RegisterProvider("primary", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		primaryCalls++
		return []byte("primary"), nil
	})); err != nil {
		t.Fatalf("register primary provider: %v", err)
	}
	if err := lb.RegisterProvider("fallback", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		fallbackCalls++
		return []byte("fallback"), nil
	})); err != nil {
		t.Fatalf("register fallback provider: %v", err)
	}

	lb.cooldown.MarkFailure("primary", FailoverReasonRateLimit)
	if lb.cooldown.IsAvailable("primary") {
		t.Fatalf("expected primary to be on cooldown")
	}

	resp, err := lb.Chat(context.Background(), &UnifiedRequest{Model: "gpt-test"}, []string{"primary", "fallback"})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp == nil || resp.Content != "fallback" {
		t.Fatalf("expected fallback response, got %#v", resp)
	}
	if primaryCalls != 0 {
		t.Fatalf("expected primary to be skipped during cooldown, got %d calls", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("expected fallback to be called once, got %d", fallbackCalls)
	}
}

func TestLoadBalancerChatStopsOnNonRetriableError(t *testing.T) {
	lb := NewLoadBalancer()
	lb.cooldown = NewCooldownTracker()

	primaryCalls := 0
	fallbackCalls := 0
	if err := lb.RegisterProvider("primary", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		primaryCalls++
		return nil, errors.New("status 400: invalid request format")
	})); err != nil {
		t.Fatalf("register primary provider: %v", err)
	}
	if err := lb.RegisterProvider("fallback", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		fallbackCalls++
		return []byte("fallback"), nil
	})); err != nil {
		t.Fatalf("register fallback provider: %v", err)
	}

	_, err := lb.Chat(context.Background(), &UnifiedRequest{Model: "gpt-test"}, []string{"primary", "fallback"})
	if err == nil {
		t.Fatalf("expected chat error")
	}

	var failErr *FailoverError
	if !errors.As(err, &failErr) {
		t.Fatalf("expected FailoverError, got %T: %v", err, err)
	}
	if failErr.Reason != FailoverReasonFormat {
		t.Fatalf("expected non-retriable format reason, got %s", failErr.Reason)
	}
	if primaryCalls != 1 {
		t.Fatalf("expected primary to be called once, got %d", primaryCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("expected fallback not to be called after non-retriable error, got %d", fallbackCalls)
	}
}

func TestLoadBalancerChatTracksRetryReasons(t *testing.T) {
	lb := NewLoadBalancer()
	lb.cooldown = NewCooldownTracker()

	if err := lb.RegisterProvider("primary", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		return nil, errors.New("status 429: too many requests")
	})); err != nil {
		t.Fatalf("register primary provider: %v", err)
	}
	if err := lb.RegisterProvider("fallback", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		return []byte("ok"), nil
	})); err != nil {
		t.Fatalf("register fallback provider: %v", err)
	}

	resp, err := lb.Chat(context.Background(), &UnifiedRequest{Model: "gpt-test"}, []string{"primary", "fallback"})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp == nil || resp.Content != "ok" {
		t.Fatalf("expected successful fallback response, got %#v", resp)
	}

	if got := lb.cooldown.FailureCount("primary", FailoverReasonRateLimit); got != 1 {
		t.Fatalf("expected one rate-limit failure for primary, got %d", got)
	}
	if got := lb.cooldown.ErrorCount("primary"); got != 1 {
		t.Fatalf("expected primary error count to be 1, got %d", got)
	}
	if lb.cooldown.CooldownRemaining("primary") <= 0 {
		t.Fatalf("expected primary cooldown remaining to be positive")
	}
}

func TestLoadBalancerChatAllProvidersInCooldown(t *testing.T) {
	lb := NewLoadBalancer()
	lb.cooldown = NewCooldownTracker()

	if err := lb.RegisterProvider("primary", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		return []byte("primary"), nil
	})); err != nil {
		t.Fatalf("register primary provider: %v", err)
	}
	if err := lb.RegisterProvider("fallback", newFakeClient(func(ctx context.Context, req *http.Request) ([]byte, error) {
		_ = ctx
		_ = req
		return []byte("fallback"), nil
	})); err != nil {
		t.Fatalf("register fallback provider: %v", err)
	}

	lb.cooldown.MarkFailure("primary", FailoverReasonRateLimit)
	lb.cooldown.MarkFailure("fallback", FailoverReasonRateLimit)

	_, err := lb.Chat(context.Background(), &UnifiedRequest{Model: "gpt-test"}, []string{"primary", "fallback"})
	if err == nil {
		t.Fatalf("expected error when all providers are in cooldown")
	}

	var exhaustedErr *FallbackExhaustedError
	if !errors.As(err, &exhaustedErr) {
		t.Fatalf("expected FallbackExhaustedError, got %T: %v", err, err)
	}
	if len(exhaustedErr.Attempts) != 2 {
		t.Fatalf("expected 2 skipped attempts, got %d", len(exhaustedErr.Attempts))
	}
	for i, attempt := range exhaustedErr.Attempts {
		if !attempt.Skipped {
			t.Fatalf("attempt %d should be marked skipped", i)
		}
		if attempt.Reason != FailoverReasonRateLimit {
			t.Fatalf("attempt %d expected rate_limit reason, got %s", i, attempt.Reason)
		}
		if attempt.Error == nil || !strings.Contains(attempt.Error.Error(), "cooldown") {
			t.Fatalf("attempt %d expected cooldown error, got %v", i, attempt.Error)
		}
	}
}

func TestCooldownTrackerResetsAfterFailureWindow(t *testing.T) {
	tracker := NewCooldownTracker()
	base := time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	current := base
	tracker.nowFunc = func() time.Time {
		return current
	}

	tracker.MarkFailure("primary", FailoverReasonRateLimit)
	tracker.MarkFailure("primary", FailoverReasonRateLimit)
	if got := tracker.ErrorCount("primary"); got != 2 {
		t.Fatalf("expected 2 errors before reset, got %d", got)
	}

	current = current.Add(defaultFailureWindow + time.Minute)
	tracker.MarkFailure("primary", FailoverReasonTimeout)

	if got := tracker.ErrorCount("primary"); got != 1 {
		t.Fatalf("expected error count reset to 1 after window, got %d", got)
	}
	if got := tracker.FailureCount("primary", FailoverReasonRateLimit); got != 0 {
		t.Fatalf("expected rate-limit counter reset, got %d", got)
	}
	if got := tracker.FailureCount("primary", FailoverReasonTimeout); got != 1 {
		t.Fatalf("expected timeout counter to be 1 after reset, got %d", got)
	}
}
