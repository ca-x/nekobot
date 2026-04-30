package daemonhost

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func TestGRPCServiceFetchAssignedRuns(t *testing.T) {
	svc := tasks.NewService(nil)
	_, err := svc.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "demo", RuntimeID: "runtime-a", Metadata: map[string]any{"computer_id": "machine-a"}})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	grpcSvc := NewGRPCService(nil, svc, nil, nil)
	resp, err := grpcSvc.FetchAssignedRuns(context.Background(), &daemonv1.FetchAssignedRunsRequest{ComputerId: "machine-a", AgentIds: []string{"runtime-a"}, Limit: 10})
	if err != nil {
		t.Fatalf("FetchAssignedRuns failed: %v", err)
	}
	if len(resp.Runs) != 1 || resp.Runs[0].TaskId != "task-1" {
		t.Fatalf("unexpected tasks: %+v", resp.Runs)
	}
}

func TestGRPCServiceUpdateRunStatusAppendsSession(t *testing.T) {
	svc := tasks.NewService(nil)
	_, err := svc.Enqueue(tasks.Task{ID: "task-1", Type: tasks.TypeRemoteAgent, Summary: "demo", RuntimeID: "runtime-a"})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	appended := false
	grpcSvc := NewGRPCService(nil, svc, nil, func(ctx context.Context, task tasks.Task, req *daemonv1.UpdateRunStatusRequest) error {
		appended = task.ID == "task-1" && req.GetState() == string(tasks.StateClaimed)
		return nil
	})
	_, err = grpcSvc.UpdateRunStatus(context.Background(), &daemonv1.UpdateRunStatusRequest{RunId: "task-1", AgentId: "runtime-a", State: string(tasks.StateClaimed)})
	if err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
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

func TestGRPCServiceCollaborationRPCsAreUnimplementedWithoutBackend(t *testing.T) {
	grpcSvc := NewGRPCService(nil, nil, nil, nil)
	_, err := grpcSvc.GetServerInfo(context.Background(), &daemonv1.GetServerInfoRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected unimplemented error, got %v", err)
	}
}
