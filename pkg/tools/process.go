package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/process"
)

// ProcessTool allows the agent to manage background PTY sessions.
type ProcessTool struct {
	processManager *process.Manager
}

// NewProcessTool creates a new process tool.
func NewProcessTool(pm *process.Manager) *ProcessTool {
	return &ProcessTool{
		processManager: pm,
	}
}

func (t *ProcessTool) Name() string {
	return "process"
}

func (t *ProcessTool) Description() string {
	return "Manage background PTY sessions started by exec tool. List, poll, read logs, write input, or kill sessions."
}

func (t *ProcessTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "poll", "log", "write", "kill"},
				"description": "Action to perform: list (all sessions), poll (check status), log (get output), write (send input), kill (terminate)",
			},
			"sessionId": map[string]interface{}{
				"type":        "string",
				"description": "Session ID (required for poll, log, write, kill)",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Offset for log action (default: 0)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Limit for log action (default: 100, 0 = all)",
			},
			"data": map[string]interface{}{
				"type":        "string",
				"description": "Data to write for write action",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ProcessTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.processManager == nil {
		return "", fmt.Errorf("process manager not available")
	}

	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action must be a string")
	}

	switch action {
	case "list":
		return t.handleList()
	case "poll":
		return t.handlePoll(args)
	case "log":
		return t.handleLog(args)
	case "write":
		return t.handleWrite(args)
	case "kill":
		return t.handleKill(args)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ProcessTool) handleList() (string, error) {
	sessions := t.processManager.List()

	if len(sessions) == 0 {
		return "No background sessions", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Background Sessions (%d):\n\n", len(sessions)))

	for i, s := range sessions {
		status := "Running"
		if !s.Running {
			status = fmt.Sprintf("Exited (%d)", s.ExitCode)
		}

		result.WriteString(fmt.Sprintf("%d. Session ID: %s\n", i+1, s.ID))
		result.WriteString(fmt.Sprintf("   Command: %s\n", s.Command))
		result.WriteString(fmt.Sprintf("   Status: %s\n", status))
		result.WriteString(fmt.Sprintf("   Duration: %v\n", s.Duration))
		result.WriteString(fmt.Sprintf("   Output Size: %d lines\n", s.OutputSize))
		result.WriteString("\n")
	}

	return result.String(), nil
}

func (t *ProcessTool) handlePoll(args map[string]interface{}) (string, error) {
	sessionID, ok := args["sessionId"].(string)
	if !ok {
		return "", fmt.Errorf("sessionId must be a string")
	}

	status, err := t.processManager.GetStatus(sessionID)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Session: %s\n", status.ID))
	result.WriteString(fmt.Sprintf("Command: %s\n", status.Command))
	result.WriteString(fmt.Sprintf("Workdir: %s\n", status.Workdir))
	result.WriteString(fmt.Sprintf("Started: %s\n", status.StartedAt.Format("2006-01-02 15:04:05")))

	if status.Running {
		result.WriteString(fmt.Sprintf("Status: Running\n"))
		result.WriteString(fmt.Sprintf("Duration: %v\n", status.Duration))
	} else {
		result.WriteString(fmt.Sprintf("Status: Exited\n"))
		result.WriteString(fmt.Sprintf("Exit Code: %d\n", status.ExitCode))
		result.WriteString(fmt.Sprintf("Duration: %v\n", status.Duration))
	}

	result.WriteString(fmt.Sprintf("Output Size: %d lines\n", status.OutputSize))

	return result.String(), nil
}

func (t *ProcessTool) handleLog(args map[string]interface{}) (string, error) {
	sessionID, ok := args["sessionId"].(string)
	if !ok {
		return "", fmt.Errorf("sessionId must be a string")
	}

	offset := getIntArg(args, "offset", 0)
	limit := getIntArg(args, "limit", 100)

	lines, total, err := t.processManager.GetOutput(sessionID, offset, limit)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
	result.WriteString(fmt.Sprintf("Total Lines: %d\n", total))
	result.WriteString(fmt.Sprintf("Showing: %d-%d\n\n", offset, offset+len(lines)))

	if len(lines) > 0 {
		result.WriteString("OUTPUT:\n")
		result.WriteString(strings.Join(lines, "\n"))
		result.WriteString("\n")
	} else {
		result.WriteString("(No output yet)\n")
	}

	return result.String(), nil
}

func (t *ProcessTool) handleWrite(args map[string]interface{}) (string, error) {
	sessionID, ok := args["sessionId"].(string)
	if !ok {
		return "", fmt.Errorf("sessionId must be a string")
	}

	data, ok := args["data"].(string)
	if !ok {
		return "", fmt.Errorf("data must be a string")
	}

	if err := t.processManager.Write(sessionID, data); err != nil {
		return "", err
	}

	return fmt.Sprintf("Sent %d bytes to session %s", len(data), sessionID), nil
}

func (t *ProcessTool) handleKill(args map[string]interface{}) (string, error) {
	sessionID, ok := args["sessionId"].(string)
	if !ok {
		return "", fmt.Errorf("sessionId must be a string")
	}

	if err := t.processManager.Kill(sessionID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Session %s terminated", sessionID), nil
}
