package daemonhost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

type stubRemoteRegistryClient struct {
	registered int
	heartbeats int
	fetches    int
	updates    []*daemonv1.UpdateTaskStatusRequest
	tasks      []*daemonv1.Task
	fetchCh    chan struct{}
}

func (s *stubRemoteRegistryClient) RegisterRemote(req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error) {
	s.registered++
	return &daemonv1.RegisterMachineResponse{Accepted: true}, nil
}

func (s *stubRemoteRegistryClient) HeartbeatRemote(req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error) {
	s.heartbeats++
	return &daemonv1.HeartbeatMachineResponse{Accepted: true}, nil
}

func (s *stubRemoteRegistryClient) FetchAssignedTasksRemote(req *daemonv1.FetchAssignedTasksRequest) (*daemonv1.FetchAssignedTasksResponse, error) {
	s.fetches++
	if s.fetchCh != nil {
		select {
		case s.fetchCh <- struct{}{}:
		default:
		}
	}
	return &daemonv1.FetchAssignedTasksResponse{Tasks: s.tasks}, nil
}

func (s *stubRemoteRegistryClient) UpdateTaskStatusRemote(req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, error) {
	cloned := *req
	s.updates = append(s.updates, &cloned)
	return &daemonv1.UpdateTaskStatusResponse{Accepted: true}, nil
}

func TestRegisterAndPollProcessesFetchedTasks(t *testing.T) {
	client := &stubRemoteRegistryClient{
		tasks:   []*daemonv1.Task{{TaskId: "task-1", RuntimeId: "runtime-a", Summary: "run tests"}},
		fetchCh: make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-client.fetchCh
		cancel()
	}()

	err := RegisterAndPoll(ctx, client, PollOptions{
		MachineName:  "machine-a",
		PollInterval: time.Millisecond,
		BuildInfo: func(string) (*daemonv1.DaemonInfo, error) {
			return &daemonv1.DaemonInfo{MachineId: "machine-a", MachineName: "machine-a"}, nil
		},
		BuildInventory: func(string) (*daemonv1.RuntimeInventory, error) {
			return &daemonv1.RuntimeInventory{Runtimes: []*daemonv1.Runtime{{RuntimeId: "runtime-a"}}}, nil
		},
		Executor: func(ctx context.Context, task *daemonv1.Task) (string, error) {
			return "done:" + task.TaskId, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterAndPoll failed: %v", err)
	}
	if client.registered != 1 {
		t.Fatalf("expected register once, got %d", client.registered)
	}
	if client.heartbeats == 0 || client.fetches == 0 {
		t.Fatalf("expected heartbeat and fetch, got heartbeats=%d fetches=%d", client.heartbeats, client.fetches)
	}
	if len(client.updates) != 3 {
		t.Fatalf("expected 3 task updates, got %d", len(client.updates))
	}
	if client.updates[0].State != "claimed" || client.updates[1].State != "running" || client.updates[2].State != "completed" {
		t.Fatalf("unexpected update flow: %+v", client.updates)
	}
	if client.updates[2].ResultMessage != "done:task-1" {
		t.Fatalf("unexpected completion result message: %+v", client.updates[2])
	}
}

func TestExecuteFetchedTaskReportsFailure(t *testing.T) {
	client := &stubRemoteRegistryClient{}
	err := executeFetchedTask(context.Background(), client, func(context.Context, *daemonv1.Task) (string, error) {
		return "partial output", errors.New("boom")
	}, &daemonv1.Task{TaskId: "task-1", RuntimeId: "runtime-a", Summary: "run tests"})
	if err != nil {
		t.Fatalf("executeFetchedTask failed: %v", err)
	}
	if len(client.updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(client.updates))
	}
	if client.updates[2].State != "failed" {
		t.Fatalf("expected failed update, got %+v", client.updates[2])
	}
	if client.updates[2].Error != "boom" || client.updates[2].ResultMessage != "partial output" {
		t.Fatalf("unexpected failure payload: %+v", client.updates[2])
	}
}

func TestCollectRuntimeIDsFiltersUnavailableRuntimes(t *testing.T) {
	inventory := &daemonv1.RuntimeInventory{
		Runtimes: []*daemonv1.Runtime{
			{RuntimeId: "runtime-a", Installed: true, Healthy: true},
			{RuntimeId: "runtime-b", Installed: false, Healthy: true},
			{RuntimeId: "runtime-c", Installed: true, Healthy: false},
		},
	}
	ids := collectRuntimeIDs(inventory)
	if len(ids) != 1 || ids[0] != "runtime-a" {
		t.Fatalf("unexpected runtime ids: %+v", ids)
	}
}

func TestDefaultCLIExecutorRejectsUnavailableRuntime(t *testing.T) {
	executor := DefaultCLIExecutor(&daemonv1.RuntimeInventory{
		Runtimes: []*daemonv1.Runtime{
			{RuntimeId: "runtime-a", Kind: "codex", Installed: false, Healthy: true},
		},
	})
	_, err := executor(context.Background(), &daemonv1.Task{TaskId: "task-1", RuntimeId: "runtime-a", Summary: "hello"})
	if err == nil {
		t.Fatalf("expected unavailable runtime error")
	}
}

func TestRunClaudeTaskUsesWorkspace(t *testing.T) {
	workspace := t.TempDir()
	binDir := t.TempDir()
	logFile := filepath.Join(workspace, "claude-call.txt")
	scriptPath := filepath.Join(binDir, "claude")
	script := "#!/usr/bin/env bash\npwd > " + logFile + "\nprintf 'daemon-ok\\n'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := runClaudeTask(context.Background(), "hello", &daemonv1.Workspace{Path: workspace})
	if err != nil {
		t.Fatalf("runClaudeTask failed: %v", err)
	}
	if result != "daemon-ok" {
		t.Fatalf("unexpected claude result: %q", result)
	}
	pwdBytes, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read fake claude log: %v", err)
	}
	if got := string(pwdBytes); got == "" || filepath.Clean(string(pwdBytes[:len(pwdBytes)-1])) != workspace {
		t.Fatalf("expected workspace cwd, got %q", got)
	}
}
