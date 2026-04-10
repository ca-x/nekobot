package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
	"nekobot/pkg/logger"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "discord"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for discord dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for discord group scope")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/v9/channels/C123/messages" {
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
		if !strings.Contains(content, "Tool call: read_file") {
			t.Fatalf("expected tool trace in discord content, got %q", content)
		}
		if !strings.Contains(content, "\n\ndone") {
			t.Fatalf("expected original reply after blank line, got %q", content)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m1","channel_id":"C123","content":"ok"}`))
	}))
	defer server.Close()

	discordgo.EndpointDiscord = server.URL + "/"
	discordgo.EndpointAPI = discordgo.EndpointDiscord + "api/v" + discordgo.APIVersion + "/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.Client = server.Client()

	channel := &Channel{
		log:         newTestLogger(t),
		channelType: "discord",
		session:     session,
	}

	err = channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "discord:C123",
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
