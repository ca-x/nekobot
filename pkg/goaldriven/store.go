package goaldriven

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"nekobot/pkg/goaldriven/criteria"
	"nekobot/pkg/state"
)

// Store persists GoalRun state.
type Store interface {
	CreateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error)
	UpdateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error)
	GetGoalRun(ctx context.Context, id string) (GoalRun, bool, error)
	ListGoalRuns(ctx context.Context) ([]GoalRun, error)
	DeleteGoalRun(ctx context.Context, goalRunID string) error
	SaveCriteria(ctx context.Context, goalRunID string, set criteria.Set) error
	LoadCriteria(ctx context.Context, goalRunID string) (criteria.Set, bool, error)
	DeleteCriteria(ctx context.Context, goalRunID string) error
	AppendEvent(ctx context.Context, evt Event) error
	ListEvents(ctx context.Context, goalRunID string) ([]Event, error)
	SaveWorkers(ctx context.Context, goalRunID string, workers []WorkerRef) error
	LoadWorkers(ctx context.Context, goalRunID string) ([]WorkerRef, error)
	DeleteWorkers(ctx context.Context, goalRunID string) error
}

const (
	goalRunIndexKey = "goaldriven.index.v1"
)

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

// DeleteGoalRun removes one GoalRun record.
func (s *MemoryStore) DeleteGoalRun(_ context.Context, goalRunID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, strings.TrimSpace(goalRunID))
	return nil
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

// DeleteCriteria removes one GoalRun criteria set.
func (s *MemoryStore) DeleteCriteria(_ context.Context, goalRunID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.criteria, strings.TrimSpace(goalRunID))
	return nil
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

// DeleteWorkers removes stored worker refs for one GoalRun.
func (s *MemoryStore) DeleteWorkers(_ context.Context, goalRunID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workers, strings.TrimSpace(goalRunID))
	return nil
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

// KVStore persists GoalRuns in the shared state.KV backend.
type KVStore struct {
	kv state.KV
}

// NewPersistentStore creates a GoalRun store backed by state.KV when available,
// otherwise it falls back to the in-memory store for tests or minimal setups.
func NewPersistentStore(kv state.KV) Store {
	if kv == nil {
		return NewMemoryStore()
	}
	return &KVStore{kv: kv}
}

// CreateGoalRun inserts one GoalRun.
func (s *KVStore) CreateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error) {
	if s == nil || s.kv == nil {
		return GoalRun{}, fmt.Errorf("kv store is unavailable")
	}
	key := goalRunKey(run.ID)
	if exists, err := s.kv.Exists(ctx, key); err != nil {
		return GoalRun{}, fmt.Errorf("check existing goal run: %w", err)
	} else if exists {
		return GoalRun{}, fmt.Errorf("goal run %s already exists", run.ID)
	}
	if err := setJSON(ctx, s.kv, key, cloneGoalRun(run)); err != nil {
		return GoalRun{}, fmt.Errorf("persist goal run: %w", err)
	}
	if err := s.kv.UpdateFunc(ctx, goalRunIndexKey, func(current interface{}) interface{} {
		ids := decodeIndex(current)
		for _, id := range ids {
			if id == run.ID {
				return ids
			}
		}
		return append(ids, run.ID)
	}); err != nil {
		_ = s.kv.Delete(ctx, key)
		return GoalRun{}, fmt.Errorf("update goal run index: %w", err)
	}
	return cloneGoalRun(run), nil
}

// UpdateGoalRun updates one GoalRun.
func (s *KVStore) UpdateGoalRun(ctx context.Context, run GoalRun) (GoalRun, error) {
	if s == nil || s.kv == nil {
		return GoalRun{}, fmt.Errorf("kv store is unavailable")
	}
	key := goalRunKey(run.ID)
	if exists, err := s.kv.Exists(ctx, key); err != nil {
		return GoalRun{}, fmt.Errorf("check goal run: %w", err)
	} else if !exists {
		return GoalRun{}, fmt.Errorf("goal run %s not found", run.ID)
	}
	if err := setJSON(ctx, s.kv, key, cloneGoalRun(run)); err != nil {
		return GoalRun{}, fmt.Errorf("persist goal run: %w", err)
	}
	return cloneGoalRun(run), nil
}

// GetGoalRun loads one GoalRun by id.
func (s *KVStore) GetGoalRun(ctx context.Context, id string) (GoalRun, bool, error) {
	if s == nil || s.kv == nil {
		return GoalRun{}, false, fmt.Errorf("kv store is unavailable")
	}
	var run GoalRun
	ok, err := getJSON(ctx, s.kv, goalRunKey(id), &run)
	if err != nil || !ok {
		return GoalRun{}, ok, err
	}
	return cloneGoalRun(run), true, nil
}

// ListGoalRuns returns all GoalRuns in stable created-at order.
func (s *KVStore) ListGoalRuns(ctx context.Context) ([]GoalRun, error) {
	if s == nil || s.kv == nil {
		return nil, fmt.Errorf("kv store is unavailable")
	}
	ids, hasIndex, err := s.index(ctx)
	if err != nil {
		return nil, err
	}
	if !hasIndex {
		ids, err = s.discoverRunIDs(ctx)
		if err != nil {
			return nil, err
		}
		if err := s.kv.Set(ctx, goalRunIndexKey, ids); err != nil {
			return nil, fmt.Errorf("persist discovered goal run index: %w", err)
		}
	}
	result := make([]GoalRun, 0, len(ids))
	for _, id := range ids {
		run, ok, err := s.GetGoalRun(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, run)
		}
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

// DeleteGoalRun removes one GoalRun record and its index entry.
func (s *KVStore) DeleteGoalRun(ctx context.Context, goalRunID string) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	id := strings.TrimSpace(goalRunID)
	if id == "" {
		return nil
	}
	if err := s.kv.Delete(ctx, goalRunKey(id)); err != nil {
		return fmt.Errorf("delete goal run %s: %w", id, err)
	}
	if err := s.kv.UpdateFunc(ctx, goalRunIndexKey, func(current interface{}) interface{} {
		ids := decodeIndex(current)
		out := make([]string, 0, len(ids))
		for _, item := range ids {
			if item != id {
				out = append(out, item)
			}
		}
		return out
	}); err != nil {
		return fmt.Errorf("update goal run index: %w", err)
	}
	return nil
}

// SaveCriteria stores one GoalRun criteria set.
func (s *KVStore) SaveCriteria(ctx context.Context, goalRunID string, set criteria.Set) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	return setJSON(ctx, s.kv, goalRunCriteriaKey(goalRunID), cloneCriteriaSet(set))
}

// LoadCriteria loads one GoalRun criteria set.
func (s *KVStore) LoadCriteria(ctx context.Context, goalRunID string) (criteria.Set, bool, error) {
	if s == nil || s.kv == nil {
		return criteria.Set{}, false, fmt.Errorf("kv store is unavailable")
	}
	var set criteria.Set
	ok, err := getJSON(ctx, s.kv, goalRunCriteriaKey(goalRunID), &set)
	if err != nil || !ok {
		return criteria.Set{}, ok, err
	}
	return cloneCriteriaSet(set), true, nil
}

// DeleteCriteria removes one persisted criteria set.
func (s *KVStore) DeleteCriteria(ctx context.Context, goalRunID string) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	if err := s.kv.Delete(ctx, goalRunCriteriaKey(goalRunID)); err != nil {
		return fmt.Errorf("delete criteria: %w", err)
	}
	return nil
}

// AppendEvent appends one GoalRun event.
func (s *KVStore) AppendEvent(ctx context.Context, evt Event) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	key := goalRunEventsKey(evt.GoalRunID)
	return s.kv.UpdateFunc(ctx, key, func(current interface{}) interface{} {
		items := decodeEvents(current)
		items = append(items, evt)
		return items
	})
}

// ListEvents returns all events for one GoalRun.
func (s *KVStore) ListEvents(ctx context.Context, goalRunID string) ([]Event, error) {
	if s == nil || s.kv == nil {
		return nil, fmt.Errorf("kv store is unavailable")
	}
	var items []Event
	ok, err := getJSON(ctx, s.kv, goalRunEventsKey(goalRunID), &items)
	if err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	return items, nil
}

// SaveWorkers stores the current worker refs for one GoalRun.
func (s *KVStore) SaveWorkers(ctx context.Context, goalRunID string, workers []WorkerRef) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	out := make([]WorkerRef, len(workers))
	copy(out, workers)
	return setJSON(ctx, s.kv, goalRunWorkersKey(goalRunID), out)
}

// LoadWorkers returns the current worker refs for one GoalRun.
func (s *KVStore) LoadWorkers(ctx context.Context, goalRunID string) ([]WorkerRef, error) {
	if s == nil || s.kv == nil {
		return nil, fmt.Errorf("kv store is unavailable")
	}
	var items []WorkerRef
	ok, err := getJSON(ctx, s.kv, goalRunWorkersKey(goalRunID), &items)
	if err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	return items, nil
}

// DeleteWorkers removes persisted worker refs for one GoalRun.
func (s *KVStore) DeleteWorkers(ctx context.Context, goalRunID string) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("kv store is unavailable")
	}
	if err := s.kv.Delete(ctx, goalRunWorkersKey(goalRunID)); err != nil {
		return fmt.Errorf("delete workers: %w", err)
	}
	return nil
}

func (s *KVStore) index(ctx context.Context) ([]string, bool, error) {
	value, ok, err := s.kv.Get(ctx, goalRunIndexKey)
	if err != nil {
		return nil, false, fmt.Errorf("load goal run index: %w", err)
	}
	if !ok {
		return nil, false, nil
	}
	return decodeIndex(value), true, nil
}

func (s *KVStore) discoverRunIDs(ctx context.Context) ([]string, error) {
	keys, err := s.kv.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list kv keys: %w", err)
	}
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(strings.TrimSpace(key), "goaldriven.run.") {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(key), "goaldriven.run.")
		if strings.TrimSpace(id) == "" {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func goalRunKey(id string) string         { return "goaldriven.run." + strings.TrimSpace(id) }
func goalRunCriteriaKey(id string) string { return "goaldriven.criteria." + strings.TrimSpace(id) }
func goalRunEventsKey(id string) string   { return "goaldriven.events." + strings.TrimSpace(id) }
func goalRunWorkersKey(id string) string  { return "goaldriven.workers." + strings.TrimSpace(id) }

func setJSON(ctx context.Context, kv state.KV, key string, value any) error {
	return kv.Set(ctx, key, value)
}

func getJSON(ctx context.Context, kv state.KV, key string, target any) (bool, error) {
	raw, ok, err := kv.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}

	var data []byte
	switch value := raw.(type) {
	case string:
		data = []byte(value)
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return false, fmt.Errorf("encode key %s: %w", key, err)
		}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("decode key %s: %w", key, err)
	}
	return true, nil
}

func decodeIndex(current interface{}) []string {
	switch value := current.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		var ids []string
		if err := json.Unmarshal([]byte(value), &ids); err != nil {
			return nil
		}
		return ids
	case []string:
		return append([]string(nil), value...)
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}

func decodeEvents(current interface{}) []Event {
	raw, _ := current.(string)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []Event
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}
