package conversationbindings

import (
	"context"
	"testing"

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
	defer client.Close()

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
