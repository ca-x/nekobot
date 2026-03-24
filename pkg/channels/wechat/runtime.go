package wechat

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/config"
	"nekobot/pkg/toolsessions"
)

const wechatConversationPrefix = "wx:"

// RuntimeSpec describes a runtime request coming from WeChat control commands.
type RuntimeSpec struct {
	Driver  string
	Tool    string
	Command string
	Workdir string
}

// RuntimePreset is the normalized runtime launch config for a tool session.
type RuntimePreset struct {
	Driver   string
	Tool     string
	Command  string
	Workdir  string
	Metadata map[string]interface{}
}

// RuntimeBindingService manages WeChat chat-to-runtime bindings on top of tool sessions.
type RuntimeBindingService struct {
	mgr *toolsessions.Manager
	cfg *config.Config
}

// NewRuntimeBindingService creates a new WeChat runtime binding service.
func NewRuntimeBindingService(mgr *toolsessions.Manager, cfg *config.Config) *RuntimeBindingService {
	return &RuntimeBindingService{mgr: mgr, cfg: cfg}
}

// BindConversation binds a WeChat conversation to a tool session.
func (s *RuntimeBindingService) BindConversation(
	ctx context.Context,
	chatID, sessionID string,
) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	return s.mgr.BindSessionConversation(
		ctx,
		sessionID,
		toolsessions.SourceChannel,
		"wechat",
		wechatConversationKey(chatID),
	)
}

// ResolveConversation resolves the currently bound tool session for a WeChat chat.
func (s *RuntimeBindingService) ResolveConversation(
	ctx context.Context,
	chatID string,
) (*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return s.mgr.FindSessionByConversation(
		ctx,
		toolsessions.SourceChannel,
		"wechat",
		wechatConversationKey(chatID),
	)
}

// ClearConversation clears the binding for a WeChat chat.
func (s *RuntimeBindingService) ClearConversation(ctx context.Context, chatID string) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	return s.mgr.ClearConversationBinding(
		ctx,
		toolsessions.SourceChannel,
		"wechat",
		wechatConversationKey(chatID),
	)
}

// ListBindings lists all current WeChat conversation bindings.
func (s *RuntimeBindingService) ListBindings(ctx context.Context) ([]*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return s.mgr.ListSessions(ctx, toolsessions.ListSessionsInput{
		Source: toolsessions.SourceChannel,
		Limit:  200,
	})
}

// BuildRuntimePreset normalizes WeChat runtime creation requests.
func BuildRuntimePreset(cfg *config.Config, spec RuntimeSpec) (RuntimePreset, error) {
	driver := strings.TrimSpace(strings.ToLower(spec.Driver))
	if driver == "" {
		driver = "process"
	}

	tool := strings.TrimSpace(spec.Tool)
	command := strings.TrimSpace(spec.Command)
	workdir := strings.TrimSpace(spec.Workdir)
	if workdir == "" && cfg != nil {
		workdir = cfg.WorkspacePath()
	}

	switch driver {
	case "acp":
		if command == "" {
			command = resolveACPRuntimeCommand(tool)
		}
		if command == "" {
			return RuntimePreset{}, fmt.Errorf("command is required for acp runtime")
		}
		if tool == "" {
			tool = logicalToolNameForCommand(command)
		}
	case "codex":
		if tool == "" || strings.EqualFold(tool, "codex") {
			tool = "codex"
		} else {
			tool = "codex"
		}
		if command == "" {
			command = "codex"
		}
	case "process":
		if command == "" {
			command = tool
		}
		if command == "" {
			return RuntimePreset{}, fmt.Errorf("command is required for process runtime")
		}
		if tool == "" {
			tool = firstCommandToken(command)
		}
	default:
		return RuntimePreset{}, fmt.Errorf("unsupported runtime driver: %s", driver)
	}

	if tool == "" {
		return RuntimePreset{}, fmt.Errorf("tool is required")
	}
	if workdir == "" {
		return RuntimePreset{}, fmt.Errorf("workdir is required")
	}

	return RuntimePreset{
		Driver:  driver,
		Tool:    tool,
		Command: command,
		Workdir: workdir,
		Metadata: map[string]interface{}{
			"driver": driver,
		},
	}, nil
}

func wechatConversationKey(chatID string) string {
	trimmed := strings.TrimSpace(chatID)
	if trimmed == "" {
		return ""
	}
	return wechatConversationPrefix + trimmed
}

func firstCommandToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func resolveACPRuntimeCommand(tool string) string {
	switch strings.TrimSpace(strings.ToLower(tool)) {
	case "claude", "claude-code", "claude-agent-acp":
		return "claude-agent-acp"
	case "codex", "codex-acp":
		return "codex-acp"
	case "cursor", "agent":
		return "agent acp"
	case "gemini":
		return "gemini --acp"
	case "opencode":
		return "opencode acp"
	default:
		return strings.TrimSpace(tool)
	}
}

func logicalToolNameForCommand(command string) string {
	cmd := firstCommandToken(command)
	switch strings.TrimSpace(strings.ToLower(cmd)) {
	case "claude-agent-acp":
		return "claude"
	case "codex-acp":
		return "codex"
	default:
		return cmd
	}
}
