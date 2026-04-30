package notificationroutes

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/cron"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func TestDispatcherDeliversCronNotificationToRouteTarget(t *testing.T) {
	ctx := context.Background()
	cfg, log, client := newDispatcherTestRuntime(t)
	routeMgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new channel account manager: %v", err)
	}
	account, err := accountMgr.Create(ctx, channelaccounts.ChannelAccount{
		ChannelType: "telegram",
		AccountKey:  "ops-bot",
		DisplayName: "Ops Bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "ops",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"12345","title":"Ops schedule"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-1",
		RouteID:        route.ID,
		EventTypesJSON: `["cron.succeeded"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType:  EventCronSucceeded,
		Job:        cron.Job{ID: "job-1", Name: "daily report"},
		Response:   "all good",
		FinishedAt: time.Date(2026, 4, 30, 9, 30, 0, 0, time.UTC),
	})

	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected one outbound notification, got %d", len(messageBus.outbound))
	}
	msg := messageBus.outbound[0]
	if msg.ChannelID != "telegram:ops-bot" {
		t.Fatalf("expected runtime channel id telegram:ops-bot, got %q", msg.ChannelID)
	}
	if msg.SessionID != "telegram:12345" {
		t.Fatalf("expected telegram session target, got %q", msg.SessionID)
	}
	if title, _ := msg.Data["title"].(string); title != "Ops schedule" {
		t.Fatalf("expected route title in data, got %q", title)
	}
	if !strings.Contains(msg.Content, "daily report") || !strings.Contains(msg.Content, "all good") {
		t.Fatalf("expected job name and response in notification content, got %q", msg.Content)
	}
}

func TestDispatcherSkipsUnmatchedCronEvent(t *testing.T) {
	ctx := context.Background()
	cfg, log, client := newDispatcherTestRuntime(t)
	routeMgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new route manager: %v", err)
	}
	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new channel account manager: %v", err)
	}
	account, err := accountMgr.Create(ctx, channelaccounts.ChannelAccount{
		ChannelType: "gotify",
		AccountKey:  "default",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "success-only",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-2",
		RouteID:        route.ID,
		EventTypesJSON: `["cron.succeeded"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronFailed,
		Job:       cron.Job{ID: "job-2", Name: "daily report"},
		Error:     "provider timeout",
	})

	if len(messageBus.outbound) != 0 {
		t.Fatalf("expected no outbound notification for unmatched event, got %+v", messageBus.outbound)
	}
}

func newDispatcherTestRuntime(t *testing.T) (*config.Config, *logger.Logger, *ent.Client) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return cfg, log, client
}

type recordingBus struct {
	outbound []*bus.Message
}

func (b *recordingBus) Start() error                                { return nil }
func (b *recordingBus) Stop() error                                 { return nil }
func (b *recordingBus) RegisterInboundHandler(string, bus.Handler)  {}
func (b *recordingBus) UnregisterInboundHandlers(string)            {}
func (b *recordingBus) RegisterOutboundHandler(string, bus.Handler) {}
func (b *recordingBus) UnregisterOutboundHandlers(string)           {}
func (b *recordingBus) RegisterHandler(string, bus.Handler)         {}
func (b *recordingBus) UnregisterHandlers(string)                   {}
func (b *recordingBus) SendInbound(*bus.Message) error              { return nil }
func (b *recordingBus) SendOutbound(msg *bus.Message) error {
	b.outbound = append(b.outbound, msg)
	return nil
}
func (b *recordingBus) GetMetrics() map[string]uint64 { return map[string]uint64{} }
