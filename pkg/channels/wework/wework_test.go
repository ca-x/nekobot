package wework

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestProcessMessageTreatsSlashCommandAsPlainTextWhenNativeCommandsDisabled(t *testing.T) {
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

	channel, err := NewChannel(log, config.WeWorkConfig{
		Enabled:    true,
		CorpID:     "corp",
		AgentID:    "agent",
		CorpSecret: "secret",
	}, messageBus, commandRegistry)
	if err != nil {
		t.Fatalf("NewChannel failed: %v", err)
	}

	channel.processMessage("user-1", "/help topic", "msg-1")

	if commandCalls != 0 {
		t.Fatalf("expected native command handler to stay disabled, got %d calls", commandCalls)
	}
	if len(messageBus.inbound) != 1 {
		t.Fatalf("expected one inbound message, got %d", len(messageBus.inbound))
	}
	msg := messageBus.inbound[0]
	if msg.ChannelID != "wework" {
		t.Fatalf("expected channel wework, got %q", msg.ChannelID)
	}
	if msg.SessionID != "wework:user-1" {
		t.Fatalf("expected session wework:user-1, got %q", msg.SessionID)
	}
	if msg.Content != "/help topic" {
		t.Fatalf("expected original slash command content, got %q", msg.Content)
	}
}

func TestAccountScopedProcessMessageStillTreatsSlashCommandAsPlainTextWhenNativeCommandsDisabled(t *testing.T) {
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

	channel, err := NewAccountChannel(log, config.WeWorkConfig{
		Enabled:    true,
		CorpID:     "corp",
		AgentID:    "agent",
		CorpSecret: "secret",
	}, messageBus, commandRegistry, "wework:corp-a", "WeWork Corp A")
	if err != nil {
		t.Fatalf("NewAccountChannel failed: %v", err)
	}

	channel.processMessage("user-1", "/help topic", "msg-1")

	if commandCalls != 0 {
		t.Fatalf("expected native command handler to stay disabled for account-scoped channel, got %d calls", commandCalls)
	}
	if len(messageBus.inbound) != 1 {
		t.Fatalf("expected one inbound message, got %d", len(messageBus.inbound))
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	log := newTestLogger(t)
	channel, err := NewChannel(log, config.WeWorkConfig{
		Enabled:    true,
		CorpID:     "corp",
		AgentID:    "agent",
		CorpSecret: "secret",
	}, &stubBus{}, commands.NewRegistry())
	if err != nil {
		t.Fatalf("NewChannel failed: %v", err)
	}

	channel.accessToken = "token"
	channel.tokenExpiresAt = time.Now().Add(time.Hour).Unix()
	channel.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.String(), "message/send?access_token=token") {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			payload := string(body)
			if !strings.Contains(payload, "Tool call: read_file") {
				t.Fatalf("expected tool trace in wework payload, got %q", payload)
			}
			if !strings.Contains(payload, "\\n\\ndone") {
				t.Fatalf("expected original reply after blank line, got %q", payload)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	err = channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "wework:user-1",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
