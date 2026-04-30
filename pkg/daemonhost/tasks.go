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
	assigneeID := metadataString(item.Metadata, "assignee_id")
	if assigneeID == "" {
		assigneeID = item.RuntimeID
	}
	return &daemonv1.Task{
		TaskId:               item.ID,
		Summary:              item.Summary,
		State:                string(item.State),
		RuntimeId:            item.RuntimeID,
		ThreadId:             item.SessionID,
		WorkspaceId:          metadataString(item.Metadata, "workspace_id"),
		ComputerId:           metadataString(item.Metadata, "computer_id"),
		CreatedByUserId:      metadataString(item.Metadata, "created_by_user_id"),
		BlockedReason:        item.PendingAction,
		Target:               metadataString(item.Metadata, "target"),
		AssigneeId:           assigneeID,
		CurrentRunId:         metadataString(item.Metadata, "current_run_id"),
		RootTaskId:           metadataString(item.Metadata, "root_task_id"),
		ParentTaskId:         metadataString(item.Metadata, "parent_task_id"),
		Source:               metadataString(item.Metadata, "source"),
		CreatedByAgentId:     metadataString(item.Metadata, "created_by_agent_id"),
		ServerRuleId:         metadataString(item.Metadata, "server_rule_id"),
		SplitProposalId:      metadataString(item.Metadata, "split_proposal_id"),
		GraphVersion:         metadataInt64(item.Metadata, "graph_version"),
		SubtaskIds:           metadataStringSlice(item.Metadata, "subtask_ids"),
		DependsOnTaskIds:     metadataStringSlice(item.Metadata, "depends_on_task_ids"),
		BlockedByTaskIds:     metadataStringSlice(item.Metadata, "blocked_by_task_ids"),
		RequiredCapabilities: metadataStringSlice(item.Metadata, "required_capabilities"),
		BoardColumn:          TaskBoardColumnForState(string(item.State), metadataString(item.Metadata, "board_column")),
	}
}

func TaskBoardColumnForState(state, explicit string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	switch strings.TrimSpace(state) {
	case string(tasks.StatePending):
		return "TODO"
	case string(tasks.StateClaimed), string(tasks.StateRunning), string(tasks.StateRequiresAction):
		return "IN PROCESS"
	case "in_review":
		return "IN REVIEW"
	case string(tasks.StateCompleted), string(tasks.StateFailed), string(tasks.StateCanceled):
		return "Done"
	default:
		return "TODO"
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

func metadataInt64(values map[string]any, key string) int64 {
	if len(values) == 0 {
		return 0
	}
	switch v := values[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func metadataStringSlice(values map[string]any, key string) []string {
	if len(values) == 0 {
		return nil
	}
	switch v := values[key].(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}
