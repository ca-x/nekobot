package process

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

type stubPreparer struct {
	prepared execenv.Prepared
	called   atomic.Int32
}

func (s *stubPreparer) Prepare(_ context.Context, spec execenv.StartSpec) (execenv.Prepared, error) {
	s.called.Add(1)
	s.prepared.Env = append([]string{}, s.prepared.Env...)
	s.prepared.Env = append(s.prepared.Env, execenv.EnvSessionID+"="+spec.SessionID)
	return s.prepared, nil
}

func TestManagerStartWithSpecUsesPreparerAndRunsCleanupOnReset(t *testing.T) {
	log := newTestLogger(t)
	mgr := NewManager(log)
	cleanupCalled := atomic.Int32{}
	workdir := t.TempDir()
	mgr.SetPreparer(&stubPreparer{prepared: execenv.Prepared{
		Workdir: workdir,
		Env:     []string{"TERM=xterm-256color"},
		Cleanup: func() error {
			cleanupCalled.Add(1)
			return nil
		},
	}})

	err := mgr.StartWithSpec(context.Background(), execenv.StartSpec{
		SessionID: "sess-prepared",
		Command:   "sleep 30",
		Workdir:   "/ignored",
		TaskID:    "task-prepared",
	})
	if err != nil {
		t.Fatalf("StartWithSpec failed: %v", err)
	}

	status, err := mgr.GetStatus("sess-prepared")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Workdir != workdir {
		t.Fatalf("expected prepared workdir %q, got %q", workdir, status.Workdir)
	}

	if err := mgr.Reset("sess-prepared"); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if cleanupCalled.Load() != 1 {
		t.Fatalf("expected cleanup to run exactly once, got %d", cleanupCalled.Load())
	}
}

func TestManagerStartWithSpecTracksManagedTaskLifecycle(t *testing.T) {
	log := newTestLogger(t)
	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	mgr := NewManager(log)
	mgr.SetTaskService(taskSvc)

	err := mgr.StartWithSpec(context.Background(), execenv.StartSpec{
		SessionID: "sess-task",
		TaskID:    "task-proc",
		RuntimeID: "runtime-a",
		Command:   "printf 'ok'",
		Workdir:   t.TempDir(),
		Env:       []string{},
	})
	if err != nil {
		t.Fatalf("StartWithSpec failed: %v", err)
	}

	task := waitForTaskState(t, store, "task-proc", tasks.StateCompleted, 3*time.Second)
	if task.RuntimeID != "runtime-a" {
		t.Fatalf("expected runtime id runtime-a, got %q", task.RuntimeID)
	}
	if task.Type != tasks.TypeRuntimeWorker {
		t.Fatalf("expected runtime worker task type, got %q", task.Type)
	}
	if task.SessionID != "sess-task" {
		t.Fatalf("expected session id sess-task, got %q", task.SessionID)
	}
	if task.StartedAt.IsZero() || task.CompletedAt.IsZero() {
		t.Fatalf("expected lifecycle timestamps to be recorded, got %+v", task)
	}
}

func TestManagerKillCancelsManagedTask(t *testing.T) {
	log := newTestLogger(t)
	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	mgr := NewManager(log)
	mgr.SetTaskService(taskSvc)

	err := mgr.StartWithSpec(context.Background(), execenv.StartSpec{
		SessionID: "sess-kill",
		TaskID:    "task-kill",
		Command:   "sleep 30",
		Workdir:   t.TempDir(),
		Env:       []string{},
	})
	if err != nil {
		t.Fatalf("StartWithSpec failed: %v", err)
	}

	if err := mgr.Kill("sess-kill"); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	task := waitForTaskState(t, store, "task-kill", tasks.StateCanceled, 3*time.Second)
	if task.CompletedAt.IsZero() {
		t.Fatalf("expected completed_at to be set for canceled task, got %+v", task)
	}
}

func TestManagerKillMarksCancelBeforeSendingSignal(t *testing.T) {
	log := newTestLogger(t)
	mgr := NewManager(log)
	session := &Session{
		ID:      "sess-order",
		Running: true,
		Process: &os.Process{Pid: os.Getpid()},
	}
	mgr.sessions[session.ID] = session

	originalKill := killProcess
	t.Cleanup(func() {
		killProcess = originalKill
	})

	killCalled := false
	killProcess = func(proc *os.Process) error {
		killCalled = true
		if proc != session.Process {
			t.Fatalf("expected process pointer to match session process")
		}
		if !session.cancelRequestedState() {
			t.Fatalf("expected cancel to be marked before sending kill signal")
		}
		return nil
	}

	if err := mgr.Kill(session.ID); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}
	if !killCalled {
		t.Fatal("expected kill process hook to be called")
	}
}

func waitForTaskState(t *testing.T, store *tasks.Store, taskID string, want tasks.State, timeout time.Duration) tasks.Task {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, task := range store.List() {
			if task.ID != taskID {
				continue
			}
			if task.State == want {
				return task
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	var states []string
	for _, task := range store.List() {
		states = append(states, fmt.Sprintf("%s=%s", task.ID, task.State))
	}
	t.Fatalf("task %s did not reach state %s before timeout; saw %v", taskID, want, states)
	return tasks.Task{}
}

func newTestLogger(t *testing.T) *logger.Logger {
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
