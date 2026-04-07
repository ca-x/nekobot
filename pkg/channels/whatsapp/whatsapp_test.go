package whatsapp

import (
	"context"
	"testing"

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
