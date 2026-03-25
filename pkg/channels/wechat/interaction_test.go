package wechat

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/commands"
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
