// Package agent provides the core agent functionality for nanobot.
package agent

import (
	"context"
	"fmt"
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

// ReadLongTerm reads the long-term memory content.
// Returns empty string if read fails.
func (ms *MemoryStore) ReadLongTerm() string {
	content, err := ms.backend.ReadLongTerm(context.Background())
	if err != nil {
		return ""
	}
	return content
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
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	parts := make([]string, 0, 2)

	longTerm := ms.ReadLongTerm()
	if strings.TrimSpace(longTerm) != "" {
		parts = append(parts, "## Long-term Memory\n\n"+longTerm)
	}

	recentNotes := ms.GetRecentDailyNotes(3)
	if strings.TrimSpace(recentNotes) != "" {
		parts = append(parts, "## Recent Daily Notes (Last 3 Days)\n\n"+recentNotes)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n---\n\n")
}
