// Package tasks defines a shared task state model for local and future remote execution units.
package tasks

import (
	"sort"
	"sync"
)

// SnapshotFunc returns the current task snapshots for one runtime source.
type SnapshotFunc func() []Task

// Store aggregates task snapshots from multiple runtime sources.
type Store struct {
	mu            sync.RWMutex
	sources       map[string]SnapshotFunc
	sessionStates map[string]SessionState
}

// NewStore creates an empty task snapshot store.
func NewStore() *Store {
	return &Store{
		sources:       make(map[string]SnapshotFunc),
		sessionStates: make(map[string]SessionState),
	}
}

// SetSource registers or replaces one named task snapshot source.
func (s *Store) SetSource(name string, source SnapshotFunc) {
	if s == nil || name == "" || source == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sources == nil {
		s.sources = make(map[string]SnapshotFunc)
	}
	s.sources[name] = source
}

// RemoveSource unregisters one named task snapshot source.
func (s *Store) RemoveSource(name string) {
	if s == nil || name == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sources, name)
}

// List returns an aggregated snapshot view across all registered sources.
func (s *Store) List() []Task {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	if len(s.sources) == 0 {
		s.mu.RUnlock()
		return nil
	}

	names := make([]string, 0, len(s.sources))
	for name := range s.sources {
		names = append(names, name)
	}
	sort.Strings(names)

	sources := make([]SnapshotFunc, 0, len(names))
	for _, name := range names {
		sources = append(sources, s.sources[name])
	}
	s.mu.RUnlock()

	var result []Task
	for _, source := range sources {
		if source == nil {
			continue
		}
		result = append(result, source()...)
	}
	return result
}
