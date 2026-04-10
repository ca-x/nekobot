package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestHandleInboundTreatsSlashCommandAsPlainTextWhenNativeCommandsDisabled(t *testing.T) {
	log := newTestLogger(t)
	messageBus := &stubBus{}
	commandRegistry := commands.NewRegistry()

	commandCalls := 0
	if err := commandRegistry.Register(&commands.Command{
		Name: "help",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			commandCalls++
			return commands.CommandResponse{Content: "help"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	channel, err := NewChannel(log, config.WhatsAppConfig{
		Enabled:   true,
		BridgeURL: "ws://example.invalid",
	}, messageBus, commandRegistry)
	if err != nil {
		t.Fatalf("NewChannel failed: %v", err)
	}

	channel.handleInbound(map[string]interface{}{
		"id":        "msg-1",
		"from":      "user-1",
		"from_name": "User One",
		"chat":      "chat-1",
		"content":   "/help topic",
	})

	if commandCalls != 0 {
		t.Fatalf("expected native command handler to stay disabled, got %d calls", commandCalls)
	}
	if len(messageBus.inbound) != 1 {
		t.Fatalf("expected one inbound message, got %d", len(messageBus.inbound))
	}
	msg := messageBus.inbound[0]
	if msg.ChannelID != "whatsapp" {
		t.Fatalf("expected channel whatsapp, got %q", msg.ChannelID)
	}
	if msg.SessionID != "whatsapp:chat-1" {
		t.Fatalf("expected session whatsapp:chat-1, got %q", msg.SessionID)
	}
	if msg.Content != "/help topic" {
		t.Fatalf("expected original slash command content, got %q", msg.Content)
	}
}

func TestAccountScopedHandleInboundStillTreatsSlashCommandAsPlainTextWhenNativeCommandsDisabled(t *testing.T) {
	log := newTestLogger(t)
	messageBus := &stubBus{}
	commandRegistry := commands.NewRegistry()

	commandCalls := 0
	if err := commandRegistry.Register(&commands.Command{
		Name: "help",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			commandCalls++
			return commands.CommandResponse{Content: "help"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	channel, err := NewAccountChannel(log, config.WhatsAppConfig{
		Enabled:   true,
		BridgeURL: "ws://example.invalid",
	}, messageBus, commandRegistry, "whatsapp:bridge-a", "WhatsApp Bridge A")
	if err != nil {
		t.Fatalf("NewAccountChannel failed: %v", err)
	}

	channel.handleInbound(map[string]interface{}{
		"id":        "msg-1",
		"from":      "user-1",
		"from_name": "User One",
		"chat":      "chat-1",
		"content":   "/help topic",
	})

	if commandCalls != 0 {
		t.Fatalf("expected native command handler to stay disabled for account-scoped channel, got %d calls", commandCalls)
	}
	if len(messageBus.inbound) != 1 {
		t.Fatalf("expected one inbound message, got %d", len(messageBus.inbound))
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	log := newTestLogger(t)
	messageBus := &stubBus{}
	commandRegistry := commands.NewRegistry()

	channel, err := NewChannel(log, config.WhatsAppConfig{
		Enabled:   true,
		BridgeURL: "ws://example.invalid",
	}, messageBus, commandRegistry)
	if err != nil {
		t.Fatalf("NewChannel failed: %v", err)
	}

	upgrader := websocket.Upgrader{}
	received := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade websocket: %v", err)
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket message: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		received <- payload
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket server: %v", err)
	}
	defer conn.Close()

	channel.conn = conn

	err = channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "whatsapp:chat-1",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	payload := <-received
	content, _ := payload["content"].(string)
	if !strings.Contains(content, "Tool call: read_file") {
		t.Fatalf("expected tool trace in whatsapp payload, got %q", content)
	}
	if !strings.Contains(content, "\n\ndone") {
		t.Fatalf("expected original reply after blank line, got %q", content)
	}
}

type stubBus struct {
	inbound []*bus.Message
}

func (b *stubBus) Start() error                                                  { return nil }
func (b *stubBus) Stop() error                                                   { return nil }
func (b *stubBus) RegisterInboundHandler(channelID string, handler bus.Handler)  {}
func (b *stubBus) UnregisterInboundHandlers(channelID string)                    {}
func (b *stubBus) RegisterOutboundHandler(channelID string, handler bus.Handler) {}
func (b *stubBus) UnregisterOutboundHandlers(channelID string)                   {}
func (b *stubBus) RegisterHandler(channelID string, handler bus.Handler)         {}
func (b *stubBus) UnregisterHandlers(channelID string)                           {}
func (b *stubBus) SendInbound(msg *bus.Message) error {
	b.inbound = append(b.inbound, msg)
	return nil
}
func (b *stubBus) SendOutbound(msg *bus.Message) error { return nil }
func (b *stubBus) GetMetrics() map[string]uint64       { return map[string]uint64{} }

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
