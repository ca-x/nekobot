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

// ---------------------------------------------------------------------------
// target_config contract tests
// ---------------------------------------------------------------------------

func TestParseTargetConfig_KnownFields(t *testing.T) {
	raw := `{"session_id":"tg:123","target":"#ops","chat_id":"chat-1","user_id":"u1","username":"bot","reply_to":"msg-1","context_token":"ctx-abc","title":"Alert"}`
	cfg, err := parseTargetConfig(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.SessionID != "tg:123" {
		t.Errorf("session_id = %q, want tg:123", cfg.SessionID)
	}
	if cfg.Target != "#ops" {
		t.Errorf("target = %q, want #ops", cfg.Target)
	}
	if cfg.ChatID != "chat-1" {
		t.Errorf("chat_id = %q, want chat-1", cfg.ChatID)
	}
	if cfg.UserID != "u1" {
		t.Errorf("user_id = %q, want u1", cfg.UserID)
	}
	if cfg.Username != "bot" {
		t.Errorf("username = %q, want bot", cfg.Username)
	}
	if cfg.ReplyTo != "msg-1" {
		t.Errorf("reply_to = %q, want msg-1", cfg.ReplyTo)
	}
	if cfg.ContextToken != "ctx-abc" {
		t.Errorf("context_token = %q, want ctx-abc", cfg.ContextToken)
	}
	if cfg.Title != "Alert" {
		t.Errorf("title = %q, want Alert", cfg.Title)
	}
}

func TestParseTargetConfig_ExtraFields(t *testing.T) {
	raw := `{"target":"#ops","priority":"high","channel_specific":{"thread_id":"t1"}}`
	cfg, err := parseTargetConfig(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Target != "#ops" {
		t.Errorf("target = %q, want #ops", cfg.Target)
	}
	if cfg.Extra["priority"] != "high" {
		t.Errorf("extra[priority] = %v, want high", cfg.Extra["priority"])
	}
	extraCS, ok := cfg.Extra["channel_specific"].(map[string]interface{})
	if !ok {
		t.Fatalf("extra[channel_specific] should be a map, got %T", cfg.Extra["channel_specific"])
	}
	if extraCS["thread_id"] != "t1" {
		t.Errorf("extra channel_specific.thread_id = %v, want t1", extraCS["thread_id"])
	}
}

func TestParseTargetConfig_EmptyAndMalformed(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"whitespace", "  \n\t  ", false},
		{"empty object", "{}", false},
		{"null string", "null", false},
		{"invalid json", "{broken", true},
		{"array instead of object", "[1,2,3]", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTargetConfig(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseTargetConfig(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestParseTargetConfig_CaseInsensitiveKeys(t *testing.T) {
	raw := `{"TARGET":"#ops","Session_ID":"tg:1","TITLE":"Alert"}`
	cfg, err := parseTargetConfig(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Target != "#ops" {
		t.Errorf("target = %q (case-insensitive), want #ops", cfg.Target)
	}
	if cfg.SessionID != "tg:1" {
		t.Errorf("session_id = %q (case-insensitive), want tg:1", cfg.SessionID)
	}
	if cfg.Title != "Alert" {
		t.Errorf("title = %q (case-insensitive), want Alert", cfg.Title)
	}
}

func TestParseTargetConfig_NonStringValuesGoToExtra(t *testing.T) {
	raw := `{"target":"#ops","count":42,"enabled":true,"tags":["a","b"]}`
	cfg, err := parseTargetConfig(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Target != "#ops" {
		t.Errorf("target = %q, want #ops", cfg.Target)
	}
	if cfg.Extra["count"] != float64(42) {
		t.Errorf("extra[count] = %v, want 42", cfg.Extra["count"])
	}
	if cfg.Extra["enabled"] != true {
		t.Errorf("extra[enabled] = %v, want true", cfg.Extra["enabled"])
	}
}

// ---------------------------------------------------------------------------
// Event matching tests
// ---------------------------------------------------------------------------

func TestBindingMatchesEvent(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		eventType string
		want      bool
	}{
		{"exact match", `["cron.succeeded"]`, "cron.succeeded", true},
		{"no match", `["cron.succeeded"]`, "cron.failed", false},
		{"wildcard", `["*"]`, "cron.succeeded", true},
		{"wildcard any", `["*"]`, "anything", true},
		{"multiple events", `["cron.succeeded","cron.failed"]`, "cron.failed", true},
		{"empty events", `[]`, "cron.succeeded", false},
		{"empty event type", `["cron.succeeded"]`, "", false},
		{"invalid json", `broken`, "cron.succeeded", false},
		{"whitespace in events", `[" cron.succeeded "]`, "cron.succeeded", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bindingMatchesEvent(tc.raw, tc.eventType)
			if got != tc.want {
				t.Errorf("bindingMatchesEvent(%q, %q) = %v, want %v", tc.raw, tc.eventType, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Dispatcher delivery tests
// ---------------------------------------------------------------------------

func TestDispatcherDeliversCronFailedEvent(t *testing.T) {
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
		AccountKey:  "alert-bot",
		DisplayName: "Alert Bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "fail-alerts",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#alerts","title":"Failure Alert"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-fail",
		RouteID:        route.ID,
		EventTypesJSON: `["cron.failed"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType:  EventCronFailed,
		Job:        cron.Job{ID: "job-fail", Name: "backup"},
		Error:      "disk full",
		FinishedAt: time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
	})

	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected one outbound notification, got %d", len(messageBus.outbound))
	}
	msg := messageBus.outbound[0]
	if !strings.Contains(msg.Content, "failed") {
		t.Errorf("expected 'failed' in content, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "disk full") {
		t.Errorf("expected error message in content, got %q", msg.Content)
	}
	if title, _ := msg.Data["title"].(string); title != "Failure Alert" {
		t.Errorf("title = %q, want Failure Alert", title)
	}
}

func TestDispatcherSkipsDisabledRoute(t *testing.T) {
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
		AccountKey:  "bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "disabled-route",
		Enabled:          false, // disabled
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#test"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-disabled",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-disabled", Name: "test"},
	})

	if len(messageBus.outbound) != 0 {
		t.Fatalf("expected no outbound for disabled route, got %d", len(messageBus.outbound))
	}
}

func TestDispatcherSkipsDisabledBinding(t *testing.T) {
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
		AccountKey:  "bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "enabled-route",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#test"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-disabled-binding",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        false, // disabled
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-disabled-binding", Name: "test"},
	})

	if len(messageBus.outbound) != 0 {
		t.Fatalf("expected no outbound for disabled binding, got %d", len(messageBus.outbound))
	}
}

func TestDispatcherSkipsDisabledAccount(t *testing.T) {
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
		AccountKey:  "bot",
		Enabled:     false, // disabled
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "route-with-disabled-account",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#test"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-acct-disabled",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-acct-disabled", Name: "test"},
	})

	if len(messageBus.outbound) != 0 {
		t.Fatalf("expected no outbound for disabled account, got %d", len(messageBus.outbound))
	}
}

func TestDispatcherDefaultTitleForSuccessAndFailure(t *testing.T) {
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
		AccountKey:  "bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "no-title",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#ops"}`, // no title
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-title-test",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)

	// Success event → default title
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-title-test", Name: "test"},
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(messageBus.outbound))
	}
	if title, _ := messageBus.outbound[0].Data["title"].(string); title != "Nekobot schedule completed" {
		t.Errorf("success title = %q, want 'Nekobot schedule completed'", title)
	}

	// Failed event → default title
	messageBus.outbound = nil
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronFailed,
		Job:       cron.Job{ID: "job-title-test", Name: "test"},
		Error:     "timeout",
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(messageBus.outbound))
	}
	if title, _ := messageBus.outbound[0].Data["title"].(string); title != "Nekobot schedule failed" {
		t.Errorf("failure title = %q, want 'Nekobot schedule failed'", title)
	}
}

// ---------------------------------------------------------------------------
// DeleteAfterRun (one-shot binding cleanup) tests
// ---------------------------------------------------------------------------

func TestDispatcherDeleteAfterRun(t *testing.T) {
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
		AccountKey:  "one-shot-bot",
		DisplayName: "One Shot Bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "one-shot",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#ops"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-oneshot",
		RouteID:        route.ID,
		EventTypesJSON: `["cron.succeeded"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)

	// First event: DeleteAfterRun=true → should deliver AND delete binding
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType:      EventCronSucceeded,
		Job:            cron.Job{ID: "job-oneshot", Name: "cleanup test"},
		DeleteAfterRun: true,
		FinishedAt:     time.Date(2026, 4, 30, 11, 0, 0, 0, time.UTC),
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound on first event, got %d", len(messageBus.outbound))
	}

	// Second identical event: binding should be gone → no delivery
	messageBus.outbound = nil
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType:      EventCronSucceeded,
		Job:            cron.Job{ID: "job-oneshot", Name: "cleanup test"},
		DeleteAfterRun: true,
		FinishedAt:     time.Date(2026, 4, 30, 11, 1, 0, 0, time.UTC),
	})
	if len(messageBus.outbound) != 0 {
		t.Fatalf("expected no outbound after one-shot cleanup, got %d", len(messageBus.outbound))
	}
}

func TestDispatcherDeleteAfterRunFalseKeepsBinding(t *testing.T) {
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
		AccountKey:  "keep-bot",
		DisplayName: "Keep Bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "persistent",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#ops"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-persistent",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)

	// First event: DeleteAfterRun=false → binding persists
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-persistent", Name: "keep test"},
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(messageBus.outbound))
	}

	// Second identical event: binding still exists → delivers again
	messageBus.outbound = nil
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-persistent", Name: "keep test"},
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound on repeat (binding persists), got %d", len(messageBus.outbound))
	}
}

// ---------------------------------------------------------------------------
// notificationSessionID tests
// ---------------------------------------------------------------------------

func TestNotificationSessionID(t *testing.T) {
	tests := []struct {
		name        string
		channelType string
		target      targetConfig
		want        string
	}{
		{
			name:        "explicit session_id wins",
			channelType: "telegram",
			target:      targetConfig{SessionID: "custom:sess", Target: "#ops"},
			want:        "custom:sess",
		},
		{
			name:        "target with colon used as-is",
			channelType: "telegram",
			target:      targetConfig{Target: "tg:12345"},
			want:        "tg:12345",
		},
		{
			name:        "target without colon gets channel prefix",
			channelType: "telegram",
			target:      targetConfig{Target: "#ops"},
			want:        "telegram:#ops",
		},
		{
			name:        "chat_id fallback",
			channelType: "telegram",
			target:      targetConfig{ChatID: "chat-99"},
			want:        "telegram:chat-99",
		},
		{
			name:        "user_id fallback",
			channelType: "gotify",
			target:      targetConfig{UserID: "u1"},
			want:        "gotify:u1",
		},
		{
			name:        "empty everything uses channel:notification",
			channelType: "telegram",
			target:      targetConfig{},
			want:        "telegram:notification",
		},
		{
			name:        "empty channel type with target",
			channelType: "",
			target:      targetConfig{Target: "#ops"},
			want:        "#ops",
		},
		{
			name:        "empty channel type empty target",
			channelType: "",
			target:      targetConfig{},
			want:        ":notification",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := notificationSessionID(tc.channelType, tc.target)
			if got != tc.want {
				t.Errorf("notificationSessionID(%q, %+v) = %q, want %q", tc.channelType, tc.target, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cronNotificationContent tests
// ---------------------------------------------------------------------------

func TestCronNotificationContent(t *testing.T) {
	tests := []struct {
		name     string
		event    cron.JobEvent
		contains []string
	}{
		{
			name: "success with response",
			event: cron.JobEvent{
				EventType:  EventCronSucceeded,
				Job:        cron.Job{ID: "j1", Name: "daily report"},
				Response:   "all good",
				FinishedAt: time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC),
			},
			contains: []string{`"daily report"`, "succeeded", "j1", "all good"},
		},
		{
			name: "failure with error",
			event: cron.JobEvent{
				EventType:  EventCronFailed,
				Job:        cron.Job{ID: "j2", Name: "backup"},
				Error:      "disk full",
				FinishedAt: time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
			},
			contains: []string{`"backup"`, "failed", "disk full"},
		},
		{
			name: "success with empty response",
			event: cron.JobEvent{
				EventType:  EventCronSucceeded,
				Job:        cron.Job{ID: "j3", Name: "ping"},
				FinishedAt: time.Date(2026, 4, 30, 11, 0, 0, 0, time.UTC),
			},
			contains: []string{`"ping"`, "succeeded"},
		},
		{
			name: "zero finished_at omits timestamp line",
			event: cron.JobEvent{
				EventType: EventCronSucceeded,
				Job:       cron.Job{ID: "j4", Name: "test"},
			},
			contains: []string{"succeeded", "j4"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cronNotificationContent(tc.event)
			for _, s := range tc.contains {
				if !strings.Contains(got, s) {
					t.Errorf("content %q should contain %q", got, s)
				}
			}
		})
	}
}

func TestNotificationTitle(t *testing.T) {
	tests := []struct {
		name   string
		event  cron.JobEvent
		target targetConfig
		want   string
	}{
		{
			name:   "custom title wins",
			event:  cron.JobEvent{EventType: EventCronSucceeded},
			target: targetConfig{Title: "Custom"},
			want:   "Custom",
		},
		{
			name:   "success default",
			event:  cron.JobEvent{EventType: EventCronSucceeded},
			target: targetConfig{},
			want:   "Nekobot schedule completed",
		},
		{
			name:   "failed default",
			event:  cron.JobEvent{EventType: EventCronFailed, Error: "oops"},
			target: targetConfig{},
			want:   "Nekobot schedule failed",
		},
		{
			name:   "succeeded event with non-empty error treated as failure",
			event:  cron.JobEvent{EventType: EventCronSucceeded, Error: "partial"},
			target: targetConfig{},
			want:   "Nekobot schedule failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := notificationTitle(tc.event, tc.target)
			if got != tc.want {
				t.Errorf("notificationTitle(%+v, %+v) = %q, want %q", tc.event, tc.target, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Dispatcher activity logging tests
// ---------------------------------------------------------------------------

func TestDispatcherLogsActivityOnSuccess(t *testing.T) {
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
		AccountKey:  "activity-bot",
		DisplayName: "Activity Bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "activity-route",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#ops"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-activity",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	var activities []ActivityEntry
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus).
		WithActivityLogger(func(_ context.Context, entry ActivityEntry) {
			activities = append(activities, entry)
		})

	// Success event
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-activity", Name: "daily report"},
		Response:  "all good",
		FinishedAt: time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	})
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(activities))
	}
	if activities[0].Kind != "cron.succeeded" {
		t.Errorf("activity kind = %q, want cron.succeeded", activities[0].Kind)
	}
	if !strings.Contains(activities[0].Summary, "daily report") {
		t.Errorf("activity summary should contain job name, got %q", activities[0].Summary)
	}
	if !strings.Contains(activities[0].Detail, "all good") {
		t.Errorf("activity detail should contain response, got %q", activities[0].Detail)
	}

	// Failed event
	activities = nil
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronFailed,
		Job:       cron.Job{ID: "job-activity", Name: "daily report"},
		Error:     "disk full",
	})
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity entry for failure, got %d", len(activities))
	}
	if activities[0].Kind != "cron.failed" {
		t.Errorf("activity kind = %q, want cron.failed", activities[0].Kind)
	}
	if !strings.Contains(activities[0].Detail, "disk full") {
		t.Errorf("activity detail should contain error, got %q", activities[0].Detail)
	}
}

func TestDispatcherNoActivityWithoutLogger(t *testing.T) {
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
		AccountKey:  "no-activity-bot",
		Enabled:     true,
		Config:      map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("create channel account: %v", err)
	}
	route, err := routeMgr.CreateRoute(ctx, NotificationRoute{
		Name:             "no-activity",
		Enabled:          true,
		ChannelAccountID: account.ID,
		TargetConfigJSON: `{"target":"#ops"}`,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := routeMgr.CreateBinding(ctx, NotificationBinding{
		Scope:          ScopeCronJob,
		Target:         "job-no-activity",
		RouteID:        route.ID,
		EventTypesJSON: `["*"]`,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := &recordingBus{}
	// No activity logger set — should not panic
	dispatcher := NewDispatcher(log, routeMgr, accountMgr, messageBus)
	dispatcher.HandleCronJobEvent(ctx, cron.JobEvent{
		EventType: EventCronSucceeded,
		Job:       cron.Job{ID: "job-no-activity", Name: "test"},
	})
	if len(messageBus.outbound) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(messageBus.outbound))
	}
}
