package googlechat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chat "google.golang.org/api/chat/v1"
	"google.golang.org/api/option"

	"nekobot/pkg/bus"
	"nekobot/pkg/logger"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "googlechat"}

	if !channel.supportsNativeCommands() {
		t.Fatal("expected native commands enabled for googlechat")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/spaces/space-1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		payload := string(body)
		if !strings.Contains(payload, "Tool call: read_file") {
			t.Fatalf("expected tool trace in googlechat payload, got %q", payload)
		}
		if !strings.Contains(payload, "\\n\\ndone") {
			t.Fatalf("expected original reply after blank line, got %q", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"spaces/space-1/messages/msg-1","text":"ok"}`))
	}))
	defer server.Close()

	service, err := chat.NewService(
		context.Background(),
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("new google chat service: %v", err)
	}

	channel := &Channel{channelType: "googlechat", service: service, log: newTestLogger(t)}
	err = channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "googlechat:spaces/space-1",
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
