package generic

import (
	"net/http"
	"strings"
	"testing"

	"nekobot/pkg/providers"
)

func TestSetupRequestHeaderPreservesReservedHeaders(t *testing.T) {
	t.Parallel()

	adaptor := New()
	req, err := http.NewRequest(http.MethodPost, "https://example.com/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	info := &providers.RelayInfo{
		APIKey: "expected-token",
		Headers: map[string]string{
			"Authorization": "Bearer attacker-token",
			"content-type":  "text/plain",
			"X-Custom":      "custom-value",
		},
	}

	if err := adaptor.SetupRequestHeader(req, info); err != nil {
		t.Fatalf("setup request header: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer expected-token" {
		t.Fatalf("expected Authorization to preserve adaptor value, got %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}
	if got := req.Header.Get("X-Custom"); got != "custom-value" {
		t.Fatalf("expected X-Custom custom-value, got %q", got)
	}
}

func TestSetupRequestHeaderAllowsMissingAPIKey(t *testing.T) {
	t.Parallel()

	adaptor := New()
	req, err := http.NewRequest(http.MethodPost, "https://example.com/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	info := &providers.RelayInfo{
		Headers: map[string]string{
			"Authorization": "Bearer attacker-token",
			"X-Custom":      "custom-value",
		},
	}

	if err := adaptor.SetupRequestHeader(req, info); err != nil {
		t.Fatalf("setup request header: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected Authorization to stay empty without API key, got %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}
	if got := req.Header.Get("X-Custom"); got != "custom-value" {
		t.Fatalf("expected X-Custom custom-value, got %q", got)
	}
}

func TestGetRequestURLTrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	adaptor := New()
	url, err := adaptor.GetRequestURL(&providers.RelayInfo{APIBase: "https://example.com/v1/"})
	if err != nil {
		t.Fatalf("get request url: %v", err)
	}

	if url != "https://example.com/v1/chat/completions" {
		t.Fatalf("expected normalized request URL, got %q", url)
	}
}

func TestParseError_HTMLBodyReturnsStructuredProviderError(t *testing.T) {
	err := parseError(403, []byte("<html><body>403 Forbidden</body></html>"))
	if err == nil {
		t.Fatal("expected provider error")
	}

	resp, ok := err.(*providers.ErrorResponse)
	if !ok {
		t.Fatalf("expected ErrorResponse, got %T", err)
	}
	if strings.Contains(resp.Message, "invalid character <") {
		t.Fatalf("expected upgraded message, got %q", resp.Message)
	}
	if !strings.Contains(resp.Message, "HTML error page") {
		t.Fatalf("expected HTML error page hint, got %q", resp.Message)
	}
}
