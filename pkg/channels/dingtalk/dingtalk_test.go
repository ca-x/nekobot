package dingtalk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
	"nekobot/pkg/logger"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "dingtalk"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for dingtalk dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for dingtalk group scope")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	channel := &Channel{channelType: "dingtalk", log: newTestLogger(t)}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		payload := string(body)
		if !strings.Contains(payload, "Tool call: read_file") {
			t.Fatalf("expected tool trace in dingtalk payload, got %q", payload)
		}
		if !strings.Contains(payload, "\\n\\ndone") {
			t.Fatalf("expected original reply after blank line, got %q", payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel.sessionWebhooks.Store("chat-1", server.URL)

	err := channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "dingtalk:chat-1",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}
