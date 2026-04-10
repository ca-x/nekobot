package qq

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/constant"
	"golang.org/x/oauth2"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
	"nekobot/pkg/logger"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "qq"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for qq dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for qq group scope")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v2/users/user-1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		content, _ := payload["content"].(string)
		if content == "" {
			t.Fatalf("expected qq content field, got %#v", payload["content"])
		}
		if content == "done" {
			t.Fatalf("expected tool trace in qq content, got %q", content)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m1","content":"ok"}`))
	}))
	defer server.Close()

	originalDomain := constant.APIDomain
	constant.APIDomain = server.URL
	defer func() { constant.APIDomain = originalDomain }()

	api := botgo.NewOpenAPI(
		"app-id",
		oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: "token",
			TokenType:   "QQBot",
			Expiry:      time.Now().Add(time.Hour),
		}),
	).WithTimeout(2 * time.Second)

	channel := &Channel{
		log:         newTestLogger(t),
		channelType: "qq",
		api:         api,
		running:     true,
	}

	err := channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "qq:user-1",
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
