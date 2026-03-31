package prompts

import (
	"context"
	"strings"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestResolvePrefersNarrowerScopeForSamePrompt(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	promptItem, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "ops",
		Name:     "Ops",
		Mode:     ModeSystem,
		Template: "scope={{channel.id}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeGlobal,
		PromptID: promptItem.ID,
		Enabled:  true,
		Priority: 100,
	}); err != nil {
		t.Fatalf("create global binding: %v", err)
	}
	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeChannel,
		Target:   "wechat",
		PromptID: promptItem.ID,
		Enabled:  true,
		Priority: 50,
	}); err != nil {
		t.Fatalf("create channel binding: %v", err)
	}

	resolved, err := mgr.Resolve(ctx, ResolveInput{Channel: "wechat"})
	if err != nil {
		t.Fatalf("resolve prompts: %v", err)
	}
	if len(resolved.Applied) != 1 {
		t.Fatalf("expected one applied prompt after dedupe, got %+v", resolved.Applied)
	}
	if resolved.Applied[0].Scope != ScopeChannel {
		t.Fatalf("expected narrower channel scope to win, got %+v", resolved.Applied[0])
	}
}

func TestResolveIgnoresDisabledBindingsAndPrompts(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	enabledPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "enabled",
		Name:     "Enabled",
		Mode:     ModeSystem,
		Template: "enabled",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create enabled prompt: %v", err)
	}
	disabledPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "disabled",
		Name:     "Disabled",
		Mode:     ModeSystem,
		Template: "disabled",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("create disabled prompt: %v", err)
	}

	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeGlobal,
		PromptID: enabledPrompt.ID,
		Enabled:  true,
		Priority: 100,
	}); err != nil {
		t.Fatalf("create enabled binding: %v", err)
	}
	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeGlobal,
		PromptID: disabledPrompt.ID,
		Enabled:  true,
		Priority: 110,
	}); err != nil {
		t.Fatalf("create disabled prompt binding: %v", err)
	}
	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeChannel,
		Target:   "wechat",
		PromptID: enabledPrompt.ID,
		Enabled:  false,
		Priority: 90,
	}); err != nil {
		t.Fatalf("create disabled binding: %v", err)
	}

	resolved, err := mgr.Resolve(ctx, ResolveInput{Channel: "wechat"})
	if err != nil {
		t.Fatalf("resolve prompts: %v", err)
	}
	if got := strings.TrimSpace(resolved.SystemText); got != "enabled" {
		t.Fatalf("expected only enabled prompt content, got %q", got)
	}
	if len(resolved.Applied) != 1 || resolved.Applied[0].PromptID != enabledPrompt.ID {
		t.Fatalf("unexpected applied prompts: %+v", resolved.Applied)
	}
}

func TestReplaceSessionBindingsSeparatesSystemAndUserPrompts(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	systemPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "system",
		Name:     "System",
		Mode:     ModeSystem,
		Template: "system",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create system prompt: %v", err)
	}
	userPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "user",
		Name:     "User",
		Mode:     ModeUser,
		Template: "user",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create user prompt: %v", err)
	}

	bindingSet, err := mgr.ReplaceSessionBindings(ctx, "session-1", []string{systemPrompt.ID}, []string{userPrompt.ID})
	if err != nil {
		t.Fatalf("replace session bindings: %v", err)
	}
	if len(bindingSet.SystemPromptIDs) != 1 || bindingSet.SystemPromptIDs[0] != systemPrompt.ID {
		t.Fatalf("unexpected system prompt ids: %+v", bindingSet.SystemPromptIDs)
	}
	if len(bindingSet.UserPromptIDs) != 1 || bindingSet.UserPromptIDs[0] != userPrompt.ID {
		t.Fatalf("unexpected user prompt ids: %+v", bindingSet.UserPromptIDs)
	}
	if len(bindingSet.Bindings) != 2 {
		t.Fatalf("expected two session bindings, got %+v", bindingSet.Bindings)
	}
}

func TestResolveRendersTemplateContextAndSeparatesModes(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	systemPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "system-context",
		Name:     "System Context",
		Mode:     ModeSystem,
		Template: "channel={{channel.id}} session={{session.id}} provider={{route.provider}} workspace={{workspace.path}} day={{now.date}} custom={{custom.role}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create system prompt: %v", err)
	}
	userPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "user-context",
		Name:     "User Context",
		Mode:     ModeUser,
		Template: "user={{user.name}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create user prompt: %v", err)
	}

	for _, binding := range []Binding{
		{Scope: ScopeGlobal, PromptID: systemPrompt.ID, Enabled: true, Priority: 100},
		{Scope: ScopeGlobal, PromptID: userPrompt.ID, Enabled: true, Priority: 110},
	} {
		if _, err := mgr.CreateBinding(ctx, binding); err != nil {
			t.Fatalf("create binding %+v: %v", binding, err)
		}
	}

	resolved, err := mgr.Resolve(ctx, ResolveInput{
		Channel:           "wechat",
		SessionID:         "session-1",
		UserID:            "u-1",
		Username:          "alice",
		RequestedProvider: "openai",
		Workspace:         "/workspace/demo",
		Custom:            map[string]any{"role": "ops"},
	})
	if err != nil {
		t.Fatalf("resolve prompts: %v", err)
	}

	if !strings.Contains(resolved.SystemText, "channel=wechat") ||
		!strings.Contains(resolved.SystemText, "session=session-1") ||
		!strings.Contains(resolved.SystemText, "provider=openai") ||
		!strings.Contains(resolved.SystemText, "workspace=/workspace/demo") ||
		!strings.Contains(resolved.SystemText, "custom=ops") {
		t.Fatalf("unexpected system prompt render: %q", resolved.SystemText)
	}
	if !strings.Contains(resolved.UserText, "user=alice") {
		t.Fatalf("unexpected user prompt render: %q", resolved.UserText)
	}
}

func TestResolveIncludesExplicitPromptIDs(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	globalPrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "global",
		Name:     "Global",
		Mode:     ModeSystem,
		Template: "global",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create global prompt: %v", err)
	}
	runtimePrompt, err := mgr.CreatePrompt(ctx, Prompt{
		Key:      "runtime",
		Name:     "Runtime",
		Mode:     ModeSystem,
		Template: "runtime={{custom.runtime_name}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create runtime prompt: %v", err)
	}

	if _, err := mgr.CreateBinding(ctx, Binding{
		Scope:    ScopeGlobal,
		PromptID: globalPrompt.ID,
		Enabled:  true,
		Priority: 100,
	}); err != nil {
		t.Fatalf("create global binding: %v", err)
	}

	resolved, err := mgr.Resolve(ctx, ResolveInput{
		Channel:           "wechat",
		RequestedProvider: "openai",
		Workspace:         "/workspace/demo",
		ExplicitPromptIDs: []string{runtimePrompt.ID},
		Custom: map[string]any{
			"runtime_name": "Support Main",
		},
	})
	if err != nil {
		t.Fatalf("resolve prompts: %v", err)
	}

	if !strings.Contains(resolved.SystemText, "global") {
		t.Fatalf("expected global prompt to remain applied, got %q", resolved.SystemText)
	}
	if !strings.Contains(resolved.SystemText, "runtime=Support Main") {
		t.Fatalf("expected explicit runtime prompt content, got %q", resolved.SystemText)
	}
	if len(resolved.Applied) != 2 {
		t.Fatalf("expected two applied prompts, got %+v", resolved.Applied)
	}
}

func newTestManager(t *testing.T) *Manager {
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

	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	return mgr
}
