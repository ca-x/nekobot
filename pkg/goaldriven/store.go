package goaldriven

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"nekobot/pkg/goaldriven/criteria"
)

// Store persists GoalRun state.
type Store interface {
	CreateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error)
	UpdateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error)
	GetGoalRun(ctx context.Context, id string) (GoalRun, bool, error)
	ListGoalRuns(ctx context.Context) ([]GoalRun, error)
	SaveCriteria(ctx context.Context, goalRunID string, set criteria.Set) error
	LoadCriteria(ctx context.Context, goalRunID string) (criteria.Set, bool, error)
	AppendEvent(ctx context.Context, evt Event) error
	ListEvents(ctx context.Context, goalRunID string) ([]Event, error)
	SaveWorkers(ctx context.Context, goalRunID string, workers []WorkerRef) error
	LoadWorkers(ctx context.Context, goalRunID string) ([]WorkerRef, error)
}

// MemoryStore is a process-local GoalRun store for the first vertical slice.
type MemoryStore struct {
	mu       sync.RWMutex
	runs     map[string]GoalRun
	criteria map[string]criteria.Set
	events   map[string][]Event
	workers  map[string][]WorkerRef
}

// NewMemoryStore creates a new in-memory GoalRun store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs:     make(map[string]GoalRun),
		criteria: make(map[string]criteria.Set),
		events:   make(map[string][]Event),
		workers:  make(map[string][]WorkerRef),
	}
}

// CreateGoalRun inserts one GoalRun.
func (s *MemoryStore) CreateGoalRun(_ context.Context, run GoalRun) (GoalRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[run.ID]; exists {
		return GoalRun{}, fmt.Errorf("goal run %s already exists", run.ID)
	}
	s.runs[run.ID] = cloneGoalRun(run)
	return cloneGoalRun(run), nil
}

// UpdateGoalRun updates one GoalRun.
func (s *MemoryStore) UpdateGoalRun(_ context.Context, run GoalRun) (GoalRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[run.ID]; !exists {
		return GoalRun{}, fmt.Errorf("goal run %s not found", run.ID)
	}
	s.runs[run.ID] = cloneGoalRun(run)
	return cloneGoalRun(run), nil
}

// GetGoalRun loads one GoalRun by id.
func (s *MemoryStore) GetGoalRun(_ context.Context, id string) (GoalRun, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runs[id]
	if !ok {
		return GoalRun{}, false, nil
	}
	return cloneGoalRun(run), true, nil
}

// ListGoalRuns returns all GoalRuns in stable created-at order.
func (s *MemoryStore) ListGoalRuns(_ context.Context) ([]GoalRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]GoalRun, 0, len(s.runs))
	for _, run := range s.runs {
		result = append(result, cloneGoalRun(run))
	}
	slices.SortFunc(result, func(a, b GoalRun) int {
		if a.CreatedAt.Equal(b.CreatedAt) {
			switch {
			case a.ID < b.ID:
				return -1
			case a.ID > b.ID:
				return 1
			default:
				return 0
			}
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		return 1
	})
	return result, nil
}

// SaveCriteria stores one GoalRun criteria set.
func (s *MemoryStore) SaveCriteria(_ context.Context, goalRunID string, set criteria.Set) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.criteria[goalRunID] = cloneCriteriaSet(set)
	return nil
}

// LoadCriteria loads one GoalRun criteria set.
func (s *MemoryStore) LoadCriteria(_ context.Context, goalRunID string) (criteria.Set, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.criteria[goalRunID]
	if !ok {
		return criteria.Set{}, false, nil
	}
	return cloneCriteriaSet(set), true, nil
}

// AppendEvent appends one GoalRun event.
func (s *MemoryStore) AppendEvent(_ context.Context, evt Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[evt.GoalRunID] = append(s.events[evt.GoalRunID], evt)
	return nil
}

// ListEvents returns all events for one GoalRun.
func (s *MemoryStore) ListEvents(_ context.Context, goalRunID string) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.events[goalRunID]
	out := make([]Event, len(items))
	copy(out, items)
	return out, nil
}

// SaveWorkers stores the current worker refs for one GoalRun.
func (s *MemoryStore) SaveWorkers(_ context.Context, goalRunID string, workers []WorkerRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WorkerRef, len(workers))
	copy(out, workers)
	s.workers[goalRunID] = out
	return nil
}

// LoadWorkers returns the current worker refs for one GoalRun.
func (s *MemoryStore) LoadWorkers(_ context.Context, goalRunID string) ([]WorkerRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.workers[goalRunID]
	out := make([]WorkerRef, len(items))
	copy(out, items)
	return out, nil
}

func cloneGoalRun(run GoalRun) GoalRun {
	out := run
	if run.RecommendedScope != nil {
		scope := *run.RecommendedScope
		out.RecommendedScope = &scope
	}
	if run.SelectedScope != nil {
		scope := *run.SelectedScope
		out.SelectedScope = &scope
	}
	out.CurrentWorkerIDs = append([]string(nil), run.CurrentWorkerIDs...)
	return out
}

func cloneCriteriaSet(set criteria.Set) criteria.Set {
	out := criteria.Set{Criteria: make([]criteria.Item, len(set.Criteria))}
	for i, item := range set.Criteria {
		cloned := item
		if item.Definition != nil {
			cloned.Definition = make(map[string]any, len(item.Definition))
			for key, value := range item.Definition {
				cloned.Definition[key] = value
			}
		}
		cloned.Evidence = append([]string(nil), item.Evidence...)
		out.Criteria[i] = cloned
	}
	return out
}
