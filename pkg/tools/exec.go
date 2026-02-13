package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ExecTool allows the agent to execute shell commands.
type ExecTool struct {
	workspace string
	restrict  bool
	timeout   time.Duration
}

// NewExecTool creates a new exec tool.
func NewExecTool(workspace string, restrict bool, timeout time.Duration) *ExecTool {
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30s timeout
	}
	return &ExecTool{
		workspace: workspace,
		restrict:  restrict,
		timeout:   timeout,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command in the workspace directory. Returns stdout and stderr."
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command must be a string")
	}

	// Basic security: prevent dangerous commands
	if t.restrict {
		dangerous := []string{"rm -rf", "dd if=", "mkfs", "> /dev/", ":(){ :|:& };:", "curl | sh", "wget | sh"}
		for _, d := range dangerous {
			if strings.Contains(command, d) {
				return "", fmt.Errorf("potentially dangerous command blocked: contains '%s'", d)
			}
		}
	}

	// Create command with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = t.workspace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()

	// Build result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Command: %s\n", command))
	result.WriteString(fmt.Sprintf("Working Directory: %s\n\n", t.workspace))

	if stdout.Len() > 0 {
		result.WriteString("STDOUT:\n")
		result.WriteString(stdout.String())
		result.WriteString("\n")
	}

	if stderr.Len() > 0 {
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
		result.WriteString("\n")
	}

	if err != nil {
		result.WriteString(fmt.Sprintf("\nError: %v\n", err))
		if execCtx.Err() == context.DeadlineExceeded {
			result.WriteString(fmt.Sprintf("(Command timed out after %v)\n", t.timeout))
		}
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	result.WriteString(fmt.Sprintf("\nExit Code: %d\n", exitCode))

	return result.String(), nil
}
