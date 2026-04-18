package openai

import (
	"net/http"
	"strings"
	"testing"

	"nekobot/pkg/providers"
)

func TestSetupRequestHeaderPreservesReservedHeaders(t *testing.T) {
	t.Parallel()

	adaptor := New()
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	info := &providers.RelayInfo{
		APIKey: "expected-token",
		Headers: map[string]string{
			"Authorization": "Bearer attacker-token",
			"authorization": "Bearer lowercase-attacker-token",
			"Content-Type":  "text/plain",
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

func TestGetRequestURLTrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	adaptor := New()
	url, err := adaptor.GetRequestURL(&providers.RelayInfo{APIBase: "https://api.openai.com/v1/"})
	if err != nil {
		t.Fatalf("get request url: %v", err)
	}

	if url != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("expected normalized request URL, got %q", url)
	}
}

func TestDoResponse_HTMLBodyReturnsUsefulError(t *testing.T) {
	adaptor := New()

	_, err := adaptor.DoResponse([]byte("<html><title>502 Bad Gateway</title></html>"), &providers.RelayInfo{})
	if err == nil {
		t.Fatal("expected HTML response error")
	}
	if strings.Contains(err.Error(), "invalid character <") {
		t.Fatalf("expected upgraded error, got %v", err)
	}
	if !strings.Contains(err.Error(), "HTML error page") {
		t.Fatalf("expected HTML error page hint, got %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected status hint in error, got %v", err)
	}
}
