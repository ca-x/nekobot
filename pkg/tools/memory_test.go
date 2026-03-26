package tools

import (
	"context"
	"strings"
	"testing"

	"nekobot/pkg/logger"
	"nekobot/pkg/memory"
)

func TestMemoryToolSearchUsesConfiguredSemanticOptions(t *testing.T) {
	mgr, err := memory.NewManager(t.TempDir()+"/embeddings.json", memory.NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Add(context.Background(), "alpha release note", memory.SourceLongTerm, memory.TypeContext, memory.Metadata{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := mgr.Add(context.Background(), "beta deployment checklist", memory.SourceLongTerm, memory.TypeContext, memory.Metadata{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	tool := NewMemoryTool(newToolsTestLogger(t), mgr, MemoryToolOptions{
		DefaultTopK:   1,
		MaxTopK:       2,
		SearchPolicy:  "vector",
		IncludeScores: true,
	})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "search",
		"query":       "release",
		"max_results": float64(5),
		"min_score":   float64(0),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.Count(out, "## Memory ") != 2 {
		t.Fatalf("expected max_results to be capped at 2, got %q", out)
	}
	if !strings.Contains(out, "score:") {
		t.Fatalf("expected scores included, got %q", out)
	}
}

func TestMemoryToolSearchFormatsCitationSource(t *testing.T) {
	mgr, err := memory.NewManager(t.TempDir()+"/embeddings.json", memory.NewSimpleEmbeddingProvider(16))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Add(
		context.Background(),
		"release note",
		memory.SourceLongTerm,
		memory.TypeContext,
		memory.Metadata{
			FilePath:      "/workspace/memory/release.md",
			LineNumber:    7,
			EndLineNumber: 9,
		},
	); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	tool := NewMemoryTool(newToolsTestLogger(t), mgr, MemoryToolOptions{
		DefaultTopK:   1,
		MaxTopK:       1,
		SearchPolicy:  "hybrid",
		IncludeScores: false,
	})

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "search",
		"query":     "release",
		"min_score": float64(0),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "*Source: /workspace/memory/release.md#L7-L9*") {
		t.Fatalf("expected citation-formatted source, got %q", out)
	}
}

func newToolsTestLogger(t *testing.T) *logger.Logger {
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
