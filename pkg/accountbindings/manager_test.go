package accountbindings

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/runtimeagents"
)

func TestManagerCRUDAndModeRules(t *testing.T) {
	ctx := context.Background()
	mgr, runtimes, accounts := newTestManager(t)

	runtimeA, err := runtimes.Create(ctx, runtimeagents.AgentRuntime{Name: "agent-a"})
	if err != nil {
		t.Fatalf("create runtime A: %v", err)
	}
	runtimeB, err := runtimes.Create(ctx, runtimeagents.AgentRuntime{Name: "agent-b"})
	if err != nil {
		t.Fatalf("create runtime B: %v", err)
	}
	account, err := accounts.Create(ctx, channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-a",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	created, err := mgr.Create(ctx, AccountBinding{
		ChannelAccountID: account.ID,
		AgentRuntimeID:   runtimeA.ID,
		BindingMode:      ModeSingleAgent,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if _, err := mgr.Create(ctx, AccountBinding{
		ChannelAccountID: account.ID,
		AgentRuntimeID:   runtimeA.ID,
		BindingMode:      ModeSingleAgent,
		Enabled:          true,
	}); !errors.Is(err, ErrBindingExists) {
		t.Fatalf("expected ErrBindingExists, got %v", err)
	}

	if _, err := mgr.Create(ctx, AccountBinding{
		ChannelAccountID: account.ID,
		AgentRuntimeID:   runtimeB.ID,
		BindingMode:      ModeSingleAgent,
		Enabled:          true,
	}); !errors.Is(err, ErrSingleAgentBindingExceeded) {
		t.Fatalf("expected ErrSingleAgentBindingExceeded, got %v", err)
	}

	updated, err := mgr.Update(ctx, created.ID, AccountBinding{
		ChannelAccountID: account.ID,
		AgentRuntimeID:   runtimeA.ID,
		BindingMode:      ModeMultiAgent,
		Enabled:          true,
		AllowPublicReply: true,
	})
	if err != nil {
		t.Fatalf("update binding: %v", err)
	}
	if updated.BindingMode != ModeMultiAgent {
		t.Fatalf("expected multi-agent mode, got %+v", updated)
	}

	second, err := mgr.Create(ctx, AccountBinding{
		ChannelAccountID: account.ID,
		AgentRuntimeID:   runtimeB.ID,
		BindingMode:      ModeMultiAgent,
		Enabled:          true,
		AllowPublicReply: false,
	})
	if err != nil {
		t.Fatalf("create second binding: %v", err)
	}
	if second.ID == "" {
		t.Fatalf("expected second binding id")
	}

	listed, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(listed))
	}

	if err := mgr.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	if err := mgr.Delete(ctx, created.ID); !errors.Is(err, ErrBindingNotFound) {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
}

func newTestManager(t *testing.T) (*Manager, *runtimeagents.Manager, *channelaccounts.Manager) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
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

	runtimes, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	accounts, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new channel account manager: %v", err)
	}
	mgr, err := NewManager(cfg, log, client, runtimes, accounts)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}
	return mgr, runtimes, accounts
}
