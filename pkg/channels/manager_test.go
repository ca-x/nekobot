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
	id          string
	channelType string
	name        string
	enabled     bool
	started     atomic.Int32
	stopped     atomic.Int32
	sendCount   atomic.Int32
	sendCh      chan struct{}
}

func (c *testChannel) ID() string { return c.id }
func (c *testChannel) ChannelType() string {
	if c.channelType != "" {
		return c.channelType
	}
	return c.id
}
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
		id:          "test",
		channelType: "test",
		name:        "Test",
		enabled:     true,
		sendCh:      make(chan struct{}, 4),
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
		id:          "test",
		channelType: "test",
		name:        "Original",
		enabled:     true,
		sendCh:      make(chan struct{}, 4),
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
		id:          "test",
		channelType: "test",
		name:        "Reloaded",
		enabled:     true,
		sendCh:      make(chan struct{}, 4),
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

func TestManagerReloadChannelRestoresTypeAliasIndex(t *testing.T) {
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
		id:          "slack:team-a",
		channelType: "slack",
		name:        "Slack Team A",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}
	if err := manager.Register(original); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	reloaded := &testChannel{
		id:          "slack:team-a",
		channelType: "slack",
		name:        "Slack Team A Reloaded",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}
	if err := manager.ReloadChannel(reloaded); err != nil {
		t.Fatalf("ReloadChannel failed: %v", err)
	}

	got, err := manager.GetChannel("slack")
	if err != nil {
		t.Fatalf("GetChannel alias failed: %v", err)
	}
	if got != reloaded {
		t.Fatalf("expected alias to resolve reloaded instance")
	}

	instances := manager.ListChannelsByType("slack")
	if len(instances) != 1 {
		t.Fatalf("expected 1 slack instance after reload, got %d", len(instances))
	}
	if instances[0] != reloaded {
		t.Fatalf("expected type index to contain reloaded instance")
	}
}

func TestManagerStopChannelWithoutBusDoesNotPanic(t *testing.T) {
	log := newTestChannelLogger(t)
	manager := NewManager(log, nil)
	channel := &testChannel{
		id:          "wechat:bot-a",
		channelType: "wechat",
		name:        "WeChat Bot A",
		enabled:     true,
		sendCh:      make(chan struct{}, 1),
	}

	if err := manager.Register(channel); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := manager.StopChannel("wechat"); err != nil {
		t.Fatalf("StopChannel failed: %v", err)
	}

	if _, err := manager.GetChannel("wechat"); err == nil {
		t.Fatalf("expected channel to be removed after StopChannel")
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

func TestManagerSupportsMultipleInstancesPerChannelType(t *testing.T) {
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
	first := &testChannel{
		id:          "telegram:alpha",
		channelType: "telegram",
		name:        "Telegram Alpha",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}
	second := &testChannel{
		id:          "telegram:beta",
		channelType: "telegram",
		name:        "Telegram Beta",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}

	if err := manager.Register(first); err != nil {
		t.Fatalf("Register first failed: %v", err)
	}
	if err := manager.Register(second); err != nil {
		t.Fatalf("Register second failed: %v", err)
	}

	gotFirst, err := manager.GetChannel("telegram:alpha")
	if err != nil {
		t.Fatalf("GetChannel instance failed: %v", err)
	}
	if gotFirst != first {
		t.Fatalf("expected first instance by id")
	}

	gotDefault, err := manager.GetChannel("telegram")
	if err != nil {
		t.Fatalf("GetChannel default alias failed: %v", err)
	}
	if gotDefault != first {
		t.Fatalf("expected default alias to resolve first registered instance")
	}

	instances := manager.ListChannelsByType("telegram")
	if len(instances) != 2 {
		t.Fatalf("expected 2 telegram instances, got %d", len(instances))
	}
}

func TestManagerStopChannelKeepsTypeAliasForRemainingInstance(t *testing.T) {
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
	first := &testChannel{
		id:          "gotify:alpha",
		channelType: "gotify",
		name:        "Gotify Alpha",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}
	second := &testChannel{
		id:          "gotify:beta",
		channelType: "gotify",
		name:        "Gotify Beta",
		enabled:     true,
		sendCh:      make(chan struct{}, 2),
	}

	if err := manager.Register(first); err != nil {
		t.Fatalf("Register first failed: %v", err)
	}
	if err := manager.Register(second); err != nil {
		t.Fatalf("Register second failed: %v", err)
	}

	if err := manager.StopChannel("gotify:alpha"); err != nil {
		t.Fatalf("StopChannel failed: %v", err)
	}

	gotDefault, err := manager.GetChannel("gotify")
	if err != nil {
		t.Fatalf("GetChannel default alias failed: %v", err)
	}
	if gotDefault != second {
		t.Fatalf("expected default alias to move to remaining instance")
	}
}
