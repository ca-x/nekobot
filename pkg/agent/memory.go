// Package agent provides the core agent functionality for nanobot.
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore manages persistent memory for the agent.
// It provides two types of memory:
// - Long-term memory: memory/MEMORY.md (persistent facts, instructions)
// - Daily notes: memory/YYYYMM/YYYYMMDD.md (daily activities, logs)
type MemoryStore struct {
	workspace string
	backend   MemoryBackend
}

// MemoryContextOptions controls how persistent memory is rendered into prompt context.
type MemoryContextOptions struct {
	IncludeWorkspaceMemory bool
	IncludeLongTerm        bool
	RecentDailyNoteDays    int
	MaxChars               int
}

// NewMemoryStore creates a file-backed memory store for the workspace.
func NewMemoryStore(workspace string) *MemoryStore {
	backend, err := newMemoryFileBackend(filepath.Join(workspace, "memory"))
	if err != nil {
		return NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{})
	}
	return NewMemoryStoreWithBackend(workspace, backend)
}

// NewMemoryStoreWithBackend creates a memory store with an explicit backend.
func NewMemoryStoreWithBackend(workspace string, backend MemoryBackend) *MemoryStore {
	if backend == nil {
		backend = &memoryNoopBackend{}
	}
	return &MemoryStore{
		workspace: workspace,
		backend:   backend,
	}
}

// DefaultMemoryContextOptions returns the default prompt memory composition.
func DefaultMemoryContextOptions() MemoryContextOptions {
	return MemoryContextOptions{
		IncludeWorkspaceMemory: true,
		IncludeLongTerm:        true,
		RecentDailyNoteDays:    1,
		MaxChars:               8000,
	}
}

// ReadLongTerm reads the long-term memory content.
// Returns empty string if read fails.
func (ms *MemoryStore) ReadLongTerm() string {
	content, err := ms.backend.ReadLongTerm(context.Background())
	if err != nil {
		return ""
	}
	return content
}

// ReadWorkspaceMemory reads workspace-scoped memory from workspace/MEMORY.md.
func (ms *MemoryStore) ReadWorkspaceMemory() string {
	if strings.TrimSpace(ms.workspace) == "" {
		return ""
	}

	path := filepath.Join(ms.workspace, "MEMORY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// WriteLongTerm writes content to long-term memory.
func (ms *MemoryStore) WriteLongTerm(content string) error {
	if err := ms.backend.WriteLongTerm(context.Background(), content); err != nil {
		return fmt.Errorf("write long-term memory: %w", err)
	}
	return nil
}

// AppendLongTerm appends content to long-term memory.
func (ms *MemoryStore) AppendLongTerm(content string) error {
	existing := ms.ReadLongTerm()
	if existing != "" {
		existing += "\n\n"
	}
	if err := ms.WriteLongTerm(existing + content); err != nil {
		return fmt.Errorf("append long-term memory: %w", err)
	}
	return nil
}

// getToday returns today's date in local timezone.
func (ms *MemoryStore) getToday() time.Time {
	return time.Now()
}

// ReadToday reads today's daily note.
// Returns empty string if read fails.
func (ms *MemoryStore) ReadToday() string {
	content, err := ms.backend.ReadDaily(context.Background(), ms.getToday())
	if err != nil {
		return ""
	}
	return content
}

// AppendToday appends content to today's daily note.
// If today's note does not exist, it creates one with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	today := ms.getToday()
	existing, err := ms.backend.ReadDaily(context.Background(), today)
	if err != nil {
		return fmt.Errorf("read daily memory before append: %w", err)
	}

	updated := content
	if strings.TrimSpace(existing) == "" {
		header := fmt.Sprintf("# %s\n\n", today.Format("2006-01-02 Monday"))
		updated = header + content
	} else {
		updated = existing + "\n\n" + content
	}

	if err := ms.backend.WriteDaily(context.Background(), today, updated); err != nil {
		return fmt.Errorf("append daily memory: %w", err)
	}
	return nil
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	if days <= 0 {
		return ""
	}

	notes := make([]string, 0, days)
	now := ms.getToday()
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i)
		note, err := ms.backend.ReadDaily(context.Background(), day)
		if err != nil {
			continue
		}
		if strings.TrimSpace(note) == "" {
			continue
		}
		notes = append(notes, note)
	}

	if len(notes) == 0 {
		return ""
	}

	return strings.Join(notes, "\n\n---\n\n")
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Deprecated: prompt composition should use MemoryContextComposer.
func (ms *MemoryStore) GetMemoryContext() string {
	return NewMemoryContextComposer(ms, DefaultMemoryContextOptions()).Build()
}
