package memory

import (
	"context"
	"testing"

	"github.com/go-kratos/blades"
	"nekobot/pkg/logger"
)

func TestQMDSearchManagerFallsBackToBuiltin(t *testing.T) {
	builtin, err := NewManager(t.TempDir()+"/embeddings.json", NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := builtin.Add(context.Background(), "alpha note", SourceLongTerm, TypeContext, Metadata{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	mgr := &QMDSearchManager{
		log:         newSearchMgrTestLogger(t),
		fallback:    builtin,
		useFallback: true,
	}

	results, err := mgr.Search(context.Background(), "alpha", DefaultSearchOptions())
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected fallback search results")
	}
	if results[0].Text == "" {
		t.Fatalf("expected result text, got %+v", results[0])
	}
}

func TestBladesMemoryStoreAdapterSearchMemory(t *testing.T) {
	builtin, err := NewManager(t.TempDir()+"/embeddings.json", NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := builtin.Add(context.Background(), "release checklist", SourceLongTerm, TypeContext, Metadata{FilePath: "/tmp/release.md"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	adapter := NewBladesMemoryStoreAdapter(builtin, SearchOptions{Limit: 3, MinScore: 0})
	memories, err := adapter.SearchMemory(context.Background(), "release")
	if err != nil {
		t.Fatalf("SearchMemory failed: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("expected memories")
	}
	if memories[0].Content == nil || memories[0].Content.Text() == "" {
		t.Fatalf("expected blades message content, got %+v", memories[0])
	}
	if got, _ := memories[0].Metadata["file_path"].(string); got != "/tmp/release.md" {
		t.Fatalf("expected file path metadata, got %+v", memories[0].Metadata)
	}
	if _, ok := memories[0].Metadata["score"]; !ok {
		t.Fatalf("expected score metadata, got %+v", memories[0].Metadata)
	}
	_ = blades.AssistantMessage
}

func newSearchMgrTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}
