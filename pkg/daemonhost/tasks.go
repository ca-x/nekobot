package daemonhost

import (
	"fmt"
	"strings"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func BuildAssignedRuns(taskService *tasks.Service, computerID string, agentIDs []string, limit int) *daemonv1.FetchAssignedRunsResponse {
	resp := &daemonv1.FetchAssignedRunsResponse{}
	if taskService == nil {
		return resp
	}
	items := taskService.List()
	agentSet := map[string]struct{}{}
	for _, id := range agentIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			agentSet[trimmed] = struct{}{}
		}
	}
	for _, item := range items {
		if limit > 0 && len(resp.Runs) >= limit {
			break
		}
		if computerID != "" && metadataString(item.Metadata, "computer_id") != strings.TrimSpace(computerID) {
			continue
		}
		if len(agentSet) > 0 {
			if _, ok := agentSet[strings.TrimSpace(item.RuntimeID)]; !ok {
				continue
			}
		}
		if item.State != tasks.StatePending && item.State != tasks.StateClaimed && item.State != tasks.StateRunning && item.State != tasks.StateRequiresAction {
			continue
		}
		resp.Runs = append(resp.Runs, taskToRunProto(item))
	}
	return resp
}

func ApplyRunStatusUpdate(taskService *tasks.Service, req *daemonv1.UpdateRunStatusRequest) (*daemonv1.UpdateRunStatusResponse, tasks.Task, error) {
	if taskService == nil {
		return nil, tasks.Task{}, fmt.Errorf("task service unavailable")
	}
	if req == nil || strings.TrimSpace(req.RunId) == "" {
		return nil, tasks.Task{}, fmt.Errorf("run_id is required")
	}
	taskID := strings.TrimSpace(req.RunId)
	switch strings.TrimSpace(req.State) {
	case string(tasks.StateClaimed):
		task, err := taskService.Claim(taskID, req.AgentId)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateRunning):
		task, err := taskService.Start(taskID)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateCompleted):
		task, err := taskService.Complete(taskID)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateFailed):
		task, err := taskService.Fail(taskID, req.Error)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateCanceled):
		task, err := taskService.Cancel(taskID)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
	case string(tasks.StateRequiresAction):
		pending := strings.TrimSpace(req.BlockedReason)
		if pending == "" {
			pending = strings.TrimSpace(req.Summary)
		}
		task, err := taskService.RequireAction(taskID, pending)
		return &daemonv1.UpdateRunStatusResponse{Accepted: err == nil}, task, err
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
		ComputerId:      metadataString(item.Metadata, "computer_id"),
		CreatedByUserId: metadataString(item.Metadata, "created_by_user_id"),
		BlockedReason:   item.PendingAction,
	}
}

func taskToRunProto(item tasks.Task) *daemonv1.Run {
	return &daemonv1.Run{
		RunId:             item.ID,
		TaskId:            item.ID,
		Target:            metadataString(item.Metadata, "target"),
		AgentId:           item.RuntimeID,
		ComputerId:        metadataString(item.Metadata, "computer_id"),
		RuntimeProfileId:  item.RuntimeID,
		Status:            string(item.State),
		State:             string(item.State),
		Summary:           item.Summary,
		StartedTimeUnix:   item.StartedAt.Unix(),
		UpdatedTimeUnix:   item.CreatedAt.Unix(),
		CompletedTimeUnix: item.CompletedAt.Unix(),
	}
}

func metadataString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
