package maixcam

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/bus"
	channelcapabilities "nekobot/pkg/channelcapabilities"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestSendMessageTargetsSessionDevice(t *testing.T) {
	ch := newTestChannel(t)
	targetConn := newStubConn("device-target")
	otherConn := newStubConn("device-other")

	ch.clients[targetConn] = true
	ch.clients[otherConn] = true

	err := ch.SendMessage(context.Background(), &bus.Message{
		ID:        "out-1",
		ChannelID: "maixcam",
		SessionID: "maixcam:device-target",
		Content:   "only target",
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	targetMsg := decodeStubPayload(t, targetConn)
	if got := targetMsg["content"]; got != "only target" {
		t.Fatalf("expected target content, got %#v", got)
	}
	if otherConn.buffer.Len() != 0 {
		t.Fatalf("expected no message for non-target device, got %q", otherConn.buffer.String())
	}
}

func TestSendMessageBroadcastsWithoutTargetSession(t *testing.T) {
	ch := newTestChannel(t)
	firstConn := newStubConn("device-1")
	secondConn := newStubConn("device-2")

	ch.clients[firstConn] = true
	ch.clients[secondConn] = true

	err := ch.SendMessage(context.Background(), &bus.Message{
		ID:        "out-2",
		ChannelID: "maixcam",
		Content:   "broadcast",
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	firstMsg := decodeStubPayload(t, firstConn)
	secondMsg := decodeStubPayload(t, secondConn)
	if got := firstMsg["content"]; got != "broadcast" {
		t.Fatalf("expected broadcast content on first device, got %#v", got)
	}
	if got := secondMsg["content"]; got != "broadcast" {
		t.Fatalf("expected broadcast content on second device, got %#v", got)
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	ch := newTestChannel(t)
	targetConn := newStubConn("device-target")
	ch.clients[targetConn] = true

	err := ch.SendMessage(context.Background(), &bus.Message{
		ID:        "out-3",
		ChannelID: "maixcam",
		SessionID: "maixcam:device-target",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	targetMsg := decodeStubPayload(t, targetConn)
	content, _ := targetMsg["content"].(string)
	if !strings.Contains(content, "Tool call: read_file") {
		t.Fatalf("expected tool trace in maixcam content, got %#v", targetMsg["content"])
	}
	if !strings.Contains(content, "\n\ndone") {
		t.Fatalf("expected original reply after blank line, got %#v", targetMsg["content"])
	}
}

func TestMaixcamDeviceIDFromSession(t *testing.T) {
	if got := maixcamDeviceIDFromSession("maixcam:device-1"); got != "device-1" {
		t.Fatalf("expected parsed device id, got %q", got)
	}
	if got := maixcamDeviceIDFromSession("telegram:123"); got != "" {
		t.Fatalf("expected empty device id for foreign session, got %q", got)
	}
}

func newTestChannel(t *testing.T) *Channel {
	t.Helper()

	log := newTestLogger(t)
	ch, err := NewChannel(log, config.MaixCamConfig{
		Enabled: true,
		Host:    "127.0.0.1",
		Port:    0,
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewChannel failed: %v", err)
	}
	return ch
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

type stubConn struct {
	buffer bytes.Buffer
	remote stubAddr
}

func newStubConn(remote string) *stubConn {
	return &stubConn{remote: stubAddr(remote)}
}

func (c *stubConn) Read(_ []byte) (int, error)       { return 0, io.EOF }
func (c *stubConn) Write(p []byte) (int, error)      { return c.buffer.Write(p) }
func (c *stubConn) Close() error                     { return nil }
func (c *stubConn) LocalAddr() net.Addr              { return stubAddr("local") }
func (c *stubConn) RemoteAddr() net.Addr             { return c.remote }
func (c *stubConn) SetDeadline(time.Time) error      { return nil }
func (c *stubConn) SetReadDeadline(time.Time) error  { return nil }
func (c *stubConn) SetWriteDeadline(time.Time) error { return nil }

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

func decodeStubPayload(t *testing.T, conn *stubConn) map[string]interface{} {
	t.Helper()

	var payload map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(conn.buffer.Bytes()), &payload); err != nil {
		t.Fatalf("failed to decode payload %q: %v", conn.buffer.String(), err)
	}
	return payload
}

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{}
	if !channel.supportsNativeCommands() {
		t.Fatal("expected maixcam native commands to be enabled by capability matrix")
	}
	if !channelcapabilities.IsCapabilityEnabled(
		channelcapabilities.GetDefaultCapabilitiesForChannel("maixcam"),
		channelcapabilities.CapabilityNativeCommands,
		channelcapabilities.CapabilityScopeDM,
		false,
	) {
		t.Fatal("expected direct capability check to enable maixcam native commands")
	}
}
