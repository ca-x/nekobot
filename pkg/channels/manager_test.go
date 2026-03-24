package channels

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"nekobot/pkg/bus"
	"nekobot/pkg/logger"
)

type testChannel struct {
	id        string
	name      string
	enabled   bool
	started   atomic.Int32
	stopped   atomic.Int32
	sendCount atomic.Int32
	sendCh    chan struct{}
}

func (c *testChannel) ID() string                      { return c.id }
func (c *testChannel) Name() string                    { return c.name }
func (c *testChannel) IsEnabled() bool                 { return c.enabled }
func (c *testChannel) Start(ctx context.Context) error { c.started.Add(1); return nil }
func (c *testChannel) Stop(ctx context.Context) error  { c.stopped.Add(1); return nil }
func (c *testChannel) SendMessage(ctx context.Context, msg *bus.Message) error {
	c.sendCount.Add(1)
	select {
	case c.sendCh <- struct{}{}:
	default:
	}
	return nil
}

func TestManagerStartRoutesOutboundExactlyOnce(t *testing.T) {
	log := newTestChannelLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("Start bus failed: %v", err)
	}
	defer func() {
		if err := messageBus.Stop(); err != nil {
			t.Fatalf("Stop bus failed: %v", err)
		}
	}()

	manager := NewManager(log, messageBus)
	ch := &testChannel{
		id:      "test",
		name:    "Test",
		enabled: true,
		sendCh:  make(chan struct{}, 4),
	}
	if err := manager.Register(ch); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start manager failed: %v", err)
	}
	defer func() {
		if err := manager.Stop(); err != nil {
			t.Fatalf("Stop manager failed: %v", err)
		}
	}()

	if err := messageBus.SendOutbound(&bus.Message{
		ID:        "m1",
		ChannelID: "test",
		SessionID: "test:1",
		Content:   "hello",
	}); err != nil {
		t.Fatalf("SendOutbound failed: %v", err)
	}

	select {
	case <-ch.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for outbound dispatch")
	}

	time.Sleep(100 * time.Millisecond)
	if got := ch.sendCount.Load(); got != 1 {
		t.Fatalf("expected SendMessage once, got %d", got)
	}
}

func TestManagerReloadChannelReplacesOutboundHandler(t *testing.T) {
	log := newTestChannelLogger(t)
	messageBus := bus.NewLocalBus(log, 8)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("Start bus failed: %v", err)
	}
	defer func() {
		if err := messageBus.Stop(); err != nil {
			t.Fatalf("Stop bus failed: %v", err)
		}
	}()

	manager := NewManager(log, messageBus)
	original := &testChannel{
		id:      "test",
		name:    "Original",
		enabled: true,
		sendCh:  make(chan struct{}, 4),
	}
	if err := manager.Register(original); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start manager failed: %v", err)
	}
	defer func() {
		if err := manager.Stop(); err != nil {
			t.Fatalf("Stop manager failed: %v", err)
		}
	}()

	reloaded := &testChannel{
		id:      "test",
		name:    "Reloaded",
		enabled: true,
		sendCh:  make(chan struct{}, 4),
	}
	if err := manager.ReloadChannel(reloaded); err != nil {
		t.Fatalf("ReloadChannel failed: %v", err)
	}

	if err := messageBus.SendOutbound(&bus.Message{
		ID:        "m2",
		ChannelID: "test",
		SessionID: "test:2",
		Content:   "hello",
	}); err != nil {
		t.Fatalf("SendOutbound failed: %v", err)
	}

	select {
	case <-reloaded.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reloaded outbound dispatch")
	}

	time.Sleep(100 * time.Millisecond)
	if got := original.sendCount.Load(); got != 0 {
		t.Fatalf("expected original channel to stop receiving outbound messages, got %d", got)
	}
	if got := reloaded.sendCount.Load(); got != 1 {
		t.Fatalf("expected reloaded channel to receive one outbound message, got %d", got)
	}
}

func newTestChannelLogger(t *testing.T) *logger.Logger {
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
