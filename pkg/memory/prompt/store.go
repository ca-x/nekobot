package prompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backend defines persistent prompt-memory operations regardless of storage medium.
type Backend interface {
	ReadLongTerm(ctx context.Context) (string, error)
	WriteLongTerm(ctx context.Context, content string) error
	ReadDaily(ctx context.Context, day time.Time) (string, error)
	WriteDaily(ctx context.Context, day time.Time, content string) error
}

// Store manages prompt-facing persistent memory for an agent workspace.
type Store struct {
	workspace   string
	backend     Backend
	backendName string
}

// ContextOptions controls how persistent memory is rendered into prompt context.
type ContextOptions struct {
	IncludeWorkspaceMemory bool
	IncludeLongTerm        bool
	IncludeActiveLearnings bool
	RecentDailyNoteDays    int
	MaxChars               int
}

// NewStore creates a file-backed prompt-memory store for the workspace.
func NewStore(workspace string) *Store {
	backend, err := NewFileBackend(filepath.Join(workspace, "memory"))
	if err != nil {
		return NewStoreWithBackend(workspace, NewNoopBackend())
	}
	return NewStoreWithBackend(workspace, backend)
}

// NewStoreWithBackend creates a prompt-memory store with an explicit backend.
func NewStoreWithBackend(workspace string, backend Backend) *Store {
	if backend == nil {
		backend = NewNoopBackend()
	}
	return &Store{
		workspace:   workspace,
		backend:     backend,
		backendName: backendName(backend),
	}
}

// BackendName returns the resolved backend label.
func (s *Store) BackendName() string {
	if s == nil {
		return "noop"
	}
	return s.backendName
}

// DefaultContextOptions returns the default prompt memory composition.
func DefaultContextOptions() ContextOptions {
	return ContextOptions{
		IncludeWorkspaceMemory: true,
		IncludeLongTerm:        true,
		IncludeActiveLearnings: true,
		RecentDailyNoteDays:    1,
		MaxChars:               8000,
	}
}

// ReadLongTerm reads the long-term memory content.
// Returns empty string if read fails.
func (s *Store) ReadLongTerm() string {
	content, err := s.backend.ReadLongTerm(context.Background())
	if err != nil {
		return ""
	}
	return content
}

// ReadWorkspaceMemory reads workspace-scoped memory from workspace/MEMORY.md.
func (s *Store) ReadWorkspaceMemory() string {
	if strings.TrimSpace(s.workspace) == "" {
		return ""
	}

	path := filepath.Join(s.workspace, "MEMORY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadActiveLearnings reads active learnings from workspace/memory/active_learnings.md.
func (s *Store) ReadActiveLearnings() string {
	if strings.TrimSpace(s.workspace) == "" {
		return ""
	}

	path := filepath.Join(s.workspace, "memory", "active_learnings.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// WriteLongTerm writes content to long-term memory.
func (s *Store) WriteLongTerm(content string) error {
	if err := s.backend.WriteLongTerm(context.Background(), content); err != nil {
		return fmt.Errorf("write long-term memory: %w", err)
	}
	return nil
}

// AppendLongTerm appends content to long-term memory.
func (s *Store) AppendLongTerm(content string) error {
	existing := s.ReadLongTerm()
	if existing != "" {
		existing += "\n\n"
	}
	if err := s.WriteLongTerm(existing + content); err != nil {
		return fmt.Errorf("append long-term memory: %w", err)
	}
	return nil
}

func (s *Store) today() time.Time {
	return time.Now()
}

// ReadToday reads today's daily note.
// Returns empty string if read fails.
func (s *Store) ReadToday() string {
	content, err := s.backend.ReadDaily(context.Background(), s.today())
	if err != nil {
		return ""
	}
	return content
}

// AppendToday appends content to today's daily note.
// If today's note does not exist, it creates one with a date header.
func (s *Store) AppendToday(content string) error {
	today := s.today()
	existing, err := s.backend.ReadDaily(context.Background(), today)
	if err != nil {
		return fmt.Errorf("read daily memory before append: %w", err)
	}

	updated := existing + "\n\n" + content
	if strings.TrimSpace(existing) == "" {
		header := fmt.Sprintf("# %s\n\n", today.Format("2006-01-02 Monday"))
		updated = header + content
	}

	if err := s.backend.WriteDaily(context.Background(), today, updated); err != nil {
		return fmt.Errorf("append daily memory: %w", err)
	}
	return nil
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (s *Store) GetRecentDailyNotes(days int) string {
	if days <= 0 {
		return ""
	}

	notes := make([]string, 0, days)
	now := s.today()
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i)
		note, err := s.backend.ReadDaily(context.Background(), day)
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
// Deprecated: prompt composition should use ContextComposer directly.
func (s *Store) GetMemoryContext() string {
	return NewContextComposer(s, DefaultContextOptions()).Build()
}

func backendName(backend Backend) string {
	switch backend.(type) {
	case *fileBackend:
		return "file"
	case *dbBackend:
		return "db"
	case *kvBackend:
		return "kv"
	default:
		return "noop"
	}
}
