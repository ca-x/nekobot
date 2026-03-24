package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-kratos/blades"
	bladesmemory "github.com/go-kratos/blades/memory"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	qmdmemory "nekobot/pkg/memory/qmd"
)

// SearchManager is the unified semantic memory interface used by tools and agents.
type SearchManager interface {
	Search(ctx context.Context, query string, opts SearchOptions) ([]*SearchResult, error)
	Add(ctx context.Context, text string, source Source, typ Type, metadata Metadata) error
	Status() map[string]interface{}
	Close() error
	IsEnabled() bool
}

// QMDSearchManager uses QMD as the primary search backend and falls back to builtin memory.
type QMDSearchManager struct {
	log         *logger.Logger
	qmd         *qmdmemory.Manager
	fallback    *Manager
	useFallback bool
}

// NewSearchManagerFromConfig creates the appropriate semantic memory manager from config.
func NewSearchManagerFromConfig(log *logger.Logger, cfg *config.Config) (SearchManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	builtin, err := NewManagerFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	if !cfg.Memory.QMD.Enabled {
		return builtin, nil
	}

	qmdCfg := qmdmemory.ConfigFromConfig(cfg.Memory.QMD)
	qmdCfg.Sessions.SessionsDir = filepath.Join(cfg.WorkspacePath(), "sessions")
	qmdMgr := qmdmemory.NewManager(log, qmdCfg)

	searchMgr := &QMDSearchManager{
		log:      log,
		qmd:      qmdMgr,
		fallback: builtin,
	}

	if !qmdMgr.IsAvailable() {
		searchMgr.useFallback = true
		return searchMgr, nil
	}

	if err := qmdMgr.Initialize(context.Background(), cfg.WorkspacePath()); err != nil {
		log.Warn("QMD initialize failed, falling back to builtin memory")
		searchMgr.useFallback = true
		return searchMgr, nil
	}

	return searchMgr, nil
}

// Search searches QMD first and falls back to builtin memory when unavailable or empty.
func (m *QMDSearchManager) Search(ctx context.Context, query string, opts SearchOptions) ([]*SearchResult, error) {
	if m == nil {
		return nil, fmt.Errorf("search manager is nil")
	}
	if m.useFallback || m.qmd == nil || !m.qmd.IsAvailable() {
		return m.fallback.Search(ctx, query, opts)
	}

	results, err := m.searchQMD(ctx, query, opts)
	if err != nil {
		m.useFallback = true
		return m.fallback.Search(ctx, query, opts)
	}
	if len(results) == 0 {
		return m.fallback.Search(ctx, query, opts)
	}
	return results, nil
}

func (m *QMDSearchManager) searchQMD(ctx context.Context, query string, opts SearchOptions) ([]*SearchResult, error) {
	status := m.qmd.GetStatus()
	collections := status.Collections
	if len(collections) == 0 {
		return nil, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	results := make([]*SearchResult, 0)
	for _, coll := range collections {
		items, err := m.qmd.Search(ctx, coll.Name, query, limit)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.Score < opts.MinScore {
				continue
			}
			results = append(results, &SearchResult{
				Embedding: Embedding{
					Text: item.Snippet,
					Metadata: Metadata{
						FilePath: item.Path,
					},
					Source: SourceLongTerm,
					Type:   TypeContext,
				},
				Score:       item.Score,
				MatchedText: item.Snippet,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Add always writes to builtin memory so the fallback store remains useful.
func (m *QMDSearchManager) Add(ctx context.Context, text string, source Source, typ Type, metadata Metadata) error {
	if m == nil || m.fallback == nil {
		return fmt.Errorf("fallback memory manager is unavailable")
	}
	return m.fallback.Add(ctx, text, source, typ, metadata)
}

// Status reports active backend state.
func (m *QMDSearchManager) Status() map[string]interface{} {
	status := map[string]interface{}{
		"backend":          "qmd",
		"fallback_enabled": m != nil && m.useFallback,
	}
	if m == nil {
		return status
	}
	if m.qmd != nil {
		qmdStatus := m.qmd.GetStatus()
		status["available"] = qmdStatus.Available
		status["collections"] = len(qmdStatus.Collections)
		status["version"] = qmdStatus.Version
	}
	if m.fallback != nil {
		status["fallback_status"] = m.fallback.Status()
	}
	return status
}

// Close closes the fallback store.
func (m *QMDSearchManager) Close() error {
	if m == nil || m.fallback == nil {
		return nil
	}
	return m.fallback.Close()
}

// IsEnabled reports whether the search manager is active.
func (m *QMDSearchManager) IsEnabled() bool {
	if m == nil {
		return false
	}
	if m.useFallback || m.qmd == nil {
		return m.fallback != nil && m.fallback.IsEnabled()
	}
	return true
}

// BackendName returns the active backend label.
func (m *QMDSearchManager) BackendName() string {
	if m == nil {
		return "none"
	}
	if m.useFallback || m.qmd == nil || !m.qmd.IsAvailable() {
		return "builtin"
	}
	return "qmd"
}

func normalizePolicy(policy string) string {
	value := strings.TrimSpace(strings.ToLower(policy))
	if value == "" {
		return "hybrid"
	}
	return value
}

// BladesMemoryStoreAdapter adapts nekobot memory search to blades.MemoryStore.
type BladesMemoryStoreAdapter struct {
	manager SearchManager
	options SearchOptions
}

// NewBladesMemoryStoreAdapter creates a blades-compatible memory store adapter.
func NewBladesMemoryStoreAdapter(manager SearchManager, opts SearchOptions) *BladesMemoryStoreAdapter {
	if opts.Limit <= 0 {
		opts = DefaultSearchOptions()
	}
	return &BladesMemoryStoreAdapter{manager: manager, options: opts}
}

// AddMemory stores a blades memory record in the underlying fallback-capable manager.
func (a *BladesMemoryStoreAdapter) AddMemory(ctx context.Context, item *bladesmemory.Memory) error {
	if a == nil || a.manager == nil {
		return fmt.Errorf("memory manager not initialized")
	}
	if item == nil || item.Content == nil {
		return fmt.Errorf("memory content is required")
	}
	metadata := Metadata{}
	if path, ok := item.Metadata["file_path"].(string); ok {
		metadata.FilePath = path
	}
	return a.manager.Add(ctx, item.Content.Text(), SourceSession, TypeContext, metadata)
}

// SaveSession persists a blades session into memory entries.
func (a *BladesMemoryStoreAdapter) SaveSession(ctx context.Context, session blades.Session) error {
	if session == nil {
		return nil
	}
	for _, msg := range session.History() {
		if msg == nil || strings.TrimSpace(msg.Text()) == "" {
			continue
		}
		if err := a.AddMemory(ctx, &bladesmemory.Memory{Content: msg}); err != nil {
			return err
		}
	}
	return nil
}

// SearchMemory queries semantic memory and converts results into blades memory entries.
func (a *BladesMemoryStoreAdapter) SearchMemory(ctx context.Context, query string) ([]*bladesmemory.Memory, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("memory manager not initialized")
	}
	results, err := a.manager.Search(ctx, query, a.options)
	if err != nil {
		return nil, err
	}
	out := make([]*bladesmemory.Memory, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		metadata := map[string]any{}
		if result.Metadata.FilePath != "" {
			metadata["file_path"] = result.Metadata.FilePath
		}
		metadata["score"] = result.Score
		out = append(out, &bladesmemory.Memory{
			Content:  blades.AssistantMessage(result.Text),
			Metadata: metadata,
		})
	}
	return out, nil
}
