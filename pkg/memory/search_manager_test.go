package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kratos/blades"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	qmdmemory "nekobot/pkg/memory/qmd"
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

func TestConfigFromConfigWithWorkspaceResolvesSessionExportDir(t *testing.T) {
	workspaceDir := t.TempDir()
	resolved := qmdmemory.ConfigFromConfigWithWorkspace(config.QMDConfig{
		Enabled: true,
		Sessions: config.QMDSessionsConfig{
			Enabled:       true,
			ExportDir:     "${WORKSPACE}/memory/sessions",
			RetentionDays: 7,
		},
	}, workspaceDir)

	if resolved.Sessions.SessionsDir != filepath.Join(workspaceDir, "sessions") {
		t.Fatalf("unexpected sessions dir: %q", resolved.Sessions.SessionsDir)
	}
	if resolved.Sessions.ExportDir != filepath.Join(workspaceDir, "memory", "sessions") {
		t.Fatalf("unexpected export dir: %q", resolved.Sessions.ExportDir)
	}
}

func TestConfigFromConfigWithWorkspaceUsesDefaultSessionExportDir(t *testing.T) {
	workspaceDir := t.TempDir()
	resolved := qmdmemory.ConfigFromConfigWithWorkspace(config.QMDConfig{
		Enabled: true,
		Sessions: config.QMDSessionsConfig{
			Enabled:       true,
			RetentionDays: 7,
		},
	}, workspaceDir)

	if resolved.Sessions.ExportDir != filepath.Join(workspaceDir, "memory", "sessions") {
		t.Fatalf("unexpected default export dir: %q", resolved.Sessions.ExportDir)
	}
}

func TestManagerSearchDecoratesCitations(t *testing.T) {
	builtin, err := NewManager(t.TempDir()+"/embeddings.json", NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := builtin.Add(
		context.Background(),
		"release checklist",
		SourceLongTerm,
		TypeContext,
		Metadata{
			FilePath:      "/workspace/memory/release.md",
			LineNumber:    10,
			EndLineNumber: 14,
		},
	); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	results, err := builtin.Search(context.Background(), "release", SearchOptions{
		Limit:    3,
		MinScore: 0,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Citation != "/workspace/memory/release.md#L10-L14" {
		t.Fatalf("expected citation, got %+v", results[0])
	}
}

func TestManagerGetRelevantContextUsesCitation(t *testing.T) {
	builtin, err := NewManager(t.TempDir()+"/embeddings.json", NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := builtin.Add(
		context.Background(),
		"deploy checklist",
		SourceLongTerm,
		TypeContext,
		Metadata{
			FilePath:   "/workspace/memory/deploy.md",
			LineNumber: 22,
		},
	); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	contextText, err := builtin.GetRelevantContext(context.Background(), "deploy", 3)
	if err != nil {
		t.Fatalf("GetRelevantContext failed: %v", err)
	}
	if !strings.Contains(contextText, "*Source: /workspace/memory/deploy.md#L22*") {
		t.Fatalf("expected citation-formatted source, got %q", contextText)
	}
}

func TestApplyMMRPromotesDiversity(t *testing.T) {
	results := []*SearchResult{
		{Embedding: Embedding{ID: "a", Text: "deploy release checklist"}, Score: 0.99},
		{Embedding: Embedding{ID: "b", Text: "deploy release checklist again"}, Score: 0.98},
		{Embedding: Embedding{ID: "c", Text: "incident response handbook"}, Score: 0.80},
	}

	reordered := ApplyMMR(results, MMRConfig{Enabled: true, Lambda: 0.4})
	if len(reordered) != 3 {
		t.Fatalf("expected 3 reordered results, got %d", len(reordered))
	}
	if reordered[0].ID != "a" {
		t.Fatalf("expected highest relevance first, got %q", reordered[0].ID)
	}
	if reordered[1].ID != "c" {
		t.Fatalf("expected diverse result second, got %q", reordered[1].ID)
	}
}

func TestManagerSearchAppliesMMR(t *testing.T) {
	builtin, err := NewManager(t.TempDir()+"/embeddings.json", NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	for _, item := range []string{
		"deploy release checklist",
		"deploy release checklist again",
		"incident response handbook",
	} {
		if err := builtin.Add(context.Background(), item, SourceLongTerm, TypeContext, Metadata{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	results, err := builtin.Search(context.Background(), "deploy", SearchOptions{
		Limit:    3,
		MinScore: 0,
		MMR: &MMRConfig{
			Enabled: true,
			Lambda:  0.4,
		},
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !strings.Contains(results[0].Text, "deploy") {
		t.Fatalf("expected deploy result first, got %+v", results[0])
	}
	if !strings.Contains(results[1].Text, "incident response") {
		t.Fatalf("expected diverse result second, got %+v", results[1])
	}
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
