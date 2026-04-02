package subagent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

type stubAgent struct {
	reply string
	err   error
}

func (s *stubAgent) Chat(ctx context.Context, message string) (string, error) {
	return s.reply, s.err
}

func newTestSubagentLogger(t *testing.T) *logger.Logger {
	t.Helper()

	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}

func TestSubagentManagerUpdatesTaskLifecycleServiceOnCompletion(t *testing.T) {
	store := tasks.NewStore()
	svc := tasks.NewService(store)
	manager := NewSubagentManager(newTestSubagentLogger(t), &stubAgent{reply: "done"}, 1)
	manager.SetTaskService(svc)
	defer manager.Stop()

	taskID, err := manager.Spawn(context.Background(), "collect findings", "research", "websocket", "webui-chat:alice")
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		task, getErr := manager.GetTask(taskID)
		if getErr != nil {
			t.Fatalf("get task failed: %v", getErr)
		}
		if task.Status == tasks.StateCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not complete before timeout, last status=%s", task.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshots := store.List()
	if len(snapshots) != 1 {
		t.Fatalf("expected one managed task snapshot, got %d", len(snapshots))
	}
	got := snapshots[0]
	if got.ID != taskID {
		t.Fatalf("expected task id %q, got %q", taskID, got.ID)
	}
	if got.State != tasks.StateCompleted {
		t.Fatalf("expected completed task state, got %s", got.State)
	}
	if got.RuntimeID != "subagent" {
		t.Fatalf("expected runtime id subagent, got %q", got.RuntimeID)
	}
	if got.StartedAt.IsZero() {
		t.Fatalf("expected started_at to be recorded")
	}
	if got.CompletedAt.IsZero() {
		t.Fatalf("expected completed_at to be recorded")
	}
}

func TestSubagentManagerUpdatesTaskLifecycleServiceOnFailure(t *testing.T) {
	store := tasks.NewStore()
	svc := tasks.NewService(store)
	manager := NewSubagentManager(newTestSubagentLogger(t), &stubAgent{err: errors.New("boom")}, 1)
	manager.SetTaskService(svc)
	defer manager.Stop()

	taskID, err := manager.Spawn(context.Background(), "collect findings", "research", "websocket", "webui-chat:alice")
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		task, getErr := manager.GetTask(taskID)
		if getErr != nil {
			t.Fatalf("get task failed: %v", getErr)
		}
		if task.Status == tasks.StateFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not fail before timeout, last status=%s", task.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshots := store.List()
	if len(snapshots) != 1 {
		t.Fatalf("expected one managed task snapshot, got %d", len(snapshots))
	}
	got := snapshots[0]
	if got.ID != taskID {
		t.Fatalf("expected task id %q, got %q", taskID, got.ID)
	}
	if got.State != tasks.StateFailed {
		t.Fatalf("expected failed task state, got %s", got.State)
	}
	if got.LastError != "boom" {
		t.Fatalf("expected last error boom, got %q", got.LastError)
	}
}

func TestSubagentManagerFallsBackWhenTaskServiceRejectsUpdates(t *testing.T) {
	manager := NewSubagentManager(newTestSubagentLogger(t), &stubAgent{reply: "done"}, 1)
	manager.SetTaskService(tasks.NewService(nil))
	defer manager.Stop()

	if _, err := manager.Spawn(context.Background(), "collect findings", "research", "websocket", "webui-chat:alice"); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
}

type blockingAgent struct {
	started chan struct{}
	wait    chan struct{}
	once    sync.Once
}

func (b *blockingAgent) Chat(ctx context.Context, message string) (string, error) {
	b.once.Do(func() { close(b.started) })
	select {
	case <-b.wait:
		return "done", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
