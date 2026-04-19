package daemonhost

import (
	"context"
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func TestGRPCServiceFetchAssignedTasks(t *testing.T) {
	svc := tasks.NewService(nil)
	_, err := svc.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "demo", RuntimeID: "runtime-a", Metadata: map[string]any{"machine_id": "machine-a"}})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	grpcSvc := NewGRPCService(nil, svc, nil, nil)
	resp, err := grpcSvc.FetchAssignedTasks(context.Background(), &daemonv1.FetchAssignedTasksRequest{MachineId: "machine-a", RuntimeIds: []string{"runtime-a"}, Limit: 10})
	if err != nil {
		t.Fatalf("FetchAssignedTasks failed: %v", err)
	}
	if len(resp.Tasks) != 1 || resp.Tasks[0].TaskId != "task-1" {
		t.Fatalf("unexpected tasks: %+v", resp.Tasks)
	}
}

func TestGRPCServiceUpdateTaskStatusAppendsSession(t *testing.T) {
	svc := tasks.NewService(nil)
	_, err := svc.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "demo", RuntimeID: "runtime-a"})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	appended := false
	grpcSvc := NewGRPCService(nil, svc, nil, func(ctx context.Context, task tasks.Task, req *daemonv1.UpdateTaskStatusRequest) error {
		appended = task.ID == "task-1" && req.GetState() == string(tasks.StateClaimed)
		return nil
	})
	_, err = grpcSvc.UpdateTaskStatus(context.Background(), &daemonv1.UpdateTaskStatusRequest{TaskId: "task-1", RuntimeId: "runtime-a", State: string(tasks.StateClaimed)})
	if err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
	if !appended {
		t.Fatalf("expected append callback to run")
	}
}

func TestGRPCServiceWorkspaceRPCsAreUnimplementedWithoutLoader(t *testing.T) {
	grpcSvc := NewGRPCService(nil, nil, nil, nil)
	_, err := grpcSvc.ListWorkspaceTree(context.Background(), &daemonv1.ListWorkspaceTreeRequest{WorkspaceId: "w"})
	if err == nil {
		t.Fatalf("expected unimplemented error")
	}
}
