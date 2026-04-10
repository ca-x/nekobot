package teams

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "teams"}
	if channel.supportsNativeCommands() {
		t.Fatal("expected teams native commands to be disabled by capability matrix")
	}
	if channelcapabilities.IsCapabilityEnabled(
		channelcapabilities.GetDefaultCapabilitiesForChannel("teams"),
		channelcapabilities.CapabilityNativeCommands,
		channelcapabilities.CapabilityScopeGroup,
		false,
	) {
		t.Fatal("expected direct capability check to disable teams native commands")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	channel := &Channel{
		channelType: "teams",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				payload := string(body)
				if !strings.Contains(payload, "Tool call: read_file") {
					t.Fatalf("expected tool trace in teams payload, got %q", payload)
				}
				if !strings.Contains(payload, "\\n\\ndone") {
					t.Fatalf("expected original reply after blank line, got %q", payload)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
		accessToken:  "token",
		tokenExpires: time.Now().Add(time.Hour),
		contexts: map[string]conversationContext{
			"teams:conv-1": {
				ServiceURL:   "https://example.invalid",
				Conversation: "conv-1",
			},
		},
	}

	err := channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "teams:conv-1",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
