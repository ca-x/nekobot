package daemonhost

import (
	"fmt"
	"strings"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func BuildAssignedTasks(taskService *tasks.Service, machineID string, runtimeIDs []string, limit int) *daemonv1.FetchAssignedTasksResponse {
	resp := &daemonv1.FetchAssignedTasksResponse{}
	if taskService == nil {
		return resp
	}
	items := taskService.List()
	runtimeSet := map[string]struct{}{}
	for _, id := range runtimeIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			runtimeSet[trimmed] = struct{}{}
		}
	}
	for _, item := range items {
		if limit > 0 && len(resp.Tasks) >= limit {
			break
		}
		if machineID != "" && metadataString(item.Metadata, "machine_id") != strings.TrimSpace(machineID) {
			continue
		}
		if len(runtimeSet) > 0 {
			if _, ok := runtimeSet[strings.TrimSpace(item.RuntimeID)]; !ok {
				continue
			}
		}
		if item.State != tasks.StatePending && item.State != tasks.StateClaimed && item.State != tasks.StateRunning && item.State != tasks.StateRequiresAction {
			continue
		}
		resp.Tasks = append(resp.Tasks, taskToProto(item))
	}
	return resp
}

func ApplyTaskStatusUpdate(taskService *tasks.Service, req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, tasks.Task, error) {
	if taskService == nil {
		return nil, tasks.Task{}, fmt.Errorf("task service unavailable")
	}
	if req == nil || strings.TrimSpace(req.TaskId) == "" {
		return nil, tasks.Task{}, fmt.Errorf("task_id is required")
	}
	switch strings.TrimSpace(req.State) {
	case string(tasks.StateClaimed):
		task, err := taskService.Claim(req.TaskId, req.RuntimeId)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateRunning):
		task, err := taskService.Start(req.TaskId)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateCompleted):
		task, err := taskService.Complete(req.TaskId)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateFailed):
		task, err := taskService.Fail(req.TaskId, req.Error)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateCanceled):
		task, err := taskService.Cancel(req.TaskId)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateRequiresAction):
		pending := strings.TrimSpace(req.BlockedReason)
		if pending == "" {
			pending = strings.TrimSpace(req.Summary)
		}
		task, err := taskService.RequireAction(req.TaskId, pending)
		return &daemonv1.UpdateTaskStatusResponse{Accepted: err == nil}, task, err
	default:
		return nil, tasks.Task{}, fmt.Errorf("unsupported task state update: %s", req.State)
	}
}

func taskToProto(item tasks.Task) *daemonv1.Task {
	return &daemonv1.Task{
		TaskId:          item.ID,
		Summary:         item.Summary,
		State:           string(item.State),
		RuntimeId:       item.RuntimeID,
		ThreadId:        item.SessionID,
		WorkspaceId:     metadataString(item.Metadata, "workspace_id"),
		MachineId:       metadataString(item.Metadata, "machine_id"),
		CreatedByUserId: metadataString(item.Metadata, "created_by_user_id"),
		BlockedReason:   item.PendingAction,
	}
}

func metadataString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
