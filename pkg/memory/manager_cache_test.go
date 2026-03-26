package memory

import (
	"context"
	"sync"
	"testing"
)

type countingEmbeddingProvider struct {
	mu        sync.Mutex
	dimension int
	counts    map[string]int
}

func newCountingEmbeddingProvider(dimension int) *countingEmbeddingProvider {
	return &countingEmbeddingProvider{
		dimension: dimension,
		counts:    make(map[string]int),
	}
}

func (p *countingEmbeddingProvider) Embed(text string) ([]float32, error) {
	p.mu.Lock()
	p.counts[text]++
	p.mu.Unlock()

	vector := make([]float32, p.dimension)
	for i := 0; i < len(text) && i < p.dimension; i++ {
		vector[i] = float32(text[i]) / 255.0
	}
	return vector, nil
}

func (p *countingEmbeddingProvider) EmbedBatch(texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vector, err := p.Embed(text)
		if err != nil {
			return nil, err
		}
		results = append(results, vector)
	}
	return results, nil
}

func (p *countingEmbeddingProvider) Dimension() int {
	return p.dimension
}

func (p *countingEmbeddingProvider) MaxBatchSize() int {
	return 100
}

func (p *countingEmbeddingProvider) Count(text string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.counts[text]
}

func TestManagerSearchCachesQueryEmbeddings(t *testing.T) {
	provider := newCountingEmbeddingProvider(16)
	manager, err := NewManager(t.TempDir()+"/embeddings.json", provider)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if err := manager.Add(context.Background(), "deploy checklist", SourceLongTerm, TypeContext, Metadata{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	searchOpts := DefaultSearchOptions()
	searchOpts.MinScore = 0

	for range 2 {
		results, searchErr := manager.Search(context.Background(), "deploy", searchOpts)
		if searchErr != nil {
			t.Fatalf("Search failed: %v", searchErr)
		}
		if len(results) == 0 {
			t.Fatal("expected search results")
		}
	}

	if got := provider.Count("deploy"); got != 1 {
		t.Fatalf("expected cached query embedding to avoid duplicate embeds, got %d", got)
	}
}

func TestManagerAddCachesEmbeddingsForRepeatedText(t *testing.T) {
	provider := newCountingEmbeddingProvider(16)
	manager, err := NewManager(t.TempDir()+"/embeddings.json", provider)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	for range 2 {
		if err := manager.Add(context.Background(), "repeat me", SourceSession, TypeContext, Metadata{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	if got := provider.Count("repeat me"); got != 1 {
		t.Fatalf("expected cached add embedding to avoid duplicate embeds, got %d", got)
	}
}
