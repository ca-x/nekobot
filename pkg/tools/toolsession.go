package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
)

// ToolSessionTool lets the agent create and manage tool sessions.
type ToolSessionTool struct {
	processMgr     *process.Manager
	toolSessionMgr *toolsessions.Manager
	workspace      string
}

// NewToolSessionTool creates a new ToolSessionTool.
func NewToolSessionTool(processMgr *process.Manager, toolSessionMgr *toolsessions.Manager, workspace string) *ToolSessionTool {
	return &ToolSessionTool{
		processMgr:     processMgr,
		toolSessionMgr: toolSessionMgr,
		workspace:      workspace,
	}
}

func (t *ToolSessionTool) Name() string {
	return "tool_session"
}

func (t *ToolSessionTool) Description() string {
	return "Create and manage tool sessions (coding assistants like codex, claude, aider, etc.). " +
		"Use 'spawn' to start a new tool session, 'list' to see active sessions, or 'terminate' to stop one."
}

func (t *ToolSessionTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"spawn", "list", "terminate"},
				"description": "Action to perform: spawn (create a new tool session), list (show agent-created sessions), terminate (stop a session)",
			},
			"tool": map[string]interface{}{
				"type":        "string",
				"description": "Tool name for spawn action: codex, claude, aider, opencode, or a custom command name",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Full command to execute (optional for spawn; defaults to the tool name)",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for the session (optional; defaults to workspace)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Session title (optional)",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID (required for terminate action)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ToolSessionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action must be a string")
	}

	switch action {
	case "spawn":
		return t.handleSpawn(ctx, args)
	case "list":
		return t.handleList(ctx)
	case "terminate":
		return t.handleTerminate(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s (valid: spawn, list, terminate)", action)
	}
}

func (t *ToolSessionTool) handleSpawn(ctx context.Context, args map[string]interface{}) (string, error) {
	toolName, _ := args["tool"].(string)
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return "", fmt.Errorf("tool is required for spawn action")
	}

	command, _ := args["command"].(string)
	command = t.resolveCommand(toolName, strings.TrimSpace(command))
	if command == "" {
		return "", fmt.Errorf("could not resolve command for tool: %s", toolName)
	}

	workdir, _ := args["workdir"].(string)
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		workdir = t.workspace
	}

	title, _ := args["title"].(string)
	title = strings.TrimSpace(title)

	metadata := map[string]interface{}{
		"user_command": command,
	}

	sess, err := t.toolSessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Source:   toolsessions.SourceAgent,
		Tool:     toolName,
		Title:    title,
		Command:  command,
		Workdir:  workdir,
		State:    toolsessions.StateRunning,
		Metadata: metadata,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create tool session: %w", err)
	}

	launchCommand := command
	tmuxSession := ""
	if wrapped, sessionName := buildRuntimeCommand(launchCommand, sess.ID); sessionName != "" {
		launchCommand = wrapped
		tmuxSession = sessionName
	}

	if err := t.processMgr.Start(context.Background(), sess.ID, launchCommand, workdir); err != nil {
		_ = t.toolSessionMgr.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error())
		return "", fmt.Errorf("failed to start tool process: %w", err)
	}

	result := fmt.Sprintf("Tool session created successfully.\nSession ID: %s\nTool: %s\nCommand: %s\nWorkdir: %s", sess.ID, toolName, command, workdir)
	if tmuxSession != "" {
		result += fmt.Sprintf("\nTmux session: %s", tmuxSession)
	}
	return result, nil
}

func (t *ToolSessionTool) handleList(ctx context.Context) (string, error) {
	sessions, err := t.toolSessionMgr.ListSessions(ctx, toolsessions.ListSessionsInput{
		Source: toolsessions.SourceAgent,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return "No agent-created tool sessions found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent tool sessions (%d):\n", len(sessions)))
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = s.Tool
		}
		sb.WriteString(fmt.Sprintf("  - [%s] %s (tool=%s, state=%s, command=%s)\n", s.ID, title, s.Tool, s.State, s.Command))
	}
	return sb.String(), nil
}

func (t *ToolSessionTool) handleTerminate(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required for terminate action")
	}

	// Kill the process first
	if err := t.processMgr.Kill(sessionID); err != nil {
		// Process may already be dead; continue to terminate the session record
	}

	if err := t.toolSessionMgr.TerminateSession(ctx, sessionID, "terminated by agent"); err != nil {
		return "", fmt.Errorf("failed to terminate session %s: %w", sessionID, err)
	}

	return fmt.Sprintf("Session %s terminated.", sessionID), nil
}

// resolveCommand maps a tool name to its default command, or uses the provided command.
func (t *ToolSessionTool) resolveCommand(toolName, command string) string {
	if command != "" {
		return command
	}
	tool := strings.TrimSpace(strings.ToLower(toolName))
	switch tool {
	case "codex":
		return "codex"
	case "claude":
		return "claude"
	case "opencode":
		return "opencode"
	case "aider":
		return "aider"
	default:
		return strings.TrimSpace(toolName)
	}
}

// buildRuntimeCommand wraps a command in tmux if available.
func buildRuntimeCommand(command, sessionID string) (string, string) {
	if !isTmuxAvailable() {
		return command, ""
	}
	name := buildTmuxSessionName(sessionID)
	shell := findShellPath()
	wrapped := fmt.Sprintf("tmux new-session -A -s %s %s -c %s", name, strconv.Quote(shell), strconv.Quote(command))
	return wrapped, name
}

func isTmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func findShellPath() string {
	candidates := []string{
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/bash",
		"/usr/bin/bash",
	}
	for _, c := range candidates {
		if isExecFile(c) {
			return c
		}
	}
	return "/bin/sh"
}

func isExecFile(path string) bool {
	info, err := exec.LookPath(path)
	return err == nil && info != ""
}

func buildTmuxSessionName(sessionID string) string {
	raw := strings.TrimSpace(strings.ToLower(sessionID))
	if raw == "" {
		return "nekobot_session"
	}
	var b strings.Builder
	b.WriteString("neko_")
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}
