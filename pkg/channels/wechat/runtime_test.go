package wechat

import (
	"context"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/toolsessions"
)

func TestRuntimeBindingServiceBindResolveAndClear(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	svc := NewRuntimeBindingService(mgr, cfg)
	ctx := context.Background()

	sess, err := mgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:           "wechat-user",
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

	if err := svc.BindConversation(ctx, "user-1", sess.ID); err != nil {
		t.Fatalf("bind conversation: %v", err)
	}

	resolved, err := svc.ResolveConversation(ctx, "user-1")
	if err != nil {
		t.Fatalf("resolve conversation: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved session, got nil")
	}
	if resolved.ID != sess.ID {
		t.Fatalf("expected session %q, got %q", sess.ID, resolved.ID)
	}

	bindings, err := svc.ListBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].ConversationKey != "wx:user-1" {
		t.Fatalf("expected conversation key wx:user-1, got %q", bindings[0].ConversationKey)
	}

	if err := svc.ClearConversation(ctx, "user-1"); err != nil {
		t.Fatalf("clear conversation: %v", err)
	}

	resolved, err = svc.ResolveConversation(ctx, "user-1")
	if err != nil {
		t.Fatalf("resolve conversation after clear: %v", err)
	}
	if resolved != nil {
		t.Fatalf("expected nil resolved session after clear, got %+v", resolved)
	}
}

func newRuntimeTestLogger(t *testing.T) *logger.Logger {
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

func newRuntimeTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
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

func TestBuildRuntimePresetSupportsACPAndCodex(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = "/workspace/project"

	tests := []struct {
		name       string
		driver     string
		tool       string
		command    string
		workdir    string
		wantTool   string
		wantCmd    string
		wantDriver string
		wantWD     string
	}{
		{
			name:       "acp codex preset",
			driver:     "acp",
			tool:       "codex",
			command:    "",
			workdir:    "",
			wantTool:   "codex",
			wantCmd:    "codex-acp",
			wantDriver: "acp",
			wantWD:     "/workspace/project",
		},
		{
			name:       "acp claude preset",
			driver:     "acp",
			tool:       "claude",
			command:    "",
			workdir:    "",
			wantTool:   "claude",
			wantCmd:    "claude-agent-acp",
			wantDriver: "acp",
			wantWD:     "/workspace/project",
		},
		{
			name:       "codex native preset",
			driver:     "codex",
			tool:       "code2",
			command:    "",
			workdir:    "/tmp/run",
			wantTool:   "codex",
			wantCmd:    "codex",
			wantDriver: "codex",
			wantWD:     "/tmp/run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preset, err := BuildRuntimePreset(cfg, RuntimeSpec{
				Driver:  tt.driver,
				Tool:    tt.tool,
				Command: tt.command,
				Workdir: tt.workdir,
			})
			if err != nil {
				t.Fatalf("BuildRuntimePreset failed: %v", err)
			}
			if preset.Tool != tt.wantTool {
				t.Fatalf("expected tool %q, got %q", tt.wantTool, preset.Tool)
			}
			if preset.Command != tt.wantCmd {
				t.Fatalf("expected command %q, got %q", tt.wantCmd, preset.Command)
			}
			if preset.Driver != tt.wantDriver {
				t.Fatalf("expected driver %q, got %q", tt.wantDriver, preset.Driver)
			}
			if preset.Workdir != tt.wantWD {
				t.Fatalf("expected workdir %q, got %q", tt.wantWD, preset.Workdir)
			}
		})
	}
}
