package wechat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/commands"
	wxtypes "nekobot/pkg/wechat/types"
)

type interactionActionType string

const (
	interactionActionConfirm     interactionActionType = "confirm"
	interactionActionDeny        interactionActionType = "deny"
	interactionActionSelect      interactionActionType = "select"
	interactionActionPassthrough interactionActionType = "passthrough"
)

type interactionAction struct {
	Type  interactionActionType
	Value string
}

type pendingSkillInstall struct {
	UserID    string
	Command   string
	Repo      string
	CreatedAt time.Time
}

func formatWeChatPrompt(promptText string, interaction *commands.CommandInteraction) string {
	if interaction == nil {
		return strings.TrimSpace(promptText)
	}

	repo := strings.TrimSpace(interaction.Repo)
	message := strings.TrimSpace(interaction.Message)
	reason := strings.TrimSpace(interaction.Reason)

	var body string
	switch {
	case message != "":
		body = message
	case repo != "":
		body = fmt.Sprintf("已找到候选技能：%s", repo)
	default:
		body = strings.TrimSpace(promptText)
	}

	if reason != "" {
		body += "\n\n原因：" + reason
	}
	if body == "" {
		body = "Claude 正在等待确认。"
	}

	return body + "\n回复 /yes 或 /select 1 允许，/no、/cancel 或 /select 2 拒绝。"
}

func parseWeChatInteractionAction(input string) (*interactionAction, bool) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	switch {
	case lower == "/yes" || lower == "/y" || lower == "/ok" || lower == "/allow" || lower == "/enter":
		return &interactionAction{Type: interactionActionConfirm}, true
	case lower == "/no" || lower == "/n" || lower == "/cancel" || lower == "/deny":
		return &interactionAction{Type: interactionActionDeny}, true
	case lower == "1":
		return &interactionAction{Type: interactionActionSelect, Value: "1"}, true
	case lower == "2":
		return &interactionAction{Type: interactionActionSelect, Value: "2"}, true
	case strings.HasPrefix(lower, "/select "):
		value := strings.TrimSpace(trimmed[len("/select "):])
		if value == "" {
			return nil, false
		}
		return &interactionAction{Type: interactionActionSelect, Value: value}, true
	case strings.HasPrefix(trimmed, "/"):
		return &interactionAction{Type: interactionActionPassthrough, Value: trimmed}, true
	default:
		return nil, false
	}
}

func (c *Channel) resolvePendingInteraction(msg wxtypes.WeixinMessage, content string) (string, bool, error) {
	action, ok := parseWeChatInteractionAction(content)
	if !ok || action == nil {
		return "", false, nil
	}
	if action.Type == interactionActionPassthrough {
		return "", false, nil
	}

	if c.runtime != nil {
		reply, handled, err := c.runtime.ResolvePendingInteraction(context.Background(), msg.FromUserID, action)
		if err != nil {
			return "", true, err
		}
		if handled {
			return reply, true, nil
		}
	}

	pending, hasPending := c.getPendingSkillInstall(msg.FromUserID)
	if !hasPending {
		return "当前没有待确认的操作。", true, nil
	}

	switch action.Type {
	case interactionActionDeny:
		c.clearPendingSkillInstall(msg.FromUserID)
		return "已取消安装。", true, nil
	case interactionActionSelect:
		switch strings.TrimSpace(action.Value) {
		case "1":
			c.clearPendingSkillInstall(msg.FromUserID)
			reply, err := c.executeConfirmedSkillInstall(msg, pending)
			if err != nil {
				return "", true, err
			}
			return reply, true, nil
		case "2":
			c.clearPendingSkillInstall(msg.FromUserID)
			return "已取消安装。", true, nil
		default:
			return "当前操作不支持该 /select 选项。", true, nil
		}
	case interactionActionConfirm:
		c.clearPendingSkillInstall(msg.FromUserID)
		reply, err := c.executeConfirmedSkillInstall(msg, pending)
		if err != nil {
			return "", true, err
		}
		return reply, true, nil
	default:
		return "", false, nil
	}
}

func (c *Channel) executeConfirmedSkillInstall(
	msg wxtypes.WeixinMessage,
	pending pendingSkillInstall,
) (string, error) {
	if c.commands == nil {
		return "❌ 安装失败：命令不可用。", nil
	}

	cmd, exists := c.commands.Get(pending.Command)
	if !exists {
		return "❌ 安装失败：命令不存在。", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   msg.FromUserID,
		UserID:   msg.FromUserID,
		Username: msg.FromUserID,
		Command:  pending.Command,
		Args:     "__confirm_install__ " + pending.Repo,
		Metadata: map[string]string{
			"skill_install_confirmed_repo": pending.Repo,
		},
	})
	if err != nil {
		return "", fmt.Errorf("execute confirmed skill install: %w", err)
	}
	if strings.TrimSpace(resp.Content) == "" {
		return "✅ 安装流程已执行（无额外输出）。", nil
	}
	return resp.Content, nil
}
