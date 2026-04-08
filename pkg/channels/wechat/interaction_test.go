package wechat

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
	wxtypes "nekobot/pkg/wechat/types"
)

func TestParseWeChatInteractionAction(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  interactionActionType
		wantValue string
		wantOK    bool
	}{
		{name: "confirm", input: "/yes", wantType: interactionActionConfirm, wantOK: true},
		{name: "deny", input: "/cancel", wantType: interactionActionDeny, wantOK: true},
		{name: "select", input: "/select 2", wantType: interactionActionSelect, wantValue: "2", wantOK: true},
		{name: "numeric confirm", input: "1", wantType: interactionActionSelect, wantValue: "1", wantOK: true},
		{name: "numeric deny", input: "2", wantType: interactionActionSelect, wantValue: "2", wantOK: true},
		{name: "numeric generic select", input: "3", wantType: interactionActionSelect, wantValue: "3", wantOK: true},
		{name: "passthrough", input: "/settings", wantType: interactionActionPassthrough, wantValue: "/settings", wantOK: true},
		{name: "plain text", input: "hello", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseWeChatInteractionAction(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if !tt.wantOK {
				return
			}
			if got == nil {
				t.Fatal("expected action, got nil")
			}
			if got.Type != tt.wantType {
				t.Fatalf("expected type %q, got %q", tt.wantType, got.Type)
			}
			if got.Value != tt.wantValue {
				t.Fatalf("expected value %q, got %q", tt.wantValue, got.Value)
			}
		})
	}
}

func TestParseWeChatInteractionActionRejectsEmptySelectValue(t *testing.T) {
	if action, ok := parseWeChatInteractionAction("/select   "); ok || action != nil {
		t.Fatalf("expected empty /select to be rejected, got ok=%v action=%#v", ok, action)
	}
}

func TestPendingSkillInstallExpires(t *testing.T) {
	ch := &Channel{
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now().Add(-16 * time.Minute),
	})

	if _, ok := ch.getPendingSkillInstall("user-1"); ok {
		t.Fatal("expected expired interaction to be evicted")
	}
}

func TestResolvePendingInteractionConfirm(t *testing.T) {
	registry := commands.NewRegistry()
	if err := registry.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if got := strings.TrimSpace(req.Args); got != "__confirm_install__ owner/repo" {
				t.Fatalf("unexpected args: %q", got)
			}
			if got := req.Metadata["skill_install_confirmed_repo"]; got != "owner/repo" {
				t.Fatalf("unexpected confirmed repo metadata: %q", got)
			}
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch := &Channel{
		commands:             registry,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/yes")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "installed" {
		t.Fatalf("expected reply %q, got %q", "installed", reply)
	}
	if _, ok := ch.getPendingSkillInstall("user-1"); ok {
		t.Fatal("expected pending interaction to be cleared after confirmation")
	}
}

func TestResolvePendingInteractionDeny(t *testing.T) {
	ch := &Channel{
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/no")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "已取消安装。" {
		t.Fatalf("unexpected deny reply: %q", reply)
	}
	if _, ok := ch.getPendingSkillInstall("user-1"); ok {
		t.Fatal("expected pending interaction to be cleared after denial")
	}
}

func TestResolvePendingInteractionSelectConfirmAlias(t *testing.T) {
	registry := commands.NewRegistry()
	if err := registry.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if got := strings.TrimSpace(req.Args); got != "__confirm_install__ owner/repo" {
				t.Fatalf("unexpected args: %q", got)
			}
			if got := req.Metadata["skill_install_confirmed_repo"]; got != "owner/repo" {
				t.Fatalf("unexpected confirmed repo metadata: %q", got)
			}
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch := &Channel{
		commands:             registry,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/select 1")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "installed" {
		t.Fatalf("expected reply %q, got %q", "installed", reply)
	}
	if _, ok := ch.getPendingSkillInstall("user-1"); ok {
		t.Fatal("expected pending interaction to be cleared after select confirm")
	}
}

func TestResolvePendingInteractionSelectDenyAlias(t *testing.T) {
	ch := &Channel{
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/select 2")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "已取消安装。" {
		t.Fatalf("unexpected deny reply: %q", reply)
	}
	if _, ok := ch.getPendingSkillInstall("user-1"); ok {
		t.Fatal("expected pending interaction to be cleared after select deny")
	}
}

func TestResolvePendingInteractionInvalidSelectShowsSupportedOptions(t *testing.T) {
	ch := &Channel{
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/select 3")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if !strings.Contains(reply, "/select 1") || !strings.Contains(reply, "/select 2") {
		t.Fatalf("expected supported options hint, got %q", reply)
	}
	if _, ok := ch.getPendingSkillInstall("user-1"); !ok {
		t.Fatal("expected pending interaction to remain after invalid select")
	}
}

func TestResolvePendingInteractionNumericConfirmAlias(t *testing.T) {
	registry := commands.NewRegistry()
	if err := registry.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if got := strings.TrimSpace(req.Args); got != "__confirm_install__ owner/repo" {
				t.Fatalf("unexpected args: %q", got)
			}
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch := &Channel{
		commands:             registry,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "1")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "installed" {
		t.Fatalf("expected reply %q, got %q", "installed", reply)
	}
}

func TestResolvePendingInteractionNumericDenyAlias(t *testing.T) {
	ch := &Channel{
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	ch.setPendingSkillInstall("user-1", pendingSkillInstall{
		UserID:    "user-1",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "2")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected interaction to be handled")
	}
	if reply != "已取消安装。" {
		t.Fatalf("unexpected deny reply: %q", reply)
	}
}

func TestResolvePendingInteractionDelegatesToRuntimeApprovals(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return &fakeACPInteractiveClient{
			prompt: "Claude 需要确认:\n\n是否允许执行 ReadFile？\n\n/yes 允许，/no 拒绝。",
			reply:  "runtime resumed",
		}, nil
	}

	ctx := context.Background()
	if _, err := controlSvc.CreateRuntime(ctx, "user-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if _, err := controlSvc.SendToRuntime(ctx, "user-1", "", "hello acp"); err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}

	ch := &Channel{
		runtime:              controlSvc,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "/yes")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected runtime interaction to be handled")
	}
	if reply != "runtime resumed" {
		t.Fatalf("unexpected runtime interaction reply: %q", reply)
	}
}

func TestResolvePendingInteractionDelegatesNumericSelectToRuntimeApprovals(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "运行时需要确认: WriteFile\n\n1. 允许一次\n2. 总是允许\n3. 拒绝\n\n回复 /select N 选择对应选项。",
		reply:  "runtime resumed",
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}

	ctx := context.Background()
	if _, err := controlSvc.CreateRuntime(ctx, "user-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if _, err := controlSvc.SendToRuntime(ctx, "user-1", "", "hello acp"); err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}

	ch := &Channel{
		runtime:              controlSvc,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
	reply, handled, err := ch.resolvePendingInteraction(wxtypes.WeixinMessage{FromUserID: "user-1"}, "3")
	if err != nil {
		t.Fatalf("resolve pending interaction: %v", err)
	}
	if !handled {
		t.Fatal("expected runtime interaction to be handled")
	}
	if reply != "runtime resumed" {
		t.Fatalf("unexpected runtime interaction reply: %q", reply)
	}
	if fake.last.Type != interactionActionSelect || fake.last.Value != "3" {
		t.Fatalf("expected numeric select to be forwarded, got %#v", fake.last)
	}
}

func TestFormatWeChatPrompt(t *testing.T) {
	got := formatWeChatPrompt("", &commands.CommandInteraction{
		Type:    commands.InteractionTypeSkillInstallConfirm,
		Repo:    "owner/repo",
		Reason:  "best match",
		Message: "请确认安装。",
	})
	if !strings.Contains(got, "请确认安装。") {
		t.Fatalf("expected prompt message, got %q", got)
	}
	if !strings.Contains(got, "/yes") || !strings.Contains(got, "/no") {
		t.Fatalf("expected action hints, got %q", got)
	}
}

func TestFormatWeChatPromptIncludesSelectAliasesForSkillInstall(t *testing.T) {
	got := formatWeChatPrompt("", &commands.CommandInteraction{
		Type:    commands.InteractionTypeSkillInstallConfirm,
		Repo:    "owner/repo",
		Message: "请确认安装。",
	})

	if !strings.Contains(got, "/select 1") {
		t.Fatalf("expected /select 1 hint, got %q", got)
	}
	if !strings.Contains(got, "/select 2") {
		t.Fatalf("expected /select 2 hint, got %q", got)
	}
}

func TestFormatWeChatPromptIncludesNumberedChoicesForSkillInstall(t *testing.T) {
	got := formatWeChatPrompt("", &commands.CommandInteraction{
		Type:    commands.InteractionTypeSkillInstallConfirm,
		Repo:    "owner/repo",
		Message: "请确认安装。",
	})

	if !strings.Contains(got, "1. 允许安装") {
		t.Fatalf("expected numbered allow option, got %q", got)
	}
	if !strings.Contains(got, "2. 拒绝安装") {
		t.Fatalf("expected numbered deny option, got %q", got)
	}
	if !strings.Contains(got, "也可以直接回复 1 或 2") {
		t.Fatalf("expected numeric shortcut hint, got %q", got)
	}
}
