package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"

	"nekobot/pkg/process"
)

// ExecTool allows the agent to execute shell commands.
type ExecTool struct {
	workspace      string
	restrict       bool
	timeout        time.Duration
	processManager *process.Manager
}

// NewExecTool creates a new exec tool.
func NewExecTool(workspace string, restrict bool, timeout time.Duration, pm *process.Manager) *ExecTool {
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30s timeout
	}
	return &ExecTool{
		workspace:      workspace,
		restrict:       restrict,
		timeout:        timeout,
		processManager: pm,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command. Supports PTY mode for interactive tools and background execution."
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"pty": map[string]interface{}{
				"type":        "boolean",
				"description": "Use PTY mode for interactive tools (vim, htop, coding agents). Default: false",
			},
			"background": map[string]interface{}{
				"type":        "boolean",
				"description": "Run in background and return session ID. Use with process tool to monitor. Default: false",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory (relative to workspace). Default: workspace root",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds. Only for non-background mode. Default: 30",
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

	// Parse options
	usePTY := getBoolArg(args, "pty", false)
	background := getBoolArg(args, "background", false)
	workdir := getStringArg(args, "workdir", "")
	timeout := getIntArg(args, "timeout", int(t.timeout.Seconds()))

	// Resolve workdir
	if workdir == "" {
		workdir = t.workspace
	} else {
		// Relative to workspace
		if !strings.HasPrefix(workdir, "/") {
			workdir = t.workspace + "/" + workdir
		}
	}

	// Basic security: prevent dangerous commands
	if t.restrict {
		dangerous := []string{"rm -rf /", "dd if=", "mkfs", "> /dev/", ":(){ :|:& };:", "curl | sh", "wget | sh"}
		for _, d := range dangerous {
			if strings.Contains(command, d) {
				return "", fmt.Errorf("potentially dangerous command blocked: contains '%s'", d)
			}
		}
	}

	// Background mode
	if background {
		if t.processManager == nil {
			return "", fmt.Errorf("process manager not available")
		}

		sessionID := uuid.New().String()
		if err := t.processManager.Start(ctx, sessionID, command, workdir); err != nil {
			return "", fmt.Errorf("starting background process: %w", err)
		}

		return fmt.Sprintf("Background process started\nSession ID: %s\nCommand: %s\nWorkdir: %s\n\nUse 'process' tool to monitor:\n- process action:log sessionId:%s\n- process action:poll sessionId:%s\n- process action:kill sessionId:%s",
			sessionID, command, workdir, sessionID, sessionID, sessionID), nil
	}

	// PTY mode
	if usePTY {
		return t.executeWithPTY(ctx, command, workdir, time.Duration(timeout)*time.Second)
	}

	// Standard mode
	return t.executeStandard(ctx, command, workdir, time.Duration(timeout)*time.Second)
}

// executeStandard executes command in standard mode.
func (t *ExecTool) executeStandard(ctx context.Context, command, workdir string, timeout time.Duration) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Command: %s\n", command))
	result.WriteString(fmt.Sprintf("Working Directory: %s\n\n", workdir))

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
			result.WriteString(fmt.Sprintf("(Command timed out after %v)\n", timeout))
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

// executeWithPTY executes command with PTY.
func (t *ExecTool) executeWithPTY(ctx context.Context, command, workdir string, timeout time.Duration) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = workdir

	// Start with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("starting PTY: %w", err)
	}
	defer ptmx.Close()

	// Capture output
	var output strings.Builder
	outputChan := make(chan string, 100)
	errChan := make(chan error, 1)

	// Read goroutine
	go func() {
		scanner := bufio.NewScanner(ptmx)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			errChan <- err
		}
		close(outputChan)
	}()

	// Wait for completion
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- cmd.Wait()
	}()

	// Collect output
	done := false
	for !done {
		select {
		case line, ok := <-outputChan:
			if !ok {
				done = true
			} else {
				output.WriteString(line)
				output.WriteString("\n")
			}
		case <-execCtx.Done():
			// Timeout
			cmd.Process.Kill()
			return "", fmt.Errorf("command timed out after %v", timeout)
		case err := <-errChan:
			return "", fmt.Errorf("reading output: %w", err)
		}
	}

	// Wait for process to finish
	processErr := <-waitChan

	// Build result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Command: %s\n", command))
	result.WriteString(fmt.Sprintf("Working Directory: %s\n", workdir))
	result.WriteString(fmt.Sprintf("Mode: PTY\n\n"))

	if output.Len() > 0 {
		result.WriteString("OUTPUT:\n")
		result.WriteString(output.String())
		result.WriteString("\n")
	}

	exitCode := 0
	if processErr != nil {
		if exitErr, ok := processErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		result.WriteString(fmt.Sprintf("\nError: %v\n", processErr))
	}

	result.WriteString(fmt.Sprintf("\nExit Code: %d\n", exitCode))

	return result.String(), nil
}

// Helper functions
func getBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

func getStringArg(args map[string]interface{}, key string, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return defaultVal
	}
}
