package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/process"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
)

func TestToolSessionToolSpawnPersistsResumeMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	preparer := &capturePreparer{}
	pm := process.NewManager(log)
	pm.SetPreparer(preparer)
	tool := NewToolSessionTool(pm, mgr, cfg.WorkspacePath())

	ctx := context.WithValue(context.Background(), execenv.MetadataRuntimeID, "runtime-agent")
	ctx = context.WithValue(ctx, execenv.MetadataTaskID, "task-agent")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "spawn",
		"tool":    "codex",
		"command": "sleep 5",
	})
	if err != nil {
		t.Fatalf("spawn tool session: %v", err)
	}
	if !strings.Contains(result, "Tool session created successfully") {
		t.Fatalf("unexpected result: %s", result)
	}

	sessionID := preparer.last.SessionID
	if sessionID == "" {
		t.Fatal("expected session id to be captured")
	}
	if preparer.last.RuntimeID != "runtime-agent" {
		t.Fatalf("expected runtime id in start spec, got %q", preparer.last.RuntimeID)
	}
	if preparer.last.TaskID != "task-agent" {
		t.Fatalf("expected task id in start spec, got %q", preparer.last.TaskID)
	}

	sess, err := mgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if got, _ := sess.Metadata[execenv.MetadataRuntimeID].(string); got != "runtime-agent" {
		t.Fatalf("expected runtime id in metadata, got %q", got)
	}
	if got, _ := sess.Metadata[execenv.MetadataTaskID].(string); got != "task-agent" {
		t.Fatalf("expected task id in metadata, got %q", got)
	}
	if isTmuxAvailable() {
		if got, _ := sess.Metadata["runtime_transport"].(string); got != "tmux" {
			t.Fatalf("expected runtime transport tmux in metadata, got %q", got)
		}
		if got, _ := sess.Metadata["tmux_session"].(string); strings.TrimSpace(got) == "" {
			t.Fatal("expected tmux session metadata to be populated")
		}
		if got, _ := sess.Metadata["launch_cmd"].(string); !strings.Contains(got, "tmux new-session") {
			t.Fatalf("expected launch_cmd to capture tmux wrapper, got %q", got)
		}
	}

	if err := pm.Reset(sessionID); err != nil {
		t.Fatalf("reset tool session process: %v", err)
	}
}

func TestToolSessionToolSpawnCreatesManagedTaskWhenTaskIDMissing(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	pm := process.NewManager(log)
	pm.SetTaskService(taskSvc)
	tool := NewToolSessionTool(pm, mgr, cfg.WorkspacePath())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "spawn",
		"tool":    "codex",
		"command": "/bin/sh -lc 'printf ok'",
	})
	if err != nil {
		t.Fatalf("spawn tool session: %v", err)
	}
	if !strings.Contains(result, "Tool session created successfully") {
		t.Fatalf("unexpected result: %s", result)
	}

	task := waitForBackgroundTaskTerminal(t, store, 3*time.Second)
	if task.ID == "" {
		t.Fatal("expected generated task id")
	}
	if task.ID != task.SessionID {
		t.Fatalf("expected generated task id to match session id, got task=%q session=%q", task.ID, task.SessionID)
	}
	if task.Type != tasks.TypeRuntimeWorker {
		t.Fatalf("expected runtime worker type, got %q", task.Type)
	}

	sess, err := mgr.GetSession(context.Background(), task.SessionID)
	if err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if got, _ := sess.Metadata[execenv.MetadataTaskID].(string); got != task.ID {
		t.Fatalf("expected task id %q in session metadata, got %q", task.ID, got)
	}
}

func waitForBackgroundTaskTerminal(t *testing.T, store *tasks.Store, timeout time.Duration) tasks.Task {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		all := store.List()
		if len(all) == 1 && tasks.IsFinal(all[0].State) {
			return all[0]
		}
		time.Sleep(10 * time.Millisecond)
	}

	var states []string
	for _, task := range store.List() {
		states = append(states, fmt.Sprintf("%s=%s", task.ID, task.State))
	}
	t.Fatalf("background task did not reach terminal state before timeout; saw %v", states)
	return tasks.Task{}
}
