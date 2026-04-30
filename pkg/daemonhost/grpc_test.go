package daemonhost

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/runs"
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

func TestGRPCServiceRunRPCsUseRunManager(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open ent client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	runMgr, err := runs.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new run manager: %v", err)
	}
	created, err := runMgr.CreateRun(context.Background(), runs.RunRecord{
		TaskID:  "task-1",
		Target:  "#websocket:run-thread",
		AgentID: "runtime-a",
		Status:  "queued",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	grpcSvc := NewGRPCService(nil, nil, nil, nil).WithRunManager(runMgr)
	if _, err := grpcSvc.UpdateRunStatus(context.Background(), &daemonv1.UpdateRunStatusRequest{
		RunId:   created.ID,
		AgentId: "runtime-a",
		State:   "running",
		Summary: "started",
	}); err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}
	if _, err := grpcSvc.AppendRunStep(context.Background(), &daemonv1.AppendRunStepRequest{
		Step: &daemonv1.RunStep{RunId: created.ID, Kind: "test", Status: "ok", Summary: "passed"},
	}); err != nil {
		t.Fatalf("AppendRunStep failed: %v", err)
	}
	got, err := grpcSvc.GetRun(context.Background(), &daemonv1.GetRunRequest{RunId: created.ID})
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if got.Run.GetStatus() != "running" || len(got.Steps) != 1 {
		t.Fatalf("unexpected run response: run=%+v steps=%+v", got.Run, got.Steps)
	}
	list, err := grpcSvc.ListRuns(context.Background(), &daemonv1.ListRunsRequest{AgentId: "runtime-a", Limit: 10})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(list.Runs) != 1 || list.Runs[0].GetRunId() != created.ID {
		t.Fatalf("unexpected runs: %+v", list.Runs)
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
