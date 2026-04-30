package daemonhost

import (
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func TestBuildAssignedRunsFiltersByComputerAndAgent(t *testing.T) {
	service := tasks.NewService(nil)
	_, err := service.Enqueue(tasks.Task{
		ID:        "task-1",
		Type:      tasks.TypeRemoteAgent,
		Summary:   "daemon work",
		RuntimeID: "runtime-a",
		Metadata: map[string]any{
			"computer_id": "machine-a",
		},
	})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	resp := BuildAssignedRuns(service, "machine-a", []string{"runtime-a"}, 10)
	if len(resp.Runs) != 1 || resp.Runs[0].RunId != "task-1" {
		t.Fatalf("unexpected assigned runs: %+v", resp.Runs)
	}
}

func TestApplyRunStatusUpdateTransitionsTask(t *testing.T) {
	service := tasks.NewService(nil)
	_, err := service.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "daemon work"})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if _, _, err := ApplyRunStatusUpdate(service, &daemonv1.UpdateRunStatusRequest{RunId: "task-1", AgentId: "runtime-a", State: string(tasks.StateClaimed)}); err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if _, _, err := ApplyRunStatusUpdate(service, &daemonv1.UpdateRunStatusRequest{RunId: "task-1", AgentId: "runtime-a", State: string(tasks.StateRunning)}); err != nil {
		t.Fatalf("start task: %v", err)
	}
	if _, _, err := ApplyRunStatusUpdate(service, &daemonv1.UpdateRunStatusRequest{RunId: "task-1", AgentId: "runtime-a", State: string(tasks.StateCompleted)}); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	items := service.List()
	if len(items) != 1 || items[0].State != tasks.StateCompleted {
		t.Fatalf("unexpected task state after updates: %+v", items)
	}
}
