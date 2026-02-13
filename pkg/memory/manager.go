package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Manager manages the memory system including embeddings and storage.
type Manager struct {
	store    Store
	provider EmbeddingProvider
	enabled  bool
}

// NewManager creates a new memory manager.
func NewManager(storePath string, provider EmbeddingProvider) (*Manager, error) {
	if provider == nil {
		// Use simple provider if none specified (testing/fallback)
		provider = NewSimpleEmbeddingProvider(384)
	}

	store, err := NewFileStore(storePath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return &Manager{
		store:    store,
		provider: provider,
		enabled:  true,
	}, nil
}

// Add adds text to memory with automatic embedding generation.
func (m *Manager) Add(ctx context.Context, text string, source Source, typ Type, metadata Metadata) error {
	if !m.enabled {
		return fmt.Errorf("memory system is disabled")
	}

	// Generate embedding
	vector, err := m.provider.Embed(text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create embedding entry
	emb := &Embedding{
		ID:        uuid.New().String(),
		Vector:    vector,
		Dimension: len(vector),
		Text:      text,
		Source:    source,
		Type:      typ,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  metadata,
	}

	// Store embedding
	if err := m.store.Add(emb); err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// Search searches memory for relevant content.
func (m *Manager) Search(ctx context.Context, query string, opts SearchOptions) ([]*SearchResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory system is disabled")
	}

	// Generate query embedding
	queryVector, err := m.provider.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search store
	results, err := m.store.Search(queryVector, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	return results, nil
}

// IndexDirectory indexes all text files in a directory.
func (m *Manager) IndexDirectory(ctx context.Context, dirPath string, source Source) error {
	if !m.enabled {
		return fmt.Errorf("memory system is disabled")
	}

	// Walk directory
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only index text files
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".txt" {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		text := string(content)

		// Skip if too short
		if len(strings.TrimSpace(text)) < 10 {
			return nil
		}

		// Add to memory
		metadata := Metadata{
			FilePath:   path,
			LineNumber: 0,
		}

		if err := m.Add(ctx, text, source, TypeContext, metadata); err != nil {
			return fmt.Errorf("failed to index %s: %w", path, err)
		}

		return nil
	})
}

// GetRelevantContext retrieves relevant context for a query.
// This is a convenience method for agent use.
func (m *Manager) GetRelevantContext(ctx context.Context, query string, maxResults int) (string, error) {
	opts := DefaultSearchOptions()
	if maxResults > 0 {
		opts.Limit = maxResults
	}

	results, err := m.Search(ctx, query, opts)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", nil
	}

	// Build context string
	var sb strings.Builder
	sb.WriteString("# Relevant Memory\n\n")

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("## Memory %d (score: %.2f)\n\n", i+1, result.Score))
		sb.WriteString(result.Text)
		sb.WriteString("\n\n")

		// Add metadata if available
		if result.Metadata.FilePath != "" {
			sb.WriteString(fmt.Sprintf("*Source: %s*\n\n", result.Metadata.FilePath))
		}

		sb.WriteString("---\n\n")
	}

	return sb.String(), nil
}

// Close closes the memory manager and saves any pending data.
func (m *Manager) Close() error {
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// Enable enables the memory system.
func (m *Manager) Enable() {
	m.enabled = true
}

// Disable disables the memory system.
func (m *Manager) Disable() {
	m.enabled = false
}

// IsEnabled returns whether the memory system is enabled.
func (m *Manager) IsEnabled() bool {
	return m.enabled
}
