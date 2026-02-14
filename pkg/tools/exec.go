package tools

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"

	"nekobot/pkg/process"
)

// ExecConfig controls ExecTool behavior.
type ExecConfig struct {
	Timeout time.Duration
	Sandbox DockerSandboxConfig
}

// DockerSandboxConfig controls containerized execution.
type DockerSandboxConfig struct {
	Enabled     bool
	Image       string
	NetworkMode string
	Mounts      []string
	Timeout     time.Duration
	AutoCleanup bool
}

// ExecTool allows the agent to execute shell commands.
type ExecTool struct {
	workspace      string
	restrict       bool
	config         ExecConfig
	processManager *process.Manager
	mu             sync.RWMutex
	sandboxOff     bool
	sandboxReason  string
}

// NewExecTool creates a new exec tool.
func NewExecTool(workspace string, restrict bool, cfg ExecConfig, pm *process.Manager) *ExecTool {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Sandbox.Image == "" {
		cfg.Sandbox.Image = "alpine:3.20"
	}
	if cfg.Sandbox.NetworkMode == "" {
		cfg.Sandbox.NetworkMode = "none"
	}
	if cfg.Sandbox.Timeout <= 0 {
		cfg.Sandbox.Timeout = 60 * time.Second
	}

	return &ExecTool{
		workspace:      workspace,
		restrict:       restrict,
		config:         cfg,
		processManager: pm,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command. Supports PTY mode, background execution, and optional Docker sandbox."
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
	timeout := getIntArg(args, "timeout", int(t.config.Timeout.Seconds()))

	// Resolve workdir
	if workdir == "" {
		workdir = t.workspace
	} else if !strings.HasPrefix(workdir, "/") {
		workdir = filepath.Join(t.workspace, workdir)
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

	execTimeout := time.Duration(timeout) * time.Second

	// Use Docker sandbox when enabled; fallback to direct execution if unavailable.
	if t.config.Sandbox.Enabled {
		if disabled, reason := t.isSandboxDisabled(); disabled {
			fallbackFn := t.executeStandard
			fallbackMode := "direct execution"
			if usePTY {
				fallbackFn = t.executeWithPTY
				fallbackMode = "direct PTY execution"
			}
			fallback, fallbackErr := fallbackFn(ctx, command, workdir, execTimeout)
			return "Docker sandbox is disabled for this process, fallback to " + fallbackMode + ".\nReason: " + reason + "\n\n" + fallback, fallbackErr
		}

		result, err := t.executeInDocker(ctx, command, workdir, execTimeout)
		if err == nil {
			return result, nil
		}
		var unavailableErr *sandboxUnavailableError
		if errors.As(err, &unavailableErr) {
			t.disableSandbox(unavailableErr.Error())
		}
		fallbackFn := t.executeStandard
		fallbackMode := "direct execution"
		if usePTY {
			fallbackFn = t.executeWithPTY
			fallbackMode = "direct PTY execution"
		}
		fallback, fallbackErr := fallbackFn(ctx, command, workdir, execTimeout)
		return "Docker sandbox unavailable, fallback to " + fallbackMode + ".\nReason: " + err.Error() + "\n\n" + fallback, fallbackErr
	}

	// PTY mode (direct execution only).
	if usePTY {
		return t.executeWithPTY(ctx, command, workdir, execTimeout)
	}

	// Standard mode
	return t.executeStandard(ctx, command, workdir, execTimeout)
}

func (t *ExecTool) executeInDocker(ctx context.Context, command, workdir string, timeout time.Duration) (string, error) {
	dockerTimeout := timeout
	if t.config.Sandbox.Timeout > dockerTimeout {
		dockerTimeout = t.config.Sandbox.Timeout
	}
	execCtx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", &sandboxUnavailableError{cause: fmt.Errorf("initializing docker client: %w", err)}
	}
	defer cli.Close()

	if _, err := cli.Ping(execCtx); err != nil {
		return "", &sandboxUnavailableError{cause: fmt.Errorf("docker daemon unavailable: %w", err)}
	}

	// Try to pull image (best effort, errors ignored because image may already exist and pull can be rate-limited).
	if reader, err := cli.ImagePull(execCtx, t.config.Sandbox.Image, image.PullOptions{}); err == nil {
		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}

	containerWorkdir := "/workspace"
	if rel, err := filepath.Rel(t.workspace, workdir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		containerWorkdir = filepath.ToSlash(filepath.Join("/workspace", rel))
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: t.workspace,
			Target: "/workspace",
		},
	}

	extraMounts, err := parseMountSpecs(t.workspace, t.config.Sandbox.Mounts)
	if err != nil {
		return "", err
	}
	mounts = append(mounts, extraMounts...)

	resp, err := cli.ContainerCreate(
		execCtx,
		&container.Config{
			Image:        t.config.Sandbox.Image,
			Cmd:          []string{"sh", "-c", command},
			WorkingDir:   containerWorkdir,
			AttachStdout: true,
			AttachStderr: true,
			Tty:          false,
		},
		&container.HostConfig{
			NetworkMode: container.NetworkMode(t.config.Sandbox.NetworkMode),
			Mounts:      mounts,
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("creating sandbox container: %w", err)
	}

	containerID := resp.ID
	if t.config.Sandbox.AutoCleanup {
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanupCancel()
			_ = cli.ContainerRemove(cleanupCtx, containerID, container.RemoveOptions{Force: true, RemoveVolumes: true})
		}()
	}

	if err := cli.ContainerStart(execCtx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("starting sandbox container: %w", err)
	}

	waitCh, errCh := cli.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)
	var waitResp container.WaitResponse

	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("waiting for sandbox container: %w", err)
		}
	case waitResp = <-waitCh:
	case <-execCtx.Done():
		return "", fmt.Errorf("sandbox command timed out after %v", dockerTimeout)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	logsReader, err := cli.ContainerLogs(execCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err == nil {
		defer logsReader.Close()
		_, _ = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, logsReader)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Command: %s\n", command))
	result.WriteString(fmt.Sprintf("Working Directory: %s\n", workdir))
	result.WriteString(fmt.Sprintf("Mode: Docker Sandbox (%s)\n\n", t.config.Sandbox.Image))

	if stdoutBuf.Len() > 0 {
		result.WriteString("STDOUT:\n")
		result.WriteString(stdoutBuf.String())
		result.WriteString("\n")
	}
	if stderrBuf.Len() > 0 {
		result.WriteString("STDERR:\n")
		result.WriteString(stderrBuf.String())
		result.WriteString("\n")
	}
	if waitResp.Error != nil && waitResp.Error.Message != "" {
		result.WriteString(fmt.Sprintf("Error: %s\n", waitResp.Error.Message))
	}
	result.WriteString(fmt.Sprintf("\nExit Code: %d\n", waitResp.StatusCode))
	return result.String(), nil
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
			_ = cmd.Process.Kill()
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
	result.WriteString("Mode: PTY\n\n")

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

func parseMountSpecs(workspace string, specs []string) ([]mount.Mount, error) {
	mounts := make([]mount.Mount, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		parts := strings.Split(spec, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("invalid mount spec %q, expected src:dst[:ro|rw]", spec)
		}
		source := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])
		mode := ""
		if len(parts) == 3 {
			mode = strings.TrimSpace(parts[2])
		}
		if source == "" || target == "" {
			return nil, fmt.Errorf("invalid mount spec %q", spec)
		}
		if !strings.HasPrefix(source, "/") {
			source = filepath.Join(workspace, source)
		}

		readOnly := mode == "ro"
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   source,
			Target:   target,
			ReadOnly: readOnly,
		})
	}
	return mounts, nil
}

type sandboxUnavailableError struct {
	cause error
}

func (e *sandboxUnavailableError) Error() string {
	return e.cause.Error()
}

func (e *sandboxUnavailableError) Unwrap() error {
	return e.cause
}

func (t *ExecTool) disableSandbox(reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sandboxOff = true
	t.sandboxReason = strings.TrimSpace(reason)
	if t.sandboxReason == "" {
		t.sandboxReason = "unknown sandbox initialization error"
	}
}

func (t *ExecTool) isSandboxDisabled() (bool, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sandboxOff, t.sandboxReason
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
