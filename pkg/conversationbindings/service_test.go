package conversationbindings

import (
	"context"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/toolsessions"
)

func TestServiceBindResolveAndClear(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:           "user-1",
		Source:          toolsessions.SourceChannel,
		Channel:         "wechat",
		ConversationKey: "wx:user-1",
		Tool:            "codex",
		Title:           "Code Assistant",
		Command:         "codex",
		Workdir:         cfg.WorkspacePath(),
		State:           toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.Bind(ctx, "user-1", sess.ID); err != nil {
		t.Fatalf("bind conversation: %v", err)
	}

	resolved, err := svc.Resolve(ctx, "user-1")
	if err != nil {
		t.Fatalf("resolve conversation: %v", err)
	}
	if resolved == nil || resolved.ID != sess.ID {
		t.Fatalf("expected resolved session %q, got %+v", sess.ID, resolved)
	}

	if got := svc.ConversationID(resolved.ConversationKey); got != "user-1" {
		t.Fatalf("expected conversation id user-1, got %q", got)
	}

	if err := svc.Clear(ctx, "user-1"); err != nil {
		t.Fatalf("clear conversation: %v", err)
	}

	resolved, err = svc.Resolve(ctx, "user-1")
	if err != nil {
		t.Fatalf("resolve conversation after clear: %v", err)
	}
	if resolved != nil {
		t.Fatalf("expected nil resolved session after clear, got %+v", resolved)
	}
}

func TestServiceListFiltersChannelAndPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	for _, input := range []toolsessions.CreateSessionInput{
		{
			Owner:           "user-1",
			Source:          toolsessions.SourceChannel,
			Channel:         "wechat",
			ConversationKey: "wx:user-1",
			Tool:            "codex",
			Command:         "codex",
			Workdir:         cfg.WorkspacePath(),
			State:           toolsessions.StateRunning,
		},
		{
			Owner:           "user-1",
			Source:          toolsessions.SourceChannel,
			Channel:         "slack",
			ConversationKey: "slack:user-2",
			Tool:            "claude",
			Command:         "claude",
			Workdir:         cfg.WorkspacePath(),
			State:           toolsessions.StateRunning,
		},
		{
			Owner:           "user-1",
			Source:          toolsessions.SourceChannel,
			Channel:         "wechat",
			ConversationKey: "user-3",
			Tool:            "aider",
			Command:         "aider",
			Workdir:         cfg.WorkspacePath(),
			State:           toolsessions.StateRunning,
		},
	} {
		if _, err := mgr.CreateSession(ctx, input); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 matching wechat binding session, got %d", len(got))
	}
	if got[0].ConversationKey != "wx:user-1" {
		t.Fatalf("expected wx:user-1, got %q", got[0].ConversationKey)
	}
}

func TestServiceBindWithOptionsAndListBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Title:   "Code Assistant",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	expiresAt := time.Now().Add(5 * time.Minute).Round(time.Second)
	if err := svc.BindWithOptions(ctx, "user-1", sess.ID, BindOptions{
		TargetKind: "session",
		Placement:  "child",
		Label:      "wechat-runtime",
		BoundBy:    "user",
		ExpiresAt:  &expiresAt,
		Details: map[string]interface{}{
			"driver": "codex",
		},
	}); err != nil {
		t.Fatalf("bind with options: %v", err)
	}

	record, err := svc.GetBinding(ctx, "user-1")
	if err != nil {
		t.Fatalf("get binding: %v", err)
	}
	if record == nil {
		t.Fatal("expected binding record, got nil")
	}
	if record.TargetSessionID != sess.ID {
		t.Fatalf("expected target session %q, got %q", sess.ID, record.TargetSessionID)
	}
	if record.TargetKind != "session" {
		t.Fatalf("expected target kind session, got %q", record.TargetKind)
	}
	if record.Placement != "child" {
		t.Fatalf("expected placement child, got %q", record.Placement)
	}
	if record.Metadata.Label != "wechat-runtime" || record.Metadata.BoundBy != "user" {
		t.Fatalf("unexpected metadata: %+v", record.Metadata)
	}
	if record.ExpiresAt == nil || !record.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expires_at %v, got %+v", expiresAt, record.ExpiresAt)
	}
	if got := record.Conversation.ConversationID; got != "user-1" {
		t.Fatalf("expected conversation id user-1, got %q", got)
	}

	records, err := svc.ListBindings(ctx)
	if err != nil {
		t.Fatalf("list binding records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 binding record, got %d", len(records))
	}

	bySession, err := svc.GetBindingsBySession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get bindings by session: %v", err)
	}
	if len(bySession) != 1 || bySession[0].TargetSessionID != sess.ID {
		t.Fatalf("unexpected session binding records: %+v", bySession)
	}
}

func TestServiceCleanupExpiredBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	expiredSession, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	activeSession, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "claude",
		Command: "claude",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create active session: %v", err)
	}

	expiredAt := time.Now().Add(-1 * time.Minute)
	activeUntil := time.Now().Add(10 * time.Minute)
	if err := svc.BindWithOptions(ctx, "expired", expiredSession.ID, BindOptions{ExpiresAt: &expiredAt}); err != nil {
		t.Fatalf("bind expired: %v", err)
	}
	if err := svc.BindWithOptions(ctx, "active", activeSession.ID, BindOptions{ExpiresAt: &activeUntil}); err != nil {
		t.Fatalf("bind active: %v", err)
	}

	cleaned, err := svc.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("cleanup expired: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned binding, got %d", cleaned)
	}

	resolvedExpired, err := svc.Resolve(ctx, "expired")
	if err != nil {
		t.Fatalf("resolve expired after cleanup: %v", err)
	}
	if resolvedExpired != nil {
		t.Fatalf("expected expired binding cleared, got %+v", resolvedExpired)
	}

	resolvedActive, err := svc.Resolve(ctx, "active")
	if err != nil {
		t.Fatalf("resolve active after cleanup: %v", err)
	}
	if resolvedActive == nil || resolvedActive.ID != activeSession.ID {
		t.Fatalf("expected active binding to remain, got %+v", resolvedActive)
	}
}

func TestServiceSupportsMultipleBindingsPerSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Title:   "Code Assistant",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.BindWithOptions(ctx, "chat-a", sess.ID, BindOptions{
		Label: "primary",
	}); err != nil {
		t.Fatalf("bind chat-a: %v", err)
	}
	if err := svc.BindWithOptions(ctx, "chat-b", sess.ID, BindOptions{
		Label: "secondary",
	}); err != nil {
		t.Fatalf("bind chat-b: %v", err)
	}

	recordA, err := svc.GetBinding(ctx, "chat-a")
	if err != nil {
		t.Fatalf("get binding chat-a: %v", err)
	}
	if recordA == nil || recordA.TargetSessionID != sess.ID || recordA.Metadata.Label != "primary" {
		t.Fatalf("unexpected chat-a binding: %+v", recordA)
	}

	recordB, err := svc.GetBinding(ctx, "chat-b")
	if err != nil {
		t.Fatalf("get binding chat-b: %v", err)
	}
	if recordB == nil || recordB.TargetSessionID != sess.ID || recordB.Metadata.Label != "secondary" {
		t.Fatalf("unexpected chat-b binding: %+v", recordB)
	}

	records, err := svc.ListBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(records))
	}

	bySession, err := svc.GetBindingsBySession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get bindings by session: %v", err)
	}
	if len(bySession) != 2 {
		t.Fatalf("expected 2 bindings for session, got %d", len(bySession))
	}
}

func TestServiceClearPromotesRemainingBindingForSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.Bind(ctx, "chat-a", sess.ID); err != nil {
		t.Fatalf("bind chat-a: %v", err)
	}
	if err := svc.Bind(ctx, "chat-b", sess.ID); err != nil {
		t.Fatalf("bind chat-b: %v", err)
	}

	if err := svc.Clear(ctx, "chat-b"); err != nil {
		t.Fatalf("clear chat-b: %v", err)
	}

	resolvedA, err := svc.Resolve(ctx, "chat-a")
	if err != nil {
		t.Fatalf("resolve chat-a: %v", err)
	}
	if resolvedA == nil || resolvedA.ID != sess.ID {
		t.Fatalf("expected chat-a to remain bound, got %+v", resolvedA)
	}

	resolvedB, err := svc.Resolve(ctx, "chat-b")
	if err != nil {
		t.Fatalf("resolve chat-b: %v", err)
	}
	if resolvedB != nil {
		t.Fatalf("expected chat-b to be cleared, got %+v", resolvedB)
	}

	updated, err := mgr.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	if got := updated.ConversationKey; got != "wx:chat-a" {
		t.Fatalf("expected primary conversation key wx:chat-a, got %q", got)
	}
}

func TestServiceBindingQueriesReturnDeterministicConversationOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.Bind(ctx, "chat-b", sess.ID); err != nil {
		t.Fatalf("bind chat-b: %v", err)
	}
	if err := svc.Bind(ctx, "chat-a", sess.ID); err != nil {
		t.Fatalf("bind chat-a: %v", err)
	}

	records, err := svc.ListBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 binding records, got %d", len(records))
	}
	if records[0].Conversation.ConversationID != "chat-a" || records[1].Conversation.ConversationID != "chat-b" {
		t.Fatalf("expected ListBindings order [chat-a chat-b], got [%s %s]",
			records[0].Conversation.ConversationID,
			records[1].Conversation.ConversationID,
		)
	}

	bySession, err := svc.GetBindingsBySession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get bindings by session: %v", err)
	}
	if len(bySession) != 2 {
		t.Fatalf("expected 2 session binding records, got %d", len(bySession))
	}
	if bySession[0].Conversation.ConversationID != "chat-a" || bySession[1].Conversation.ConversationID != "chat-b" {
		t.Fatalf("expected GetBindingsBySession order [chat-a chat-b], got [%s %s]",
			bySession[0].Conversation.ConversationID,
			bySession[1].Conversation.ConversationID,
		)
	}
}

func TestServiceRebindingPromotesDeterministicPrimaryConversation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	primarySess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create primary session: %v", err)
	}
	secondarySess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "claude",
		Command: "claude",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create secondary session: %v", err)
	}

	if err := svc.Bind(ctx, "chat-c", primarySess.ID); err != nil {
		t.Fatalf("bind chat-c: %v", err)
	}
	if err := svc.Bind(ctx, "chat-b", primarySess.ID); err != nil {
		t.Fatalf("bind chat-b: %v", err)
	}
	if err := svc.Bind(ctx, "chat-a", primarySess.ID); err != nil {
		t.Fatalf("bind chat-a: %v", err)
	}

	if err := svc.Bind(ctx, "chat-c", secondarySess.ID); err != nil {
		t.Fatalf("rebind chat-c: %v", err)
	}

	updatedPrimary, err := mgr.GetSession(ctx, primarySess.ID)
	if err != nil {
		t.Fatalf("get updated primary session: %v", err)
	}
	if got := updatedPrimary.ConversationKey; got != "wx:chat-a" {
		t.Fatalf("expected deterministic primary conversation key wx:chat-a, got %q", got)
	}

	updatedSecondary, err := mgr.GetSession(ctx, secondarySess.ID)
	if err != nil {
		t.Fatalf("get updated secondary session: %v", err)
	}
	if got := updatedSecondary.ConversationKey; got != "wx:chat-c" {
		t.Fatalf("expected rebound session conversation key wx:chat-c, got %q", got)
	}
}

func TestServiceRebindPreservesExistingPrimaryConversationOnTargetSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	sourceSession, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create source session: %v", err)
	}

	targetSession, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "claude",
		Command: "claude",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create target session: %v", err)
	}

	if err := svc.Bind(ctx, "chat-source", sourceSession.ID); err != nil {
		t.Fatalf("bind source conversation: %v", err)
	}
	if err := svc.Bind(ctx, "chat-primary", targetSession.ID); err != nil {
		t.Fatalf("bind target primary conversation: %v", err)
	}

	if err := svc.Bind(ctx, "chat-source", targetSession.ID); err != nil {
		t.Fatalf("rebind source conversation to target session: %v", err)
	}

	resolvedSource, err := svc.Resolve(ctx, "chat-source")
	if err != nil {
		t.Fatalf("resolve rebound conversation: %v", err)
	}
	if resolvedSource == nil || resolvedSource.ID != targetSession.ID {
		t.Fatalf("expected chat-source bound to target session %q, got %+v", targetSession.ID, resolvedSource)
	}

	clearedSource, err := mgr.GetSession(ctx, sourceSession.ID)
	if err != nil {
		t.Fatalf("get cleared source session: %v", err)
	}
	if got := clearedSource.ConversationKey; got != "" {
		t.Fatalf("expected source session conversation key cleared, got %q", got)
	}

	updatedTarget, err := mgr.GetSession(ctx, targetSession.ID)
	if err != nil {
		t.Fatalf("get updated target session: %v", err)
	}
	if got := updatedTarget.ConversationKey; got != "wx:chat-primary" {
		t.Fatalf("expected target primary conversation key to remain wx:chat-primary, got %q", got)
	}

	records, err := svc.GetBindingsBySession(ctx, targetSession.ID)
	if err != nil {
		t.Fatalf("get target session bindings: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 bindings on target session, got %d", len(records))
	}
	if records[0].Conversation.ConversationID != "chat-primary" || records[1].Conversation.ConversationID != "chat-source" {
		t.Fatalf("unexpected target binding order after rebind: %+v", records)
	}
}

func TestServiceResolveFindsOlderBindingBeyondSessionListCap(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := New(mgr, toolsessions.SourceChannel, "wechat", "wx:")
	ctx := context.Background()

	oldestSession, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "user-1",
		Source:  toolsessions.SourceChannel,
		Channel: "wechat",
		Tool:    "codex",
		Command: "codex",
		Workdir: cfg.WorkspacePath(),
		State:   toolsessions.StateRunning,
	})
	if err != nil {
		t.Fatalf("create oldest bound session: %v", err)
	}

	if err := svc.Bind(ctx, "chat-older", oldestSession.ID); err != nil {
		t.Fatalf("bind oldest conversation: %v", err)
	}

	for i := 0; i < 200; i++ {
		_, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
			Owner:   "user-1",
			Source:  toolsessions.SourceChannel,
			Channel: "wechat",
			Tool:    "codex",
			Command: "codex",
			Workdir: cfg.WorkspacePath(),
			State:   toolsessions.StateRunning,
		})
		if err != nil {
			t.Fatalf("create filler session %d: %v", i, err)
		}
	}

	resolved, err := svc.Resolve(ctx, "chat-older")
	if err != nil {
		t.Fatalf("resolve older conversation: %v", err)
	}
	if resolved == nil || resolved.ID != oldestSession.ID {
		t.Fatalf("expected oldest session %q, got %+v", oldestSession.ID, resolved)
	}
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

func newTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return client
}
