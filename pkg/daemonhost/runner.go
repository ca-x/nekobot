package daemonhost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

const DefaultHeartbeatInterval = 15 * time.Second

type TaskExecutor func(context.Context, *daemonv1.Task) (string, error)

type TaskReporter interface {
	UpdateTaskStatusRemote(req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, error)
}

type TaskFetcher interface {
	FetchAssignedTasksRemote(req *daemonv1.FetchAssignedTasksRequest) (*daemonv1.FetchAssignedTasksResponse, error)
}

type RemoteRegistryClient interface {
	RegisterRemote(req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error)
	HeartbeatRemote(req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error)
	TaskFetcher
	TaskReporter
}

type PollOptions struct {
	MachineName      string
	PollInterval     time.Duration
	TaskLimit        uint32
	Executor         TaskExecutor
	BuildInfo        func(string) (*daemonv1.DaemonInfo, error)
	DaemonURL        string
	BuildInventory   func(string) (*daemonv1.RuntimeInventory, error)
	InventoryHomeDir string
}

type runtimeContext struct {
	runtime   *daemonv1.Runtime
	workspace *daemonv1.Workspace
}

func RegisterAndPoll(ctx context.Context, client RemoteRegistryClient, opts PollOptions) error {
	if client == nil {
		return fmt.Errorf("remote registry client is required")
	}
	buildInfo := opts.BuildInfo
	if buildInfo == nil {
		buildInfo = BuildInfo
	}
	buildInventory := opts.BuildInventory
	if buildInventory == nil {
		buildInventory = BuildInventory
	}
	info, inventory, err := buildDaemonSnapshot(opts.MachineName, opts.InventoryHomeDir, opts.DaemonURL, buildInfo, buildInventory)
	if err != nil {
		return err
	}
	if _, err := client.RegisterRemote(&daemonv1.RegisterMachineRequest{Info: info, Inventory: inventory}); err != nil {
		return err
	}

	interval := opts.PollInterval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	if err := pollOnce(ctx, client, opts, buildInfo, buildInventory); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := pollOnce(ctx, client, opts, buildInfo, buildInventory); err != nil {
				return err
			}
		}
	}
}

func pollOnce(ctx context.Context, client RemoteRegistryClient, opts PollOptions, buildInfo func(string) (*daemonv1.DaemonInfo, error), buildInventory func(string) (*daemonv1.RuntimeInventory, error)) error {
	info, inventory, err := buildDaemonSnapshot(opts.MachineName, opts.InventoryHomeDir, opts.DaemonURL, buildInfo, buildInventory)
	if err != nil {
		return err
	}
	if _, err := client.HeartbeatRemote(&daemonv1.HeartbeatMachineRequest{Info: info, Inventory: inventory}); err != nil {
		return err
	}
	fetchResp, err := client.FetchAssignedTasksRemote(&daemonv1.FetchAssignedTasksRequest{
		MachineId:  info.MachineId,
		RuntimeIds: collectRuntimeIDs(inventory),
		Limit:      normalizedTaskLimit(opts.TaskLimit),
	})
	if err != nil {
		return err
	}
	executor := opts.Executor
	if executor == nil {
		executor = DefaultCLIExecutor(inventory)
	}
	for _, task := range fetchResp.Tasks {
		if err := executeFetchedTask(ctx, client, executor, task); err != nil {
			return err
		}
	}
	return nil
}

func buildDaemonSnapshot(machineName, inventoryHomeDir, daemonURL string, buildInfo func(string) (*daemonv1.DaemonInfo, error), buildInventory func(string) (*daemonv1.RuntimeInventory, error)) (*daemonv1.DaemonInfo, *daemonv1.RuntimeInventory, error) {
	info, err := buildInfo(machineName)
	if err != nil {
		return nil, nil, fmt.Errorf("build daemon info: %w", err)
	}
	if strings.TrimSpace(daemonURL) != "" {
		info.DaemonUrl = strings.TrimSpace(daemonURL)
	}
	inventory, err := buildInventory(inventoryHomeDir)
	if err != nil {
		return nil, nil, fmt.Errorf("build inventory: %w", err)
	}
	normalizeInventoryMachine(info, inventory)
	return info, inventory, nil
}

func executeFetchedTask(ctx context.Context, reporter TaskReporter, executor TaskExecutor, task *daemonv1.Task) error {
	if reporter == nil || task == nil || strings.TrimSpace(task.TaskId) == "" {
		return nil
	}
	if _, err := reporter.UpdateTaskStatusRemote(&daemonv1.UpdateTaskStatusRequest{
		TaskId:    task.TaskId,
		RuntimeId: task.RuntimeId,
		State:     "claimed",
		Summary:   task.Summary,
	}); err != nil {
		return err
	}
	if _, err := reporter.UpdateTaskStatusRemote(&daemonv1.UpdateTaskStatusRequest{
		TaskId:    task.TaskId,
		RuntimeId: task.RuntimeId,
		State:     "running",
		Summary:   task.Summary,
	}); err != nil {
		return err
	}

	resultMessage, execErr := runTaskExecutor(ctx, executor, task)
	if execErr != nil {
		_, reportErr := reporter.UpdateTaskStatusRemote(&daemonv1.UpdateTaskStatusRequest{
			TaskId:        task.TaskId,
			RuntimeId:     task.RuntimeId,
			State:         "failed",
			Summary:       task.Summary,
			Error:         execErr.Error(),
			ResultMessage: strings.TrimSpace(resultMessage),
			BlockedReason: "",
		})
		if reportErr != nil {
			return fmt.Errorf("task failed (%v) and failure report failed: %w", execErr, reportErr)
		}
		return nil
	}
	_, err := reporter.UpdateTaskStatusRemote(&daemonv1.UpdateTaskStatusRequest{
		TaskId:        task.TaskId,
		RuntimeId:     task.RuntimeId,
		State:         "completed",
		Summary:       task.Summary,
		ResultMessage: strings.TrimSpace(resultMessage),
	})
	return err
}

func runTaskExecutor(ctx context.Context, executor TaskExecutor, task *daemonv1.Task) (string, error) {
	if task == nil {
		return "", nil
	}
	if executor == nil {
		return defaultTaskResult(task), nil
	}
	return executor(ctx, task)
}

func defaultTaskResult(task *daemonv1.Task) string {
	if task == nil {
		return ""
	}
	summary := strings.TrimSpace(task.Summary)
	if summary == "" {
		summary = strings.TrimSpace(task.TaskId)
	}
	parts := []string{"Daemon task finished."}
	if summary != "" {
		parts = append(parts, "Summary: "+summary)
	}
	if runtimeID := strings.TrimSpace(task.RuntimeId); runtimeID != "" {
		parts = append(parts, "Runtime: "+runtimeID)
	}
	return strings.Join(parts, "\n")
}

func DefaultCLIExecutor(inventory *daemonv1.RuntimeInventory) TaskExecutor {
	contexts := buildRuntimeContexts(inventory)
	return func(ctx context.Context, task *daemonv1.Task) (string, error) {
		if task == nil {
			return "", nil
		}
		runtimeID := strings.TrimSpace(task.RuntimeId)
		rtCtx, ok := contexts[runtimeID]
		if !ok || rtCtx.runtime == nil {
			return "", fmt.Errorf("runtime %s is not available on this daemon", runtimeID)
		}
		if !rtCtx.runtime.Installed || !rtCtx.runtime.Healthy {
			return "", fmt.Errorf("runtime %s is not installed or healthy", runtimeID)
		}
		prompt := strings.TrimSpace(task.Summary)
		if prompt == "" {
			prompt = strings.TrimSpace(task.TaskId)
		}
		switch strings.ToLower(strings.TrimSpace(rtCtx.runtime.Kind)) {
		case "codex":
			return runCodexTask(ctx, prompt, rtCtx.workspace)
		case "claude":
			return runClaudeTask(ctx, prompt, rtCtx.workspace)
		case "opencode":
			return runOpenCodeTask(ctx, prompt, rtCtx.workspace)
		default:
			return "", fmt.Errorf("runtime kind %s does not support daemon execution yet", rtCtx.runtime.Kind)
		}
	}
}

func buildRuntimeContexts(inventory *daemonv1.RuntimeInventory) map[string]runtimeContext {
	result := map[string]runtimeContext{}
	if inventory == nil {
		return result
	}
	workspaces := map[string]*daemonv1.Workspace{}
	for _, workspace := range inventory.Workspaces {
		if workspace == nil {
			continue
		}
		workspaceID := strings.TrimSpace(workspace.WorkspaceId)
		if workspaceID == "" {
			continue
		}
		workspaces[workspaceID] = workspace
	}
	for _, runtime := range inventory.Runtimes {
		if runtime == nil {
			continue
		}
		runtimeID := strings.TrimSpace(runtime.RuntimeId)
		if runtimeID == "" {
			continue
		}
		result[runtimeID] = runtimeContext{
			runtime:   runtime,
			workspace: workspaces[strings.TrimSpace(runtime.WorkspaceId)],
		}
	}
	return result
}

func normalizeInventoryMachine(info *daemonv1.DaemonInfo, inventory *daemonv1.RuntimeInventory) {
	if info == nil || inventory == nil {
		return
	}
	machineID := strings.TrimSpace(info.MachineId)
	if machineID == "" {
		return
	}
	workspaceIDMap := map[string]string{}
	for _, workspace := range inventory.Workspaces {
		if workspace == nil {
			continue
		}
		oldWorkspaceID := strings.TrimSpace(workspace.WorkspaceId)
		newWorkspaceID := oldWorkspaceID
		if workspace.IsDefault || oldWorkspaceID == "" {
			newWorkspaceID = machineID + ":default"
		} else if parts := strings.Split(oldWorkspaceID, ":"); len(parts) > 1 {
			newWorkspaceID = machineID + ":" + strings.Join(parts[1:], ":")
		}
		workspaceIDMap[oldWorkspaceID] = newWorkspaceID
		workspace.WorkspaceId = newWorkspaceID
		workspace.MachineId = machineID
	}
	for _, runtime := range inventory.Runtimes {
		if runtime == nil {
			continue
		}
		oldWorkspaceID := strings.TrimSpace(runtime.WorkspaceId)
		newWorkspaceID := workspaceIDMap[oldWorkspaceID]
		if newWorkspaceID == "" {
			newWorkspaceID = machineID + ":default"
		}
		runtime.WorkspaceId = newWorkspaceID
		runtime.MachineId = machineID
		kind := strings.TrimSpace(runtime.Kind)
		if kind == "" {
			kind = strings.TrimSpace(runtime.RuntimeId)
		}
		if kind != "" {
			runtime.RuntimeId = newWorkspaceID + ":" + kind
		}
	}
}

func runCodexTask(ctx context.Context, prompt string, workspace *daemonv1.Workspace) (string, error) {
	outputPath, err := os.CreateTemp("", "nekobot-daemon-codex-*.txt")
	if err != nil {
		return "", err
	}
	outputPath.Close()
	defer os.Remove(outputPath.Name())

	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--sandbox", "workspace-write",
		"-C", workspaceDir(workspace),
		"-o", outputPath.Name(),
		prompt,
	}
	cmd := exec.CommandContext(ctx, "codex", args...)
	output, runErr := cmd.CombinedOutput()

	fileOutput, readErr := os.ReadFile(outputPath.Name())
	if readErr == nil && strings.TrimSpace(string(fileOutput)) != "" {
		return strings.TrimSpace(string(fileOutput)), nil
	}
	if extracted := extractCodexMessage(string(output)); extracted != "" {
		return extracted, nil
	}
	if runErr != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("codex exec: %w", runErr)
	}
	if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
		return trimmed, nil
	}
	return "", fmt.Errorf("codex exec returned no output")
}

func runClaudeTask(ctx context.Context, prompt string, workspace *daemonv1.Workspace) (string, error) {
	args := []string{
		"--bare",
		"-p",
		"--permission-mode", "bypassPermissions",
		prompt,
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = workspaceDir(workspace)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed != "" {
			return trimmed, nil
		}
		return "", fmt.Errorf("claude print: %w", err)
	}
	if trimmed == "" {
		return "", fmt.Errorf("claude print returned no output")
	}
	return trimmed, nil
}

func runOpenCodeTask(ctx context.Context, prompt string, workspace *daemonv1.Workspace) (string, error) {
	tempHome, err := os.MkdirTemp("", "nekobot-daemon-opencode-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempHome)

	args := []string{
		"--pure",
		"run",
		"--format", "json",
		"--dir", workspaceDir(workspace),
		prompt,
	}
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Env = append(os.Environ(),
		"HOME="+tempHome,
		"XDG_CONFIG_HOME="+tempHome,
	)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if extracted := extractOpenCodeMessage(trimmed); extracted != "" {
		return extracted, nil
	}
	if err != nil {
		if trimmed != "" {
			return trimmed, fmt.Errorf("opencode run: %w", err)
		}
		return "", fmt.Errorf("opencode run: %w", err)
	}
	if trimmed == "" {
		return "", fmt.Errorf("opencode run returned no output")
	}
	return trimmed, nil
}

func extractOpenCodeMessage(output string) string {
	if strings.TrimSpace(output) == "" {
		return ""
	}
	type textPart struct {
		Text string `json:"text"`
	}
	type event struct {
		Type string   `json:"type"`
		Part textPart `json:"part"`
	}
	lines := strings.Split(output, "\n")
	var last string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var item event
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if item.Type == "text" && strings.TrimSpace(item.Part.Text) != "" {
			last = strings.TrimSpace(item.Part.Text)
		}
	}
	return last
}

func workspaceDir(workspace *daemonv1.Workspace) string {
	if workspace == nil {
		if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
			return cwd
		}
		return "."
	}
	if path := strings.TrimSpace(workspace.Path); path != "" {
		return filepath.Clean(path)
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return cwd
	}
	return "."
}

func extractCodexMessage(output string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	inAssistantBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "codex" {
			inAssistantBlock = true
			filtered = filtered[:0]
			continue
		}
		if trimmed == "user" {
			inAssistantBlock = false
			filtered = filtered[:0]
			continue
		}
		if trimmed == "--------" ||
			strings.HasPrefix(trimmed, "OpenAI Codex") ||
			strings.HasPrefix(trimmed, "workdir:") ||
			strings.HasPrefix(trimmed, "model:") ||
			strings.HasPrefix(trimmed, "provider:") ||
			strings.HasPrefix(trimmed, "approval:") ||
			strings.HasPrefix(trimmed, "sandbox:") ||
			strings.HasPrefix(trimmed, "reasoning effort:") ||
			strings.HasPrefix(trimmed, "reasoning summaries:") ||
			strings.HasPrefix(trimmed, "session id:") ||
			strings.HasPrefix(trimmed, "hook:") ||
			strings.HasPrefix(trimmed, "warning:") ||
			strings.HasPrefix(trimmed, "Reading additional input from stdin") {
			continue
		}
		if !inAssistantBlock {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		return ""
	}
	return filtered[len(filtered)-1]
}

func normalizedTaskLimit(limit uint32) uint32 {
	if limit == 0 {
		return 10
	}
	return limit
}

func collectRuntimeIDs(inventory *daemonv1.RuntimeInventory) []string {
	if inventory == nil {
		return nil
	}
	ids := make([]string, 0, len(inventory.Runtimes))
	for _, item := range inventory.Runtimes {
		if item == nil {
			continue
		}
		if !item.Installed || !item.Healthy {
			continue
		}
		runtimeID := strings.TrimSpace(item.RuntimeId)
		if runtimeID == "" {
			continue
		}
		ids = append(ids, runtimeID)
	}
	return ids
}
