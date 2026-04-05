package feishu

import (
	"context"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"nekobot/pkg/bus"
	"nekobot/pkg/channelcapabilities"
	"nekobot/pkg/commands"
	"nekobot/pkg/logger"
)

func TestHandleMessageReceiveFallsBackToBusWhenNativeCommandsDisabled(t *testing.T) {
	log := newFeishuTestLogger(t)
	commandCalls := 0
	registry := commands.NewRegistry()
	if err := registry.Register(&commands.Command{
		Name: "install",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			commandCalls++
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	fakeBus := &feishuTestBus{}
	ch := &Channel{
		log:          log,
		bus:          fakeBus,
		commands:     registry,
		capabilities: channelcapabilities.ChannelCapabilities{NativeCommands: channelcapabilities.CapabilityScopeOff},
	}

	if err := ch.handleMessageReceive(context.Background(), feishuTestMessageEvent("/install repo", "chat-1", "p2p", "user-1")); err != nil {
		t.Fatalf("handleMessageReceive: %v", err)
	}

	if commandCalls != 0 {
		t.Fatalf("expected command handler to remain unused, got %d calls", commandCalls)
	}
	if len(fakeBus.inbound) != 1 {
		t.Fatalf("expected one inbound bus message, got %d", len(fakeBus.inbound))
	}
	if fakeBus.inbound[0].Content != "/install repo" {
		t.Fatalf("expected slash command to reach bus unchanged, got %q", fakeBus.inbound[0].Content)
	}
}

func TestHandleMessageReceiveExecutesCommandWhenNativeCommandsEnabled(t *testing.T) {
	log := newFeishuTestLogger(t)
	commandCalls := 0
	registry := commands.NewRegistry()
	if err := registry.Register(&commands.Command{
		Name: "install",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			commandCalls++
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch := &Channel{
		log:      log,
		bus:      &feishuTestBus{},
		commands: registry,
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected nil-client send path panic after command execution in test harness")
		}
	}()

	_ = ch.handleMessageReceive(context.Background(), feishuTestMessageEvent("/install repo", "chat-1", "p2p", "user-1"))

	if commandCalls != 1 {
		t.Fatalf("expected command handler call, got %d", commandCalls)
	}
}

type feishuTestBus struct {
	inbound []*bus.Message
}

func (b *feishuTestBus) Start() error { return nil }

func (b *feishuTestBus) Stop() error { return nil }

func (b *feishuTestBus) RegisterInboundHandler(channelID string, handler bus.Handler) {}

func (b *feishuTestBus) UnregisterInboundHandlers(channelID string) {}

func (b *feishuTestBus) RegisterOutboundHandler(channelID string, handler bus.Handler) {}

func (b *feishuTestBus) UnregisterOutboundHandlers(channelID string) {}

func (b *feishuTestBus) RegisterHandler(channelID string, handler bus.Handler) {}

func (b *feishuTestBus) UnregisterHandlers(channelID string) {}

func (b *feishuTestBus) SendInbound(msg *bus.Message) error {
	b.inbound = append(b.inbound, msg)
	return nil
}

func (b *feishuTestBus) SendOutbound(msg *bus.Message) error { return nil }

func (b *feishuTestBus) GetMetrics() map[string]uint64 { return map[string]uint64{} }

func feishuTestMessageEvent(content, chatID, chatType, userID string) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					UserId: &userID,
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   stringPtr("om_123"),
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: stringPtr(larkim.MsgTypeText),
				Content:     stringPtr(`{"text":"` + content + `"}`),
			},
		},
	}
}

func newFeishuTestLogger(t *testing.T) *logger.Logger {
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

func stringPtr(value string) *string {
	return &value
}
