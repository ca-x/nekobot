package tasks

import (
	"errors"
	"testing"
	"time"
)

func TestServiceLifecycleVisibleThroughStore(t *testing.T) {
	store := NewStore()
	store.SetSource("alpha", func() []Task {
		return []Task{{ID: "alpha-1", State: StateRunning}}
	})
	store.SetSource("zeta", func() []Task {
		return []Task{{ID: "zeta-1", State: StatePending}}
	})

	svc := NewService(store)
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	nowValues := []time.Time{
		base,
		base.Add(1 * time.Minute),
		base.Add(2 * time.Minute),
		base.Add(3 * time.Minute),
		base.Add(3 * time.Minute),
	}
	svc.now = func() time.Time {
		if len(nowValues) == 0 {
			t.Fatal("unexpected now() call")
		}
		current := nowValues[0]
		nowValues = nowValues[1:]
		return current
	}

	task, err := svc.Enqueue(Task{
		ID:        "managed-1",
		Type:      TypeBackgroundAgent,
		Summary:   "managed task",
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if task.State != StatePending {
		t.Fatalf("expected pending state after enqueue, got %s", task.State)
	}

	task, err = svc.Claim("managed-1", "runtime-a")
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if task.State != StateClaimed {
		t.Fatalf("expected claimed state, got %s", task.State)
	}
	if task.RuntimeID != "runtime-a" {
		t.Fatalf("expected runtime id to be recorded, got %q", task.RuntimeID)
	}

	task, err = svc.Start("managed-1")
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if task.State != StateRunning {
		t.Fatalf("expected running state, got %s", task.State)
	}
	if !task.StartedAt.Equal(base.Add(1 * time.Minute)) {
		t.Fatalf("unexpected started_at: %s", task.StartedAt)
	}

	task, err = svc.RequireAction("managed-1", "approve tool call")
	if err != nil {
		t.Fatalf("require action failed: %v", err)
	}
	if task.State != StateRequiresAction {
		t.Fatalf("expected requires_action state, got %s", task.State)
	}
	if task.PendingAction != "approve tool call" {
		t.Fatalf("expected pending action, got %q", task.PendingAction)
	}

	task, err = svc.Start("managed-1")
	if err != nil {
		t.Fatalf("resume start failed: %v", err)
	}
	if task.State != StateRunning {
		t.Fatalf("expected running state after resume, got %s", task.State)
	}
	if task.PendingAction != "" {
		t.Fatalf("expected pending action to be cleared, got %q", task.PendingAction)
	}
	if !task.StartedAt.Equal(base.Add(1 * time.Minute)) {
		t.Fatalf("expected original started_at to be preserved, got %s", task.StartedAt)
	}

	task, err = svc.Complete("managed-1")
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if task.State != StateCompleted {
		t.Fatalf("expected completed state, got %s", task.State)
	}
	if !task.CompletedAt.Equal(base.Add(2 * time.Minute)) {
		t.Fatalf("unexpected completed_at: %s", task.CompletedAt)
	}

	got := store.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks in aggregated view, got %d", len(got))
	}
	if got[0].ID != "alpha-1" || got[1].ID != "managed-1" || got[2].ID != "zeta-1" {
		t.Fatalf("unexpected aggregated ordering: %+v", got)
	}
	if got[1].State != StateCompleted {
		t.Fatalf("expected managed task to be visible as completed, got %+v", got[1])
	}
}

func TestServiceListOrdersManagedTasksDeterministically(t *testing.T) {
	svc := NewService(nil)
	createdAt := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time {
		return createdAt
	}

	if _, err := svc.Enqueue(Task{ID: "b-task", Type: TypeLocalAgent}); err != nil {
		t.Fatalf("enqueue b-task failed: %v", err)
	}
	if _, err := svc.Enqueue(Task{ID: "a-task", Type: TypeLocalAgent}); err != nil {
		t.Fatalf("enqueue a-task failed: %v", err)
	}

	got := svc.List()
	if len(got) != 2 {
		t.Fatalf("expected 2 managed tasks, got %d", len(got))
	}
	if got[0].ID != "a-task" || got[1].ID != "b-task" {
		t.Fatalf("unexpected managed ordering: %+v", got)
	}
}

func TestServiceRejectsInvalidTransitions(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.Enqueue(Task{ID: "task-1", Type: TypeRuntimeWorker}); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	if _, err := svc.Start("task-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition starting pending task, got %v", err)
	}
	if _, err := svc.RequireAction("task-1", "approve"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition requiring action from pending task, got %v", err)
	}
	if _, err := svc.Complete("task-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition completing pending task, got %v", err)
	}

	if _, err := svc.Claim("task-1", "runtime-a"); err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if _, err := svc.Claim("task-1", "runtime-b"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition re-claiming task, got %v", err)
	}
	if _, err := svc.Cancel("task-1"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if _, err := svc.Start("task-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition starting canceled task, got %v", err)
	}
}

func TestServiceFailAndCancelAreTerminal(t *testing.T) {
	svc := NewService(nil)
	nowValues := []time.Time{
		time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 11, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 11, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 11, 3, 0, 0, time.UTC),
	}
	svc.now = func() time.Time {
		current := nowValues[0]
		nowValues = nowValues[1:]
		return current
	}

	if _, err := svc.Enqueue(Task{ID: "task-fail", Type: TypeRemoteAgent}); err != nil {
		t.Fatalf("enqueue task-fail failed: %v", err)
	}
	task, err := svc.Fail("task-fail", "upstream timeout")
	if err != nil {
		t.Fatalf("fail failed: %v", err)
	}
	if task.State != StateFailed {
		t.Fatalf("expected failed state, got %s", task.State)
	}
	if task.LastError != "upstream timeout" {
		t.Fatalf("expected last error to be recorded, got %q", task.LastError)
	}
	if !task.CompletedAt.Equal(time.Date(2026, 4, 1, 11, 1, 0, 0, time.UTC)) {
		t.Fatalf("unexpected completed_at for failed task: %s", task.CompletedAt)
	}

	if _, err := svc.Enqueue(Task{ID: "task-cancel", Type: TypeRemoteAgent}); err != nil {
		t.Fatalf("enqueue task-cancel failed: %v", err)
	}
	task, err = svc.Cancel("task-cancel")
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if task.State != StateCanceled {
		t.Fatalf("expected canceled state, got %s", task.State)
	}
	if !IsFinal(task.State) {
		t.Fatalf("expected canceled task to be final")
	}
}
