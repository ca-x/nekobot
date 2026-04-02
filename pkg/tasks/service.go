package tasks

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrTaskNotFound indicates a managed task does not exist.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskExists indicates a managed task with the same ID already exists.
	ErrTaskExists = errors.New("task already exists")
	// ErrInvalidTransition indicates a lifecycle transition is not allowed.
	ErrInvalidTransition = errors.New("invalid task transition")
)

// Service manages an in-memory lifecycle for explicitly managed tasks.
type Service struct {
	store *Store

	mu    sync.RWMutex
	tasks map[string]Task
	now   func() time.Time
}

// NewService creates a managed task lifecycle service.
func NewService(store *Store) *Service {
	svc := &Service{
		store: store,
		tasks: make(map[string]Task),
		now:   time.Now,
	}
	if store != nil {
		store.SetSource("managed", svc.List)
	}
	return svc
}

// List returns managed tasks sorted by ID for deterministic aggregation.
func (s *Service) List() []Task {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.tasks) == 0 {
		return nil
	}

	ids := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]Task, 0, len(ids))
	for _, id := range ids {
		result = append(result, cloneTask(s.tasks[id]))
	}
	return result
}

// Enqueue creates a new managed task in pending state.
func (s *Service) Enqueue(task Task) (Task, error) {
	if s == nil {
		return Task{}, fmt.Errorf("service is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	normalized, err := normalizeNewTask(task, now)
	if err != nil {
		return Task{}, err
	}
	if _, exists := s.tasks[normalized.ID]; exists {
		return Task{}, ErrTaskExists
	}
	s.tasks[normalized.ID] = normalized
	return cloneTask(normalized), nil
}

// Claim marks a pending task as claimed by one runtime.
func (s *Service) Claim(taskID, runtimeID string) (Task, error) {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return Task{}, fmt.Errorf("runtime id is required")
	}
	return s.transition(taskID, func(task *Task, _ func() time.Time) error {
		if task.State != StatePending {
			return transitionError(task.ID, task.State, StateClaimed)
		}
		task.State = StateClaimed
		task.RuntimeID = runtimeID
		return nil
	})
}

// Start marks a claimed or action-blocked task as running.
func (s *Service) Start(taskID string) (Task, error) {
	return s.transition(taskID, func(task *Task, now func() time.Time) error {
		if task.State != StateClaimed && task.State != StateRequiresAction {
			return transitionError(task.ID, task.State, StateRunning)
		}
		task.State = StateRunning
		task.PendingAction = ""
		task.LastError = ""
		if task.StartedAt.IsZero() {
			task.StartedAt = now()
		}
		return nil
	})
}

// RequireAction moves a running task into an action-blocked state.
func (s *Service) RequireAction(taskID, pendingAction string) (Task, error) {
	pendingAction = strings.TrimSpace(pendingAction)
	if pendingAction == "" {
		return Task{}, fmt.Errorf("pending action is required")
	}
	return s.transition(taskID, func(task *Task, _ func() time.Time) error {
		if task.State != StateRunning {
			return transitionError(task.ID, task.State, StateRequiresAction)
		}
		task.State = StateRequiresAction
		task.PendingAction = pendingAction
		return nil
	})
}

// Complete marks a running task as completed.
func (s *Service) Complete(taskID string) (Task, error) {
	return s.transition(taskID, func(task *Task, now func() time.Time) error {
		if task.State != StateRunning && task.State != StateRequiresAction {
			return transitionError(task.ID, task.State, StateCompleted)
		}
		task.State = StateCompleted
		task.PendingAction = ""
		task.CompletedAt = now()
		task.LastError = ""
		return nil
	})
}

// Fail marks a non-final task as failed.
func (s *Service) Fail(taskID, lastError string) (Task, error) {
	lastError = strings.TrimSpace(lastError)
	if lastError == "" {
		return Task{}, fmt.Errorf("last error is required")
	}
	return s.finish(taskID, StateFailed, lastError)
}

// Cancel marks a pending or in-flight task as canceled.
func (s *Service) Cancel(taskID string) (Task, error) {
	return s.transition(taskID, func(task *Task, now func() time.Time) error {
		if IsFinal(task.State) {
			return transitionError(task.ID, task.State, StateCanceled)
		}
		switch task.State {
		case StatePending, StateClaimed, StateRunning, StateRequiresAction:
		default:
			return transitionError(task.ID, task.State, StateCanceled)
		}
		task.State = StateCanceled
		task.PendingAction = ""
		task.CompletedAt = now()
		return nil
	})
}

func (s *Service) finish(taskID string, finalState State, lastError string) (Task, error) {
	return s.transition(taskID, func(task *Task, now func() time.Time) error {
		if IsFinal(task.State) {
			return transitionError(task.ID, task.State, finalState)
		}
		switch task.State {
		case StatePending, StateClaimed, StateRunning, StateRequiresAction:
		default:
			return transitionError(task.ID, task.State, finalState)
		}
		task.State = finalState
		task.PendingAction = ""
		task.CompletedAt = now()
		task.LastError = lastError
		return nil
	})
}

func (s *Service) transition(taskID string, apply func(task *Task, now func() time.Time) error) (Task, error) {
	if s == nil {
		return Task{}, fmt.Errorf("service is nil")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Task{}, fmt.Errorf("task id is required")
	}
	if apply == nil {
		return Task{}, fmt.Errorf("transition function is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return Task{}, ErrTaskNotFound
	}
	if err := apply(&task, s.now); err != nil {
		return Task{}, err
	}
	s.tasks[taskID] = task
	return cloneTask(task), nil
}

func normalizeNewTask(task Task, now time.Time) (Task, error) {
	task.ID = strings.TrimSpace(task.ID)
	if task.ID == "" {
		return Task{}, fmt.Errorf("task id is required")
	}
	if task.Type == "" {
		return Task{}, fmt.Errorf("task type is required")
	}
	if task.State != "" && task.State != StatePending {
		return Task{}, transitionError(task.ID, task.State, StatePending)
	}
	task.State = StatePending
	task.Summary = strings.TrimSpace(task.Summary)
	task.SessionID = strings.TrimSpace(task.SessionID)
	task.RuntimeID = strings.TrimSpace(task.RuntimeID)
	task.ActualProvider = strings.TrimSpace(task.ActualProvider)
	task.ActualModel = strings.TrimSpace(task.ActualModel)
	task.PermissionMode = strings.TrimSpace(task.PermissionMode)
	task.PendingAction = ""
	task.LastError = ""
	task.CreatedAt = now
	task.StartedAt = time.Time{}
	task.CompletedAt = time.Time{}
	if task.Metadata != nil {
		task.Metadata = cloneMetadata(task.Metadata)
	}
	return task, nil
}

func transitionError(taskID string, from, to State) error {
	return fmt.Errorf("%w: task %s cannot transition from %s to %s", ErrInvalidTransition, taskID, from, to)
}

func cloneTask(task Task) Task {
	if task.Metadata != nil {
		task.Metadata = cloneMetadata(task.Metadata)
	}
	return task
}

func cloneMetadata(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
