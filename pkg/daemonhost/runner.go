package daemonhost

import (
	"context"
	"fmt"
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
	BuildInventory   func(string) (*daemonv1.RuntimeInventory, error)
	InventoryHomeDir string
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
	info, inventory, err := buildDaemonSnapshot(opts.MachineName, opts.InventoryHomeDir, buildInfo, buildInventory)
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
	info, inventory, err := buildDaemonSnapshot(opts.MachineName, opts.InventoryHomeDir, buildInfo, buildInventory)
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
	for _, task := range fetchResp.Tasks {
		if err := executeFetchedTask(ctx, client, opts.Executor, task); err != nil {
			return err
		}
	}
	return nil
}

func buildDaemonSnapshot(machineName, inventoryHomeDir string, buildInfo func(string) (*daemonv1.DaemonInfo, error), buildInventory func(string) (*daemonv1.RuntimeInventory, error)) (*daemonv1.DaemonInfo, *daemonv1.RuntimeInventory, error) {
	info, err := buildInfo(machineName)
	if err != nil {
		return nil, nil, fmt.Errorf("build daemon info: %w", err)
	}
	inventory, err := buildInventory(inventoryHomeDir)
	if err != nil {
		return nil, nil, fmt.Errorf("build inventory: %w", err)
	}
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
		runtimeID := strings.TrimSpace(item.RuntimeId)
		if runtimeID == "" {
			continue
		}
		ids = append(ids, runtimeID)
	}
	return ids
}
