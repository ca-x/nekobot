package infoflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "infoflow"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for infoflow group scope")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	log := newTestLogger(t)

	var gotPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewChannel(log, config.InfoflowConfig{
		Enabled:    true,
		WebhookURL: server.URL,
	}, &stubBus{}, commands.NewRegistry())
	if err != nil {
		t.Fatalf("new channel: %v", err)
	}

	err = ch.SendMessage(context.Background(), &bus.Message{
		SessionID: "infoflow:session-1",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	content, _ := gotPayload["content"].(string)
	if !strings.Contains(content, "Tool call: read_file") {
		t.Fatalf("expected tool trace in infoflow content, got %#v", gotPayload["content"])
	}
	if !strings.Contains(content, "\n\ndone") {
		t.Fatalf("expected original reply after blank line, got %#v", gotPayload["content"])
	}
}

type stubBus struct{}

func (b *stubBus) Start() error                                                  { return nil }
func (b *stubBus) Stop() error                                                   { return nil }
func (b *stubBus) RegisterInboundHandler(channelID string, handler bus.Handler)  {}
func (b *stubBus) UnregisterInboundHandlers(channelID string)                    {}
func (b *stubBus) RegisterOutboundHandler(channelID string, handler bus.Handler) {}
func (b *stubBus) UnregisterOutboundHandlers(channelID string)                   {}
func (b *stubBus) RegisterHandler(channelID string, handler bus.Handler)         {}
func (b *stubBus) UnregisterHandlers(channelID string)                           {}
func (b *stubBus) SendInbound(msg *bus.Message) error                            { return nil }
func (b *stubBus) SendOutbound(msg *bus.Message) error                           { return nil }
func (b *stubBus) GetMetrics() map[string]uint64                                 { return map[string]uint64{} }

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
