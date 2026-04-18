package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/process"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/toolsessions"
)

// ToolSessionTool lets the agent create and manage tool sessions.
type ToolSessionTool struct {
	processMgr       *process.Manager
	toolSessionMgr   *toolsessions.Manager
	cfg              *config.Config
	processProbe     externalagent.ProcessProbe
	processStarter   externalagent.ProcessStarter
	sessionUpdater   externalagent.SessionUpdater
	runtimeTransport externalagent.RuntimeTransport
}

// NewToolSessionTool creates a new ToolSessionTool.
func NewToolSessionTool(processMgr *process.Manager, toolSessionMgr *toolsessions.Manager, cfg *config.Config) *ToolSessionTool {
	return &ToolSessionTool{
		processMgr:       processMgr,
		toolSessionMgr:   toolSessionMgr,
		cfg:              cfg,
		processProbe:     processManagerProbe{manager: processMgr},
		processStarter:   processMgr,
		sessionUpdater:   toolSessionMgr,
		runtimeTransport: runtimeagents.DefaultTransport(),
	}
}

func (t *ToolSessionTool) Name() string {
	return "tool_session"
}

func (t *ToolSessionTool) Description() string {
	return "Create and manage tool sessions (coding assistants like codex, claude, aider, etc.). " +
		"Use 'spawn' to start a new tool session, 'list' to see active sessions, or 'terminate' to stop one. " +
		"Sessions you create are visible in the web dashboard under Tool Sessions with an 'Agent' badge, " +
		"allowing users to monitor progress, view terminal output, and interact with the session through the UI."
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
	workdir, _ := args["workdir"].(string)
	title, _ := args["title"].(string)
	resolved, err := t.resolveSpawnSpec(toolName, strings.TrimSpace(command), strings.TrimSpace(workdir), strings.TrimSpace(title))
	if err != nil {
		return "", err
	}
	toolName = resolved.Tool
	command = resolved.Command
	workdir = resolved.Workspace
	title = resolved.Title

	metadata := map[string]interface{}{
		"user_command": command,
	}
	baseSpec := execenv.StartSpecFromContext(ctx, "", "", "", nil)
	runtimeID := baseSpec.RuntimeID
	if runtimeID != "" {
		metadata[execenv.MetadataRuntimeID] = runtimeID
	}
	taskID := baseSpec.TaskID
	if taskID != "" {
		metadata[execenv.MetadataTaskID] = taskID
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
	if taskID == "" {
		taskID = sess.ID
		metadata[execenv.MetadataTaskID] = taskID
		if err := t.toolSessionMgr.UpdateSessionMetadata(ctx, sess.ID, metadata); err != nil {
			if terminateErr := t.toolSessionMgr.TerminateSession(context.Background(), sess.ID, "failed to persist task metadata: "+err.Error()); terminateErr != nil {
				return "", fmt.Errorf("failed to persist tool session metadata: %w (terminate session: %v)", err, terminateErr)
			}
			return "", fmt.Errorf("failed to persist tool session metadata: %w", err)
		}
	}
	sess.Metadata = metadata

	if t.isSupportedExternalAgentKind(toolName) {
		if err := externalagent.EnsureProcess(
			ctx,
			t.workspace(),
			t.processProbe,
			t.processStarter,
			t.sessionUpdater,
			t.runtimeTransport,
			sess,
		); err != nil {
			if terminateErr := t.toolSessionMgr.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error()); terminateErr != nil {
				return "", fmt.Errorf("failed to start tool process: %w (terminate session: %v)", err, terminateErr)
			}
			return "", fmt.Errorf("failed to start tool process: %w", err)
		}
		refreshed, err := t.toolSessionMgr.GetSession(ctx, sess.ID)
		if err == nil && refreshed != nil {
			sess = refreshed
			toolName = sess.Tool
			command = sess.Command
			workdir = sess.Workdir
		}
		result := fmt.Sprintf("Tool session created successfully.\nSession ID: %s\nTool: %s\nCommand: %s\nWorkdir: %s", sess.ID, toolName, command, workdir)
		if got := runtimeagents.MetadataString(sess.Metadata, runtimeagents.MetadataRuntimeSession); got != "" {
			result += fmt.Sprintf("\nRuntime session: %s", got)
		}
		return result, nil
	}

	launchCommand := command
	tmuxSession := ""
	if t.runtimeTransport != nil {
		launchInfo := t.runtimeTransport.WrapStart(launchCommand, sess.ID)
		launchCommand = launchInfo.LaunchCommand
		tmuxSession = launchInfo.SessionName
		metadata = runtimeagents.ApplyLaunchMetadata(metadata, launchInfo)
		if err := t.toolSessionMgr.UpdateSessionMetadata(ctx, sess.ID, metadata); err != nil {
			if terminateErr := t.toolSessionMgr.TerminateSession(context.Background(), sess.ID, "failed to persist runtime metadata: "+err.Error()); terminateErr != nil {
				return "", fmt.Errorf("failed to persist runtime metadata: %w (terminate session: %v)", err, terminateErr)
			}
			return "", fmt.Errorf("failed to persist runtime metadata: %w", err)
		}
	}

	spec := execenv.StartSpecFromContext(ctx, sess.ID, launchCommand, workdir, metadata)
	if err := t.processMgr.StartWithSpec(context.Background(), spec); err != nil {
		if terminateErr := t.toolSessionMgr.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error()); terminateErr != nil {
			return "", fmt.Errorf("failed to start tool process: %w (terminate session: %v)", err, terminateErr)
		}
		return "", fmt.Errorf("failed to start tool process: %w", err)
	}

	result := fmt.Sprintf("Tool session created successfully.\nSession ID: %s\nTool: %s\nCommand: %s\nWorkdir: %s", sess.ID, toolName, command, workdir)
	if tmuxSession != "" {
		result += fmt.Sprintf("\nRuntime session: %s", tmuxSession)
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
	_, _ = fmt.Fprintf(&sb, "Agent tool sessions (%d):\n", len(sessions))
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = s.Tool
		}
		_, _ = fmt.Fprintf(&sb, "  - [%s] %s (tool=%s, state=%s, command=%s)\n", s.ID, title, s.Tool, s.State, s.Command)
	}
	return sb.String(), nil
}

func (t *ToolSessionTool) handleTerminate(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required for terminate action")
	}

	// Kill the process first. If it is already gone, still terminate the session record.
	killErr := t.processMgr.Kill(sessionID)

	if err := t.toolSessionMgr.TerminateSession(ctx, sessionID, "terminated by agent"); err != nil {
		return "", fmt.Errorf("failed to terminate session %s: %w", sessionID, err)
	}
	if killErr != nil {
		return fmt.Sprintf("Session %s terminated (process stop warning: %v).", sessionID, killErr), nil
	}

	return fmt.Sprintf("Session %s terminated.", sessionID), nil
}

func (t *ToolSessionTool) resolveSpawnSpec(toolName, command, workdir, title string) (externalagent.SessionSpec, error) {
	spec := externalagent.SessionSpec{
		Owner:     "agent",
		AgentKind: toolName,
		Workspace: workdir,
		Tool:      toolName,
		Title:     title,
		Command:   command,
	}
	if t.isSupportedExternalAgentKind(toolName) {
		return externalagent.NormalizeLaunchSpec(t.cfg, spec)
	}

	command = t.resolveCommand(toolName, command)
	if command == "" {
		return externalagent.SessionSpec{}, fmt.Errorf("could not resolve command for tool: %s", toolName)
	}
	if strings.TrimSpace(workdir) == "" {
		workdir = t.workspace()
	}
	if strings.TrimSpace(workdir) == "" {
		return externalagent.SessionSpec{}, fmt.Errorf("workdir is required")
	}
	spec.Workspace = workdir
	spec.Command = command
	spec.Title = title
	return spec, nil
}

func (t *ToolSessionTool) isSupportedExternalAgentKind(toolName string) bool {
	_, err := externalagent.NormalizeLaunchSpec(t.cfg, externalagent.SessionSpec{
		Owner:     "agent",
		AgentKind: strings.TrimSpace(toolName),
		Workspace: t.workspace(),
	})
	return err == nil
}

func (t *ToolSessionTool) workspace() string {
	if t == nil || t.cfg == nil {
		return ""
	}
	return strings.TrimSpace(t.cfg.WorkspacePath())
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
	launchInfo := runtimeagents.DefaultTransport().WrapStart(command, sessionID)
	return launchInfo.LaunchCommand, launchInfo.SessionName
}

func isTmuxAvailable() bool {
	return runtimeagents.DefaultTransport().Available()
}

func buildTmuxSessionName(sessionID string) string {
	return runtimeagents.TmuxSessionName(sessionID)
}

type processManagerProbe struct {
	manager *process.Manager
}

func (p processManagerProbe) HasProcess(sessionID string) bool {
	if p.manager == nil {
		return false
	}
	_, err := p.manager.GetStatus(strings.TrimSpace(sessionID))
	return err == nil
}
