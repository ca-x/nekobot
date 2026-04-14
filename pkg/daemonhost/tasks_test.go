package daemonhost

import (
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func TestBuildAssignedTasksFiltersByMachineAndRuntime(t *testing.T) {
	service := tasks.NewService(nil)
	_, err := service.Enqueue(tasks.Task{
		ID:        "task-1",
		Type:      tasks.TypeRemoteAgent,
		Summary:   "daemon work",
		RuntimeID: "runtime-a",
		Metadata: map[string]any{
			"machine_id": "machine-a",
		},
	})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	resp := BuildAssignedTasks(service, "machine-a", []string{"runtime-a"}, 10)
	if len(resp.Tasks) != 1 || resp.Tasks[0].TaskId != "task-1" {
		t.Fatalf("unexpected assigned tasks: %+v", resp.Tasks)
	}
}

func TestApplyTaskStatusUpdateTransitionsTask(t *testing.T) {
	service := tasks.NewService(nil)
	_, err := service.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "daemon work"})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if _, _, err := ApplyTaskStatusUpdate(service, &daemonv1.UpdateTaskStatusRequest{TaskId: "task-1", RuntimeId: "runtime-a", State: string(tasks.StateClaimed)}); err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if _, _, err := ApplyTaskStatusUpdate(service, &daemonv1.UpdateTaskStatusRequest{TaskId: "task-1", RuntimeId: "runtime-a", State: string(tasks.StateRunning)}); err != nil {
		t.Fatalf("start task: %v", err)
	}
	if _, _, err := ApplyTaskStatusUpdate(service, &daemonv1.UpdateTaskStatusRequest{TaskId: "task-1", RuntimeId: "runtime-a", State: string(tasks.StateCompleted)}); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	items := service.List()
	if len(items) != 1 || items[0].State != tasks.StateCompleted {
		t.Fatalf("unexpected task state after updates: %+v", items)
	}
}
