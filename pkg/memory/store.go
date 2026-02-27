package memory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"nekobot/pkg/fileutil"
)

// FileStore implements an in-memory vector store with file persistence.
type FileStore struct {
	mu         sync.RWMutex
	embeddings map[string]*Embedding
	filePath   string
	autoSave   bool
}

// NewFileStore creates a new file-based vector store.
func NewFileStore(filePath string, autoSave bool) (*FileStore, error) {
	store := &FileStore{
		embeddings: make(map[string]*Embedding),
		filePath:   filePath,
		autoSave:   autoSave,
	}

	// Load existing data if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load store: %w", err)
		}
	}

	return store, nil
}

// Add adds a single embedding to the store.
func (s *FileStore) Add(emb *Embedding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if emb.ID == "" {
		emb.ID = uuid.New().String()
	}

	if emb.CreatedAt.IsZero() {
		emb.CreatedAt = time.Now()
	}
	emb.UpdatedAt = time.Now()

	s.embeddings[emb.ID] = emb

	if s.autoSave {
		return s.saveUnsafe()
	}

	return nil
}

// AddBatch adds multiple embeddings in one operation.
func (s *FileStore) AddBatch(embeddings []*Embedding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, emb := range embeddings {
		if emb.ID == "" {
			emb.ID = uuid.New().String()
		}
		if emb.CreatedAt.IsZero() {
			emb.CreatedAt = time.Now()
		}
		emb.UpdatedAt = time.Now()

		s.embeddings[emb.ID] = emb
	}

	if s.autoSave {
		return s.saveUnsafe()
	}

	return nil
}

// Search performs vector similarity search.
func (s *FileStore) Search(query []float32, opts SearchOptions) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*SearchResult

	for _, emb := range s.embeddings {
		// Apply source filter
		if len(opts.Sources) > 0 && !contains(opts.Sources, emb.Source) {
			continue
		}

		// Apply type filter
		if len(opts.Types) > 0 && !containsType(opts.Types, emb.Type) {
			continue
		}

		// Calculate similarity
		var score float64
		if opts.Hybrid {
			// Hybrid search: combine vector and text similarity
			vectorSim := cosineSimilarity(query, emb.Vector)
			textSim := 0.0 // TODO: implement keyword matching

			score = opts.VectorWeight*vectorSim + opts.TextWeight*textSim
		} else {
			// Pure vector search
			score = cosineSimilarity(query, emb.Vector)
		}

		// Apply minimum score filter
		if score < opts.MinScore {
			continue
		}

		results = append(results, &SearchResult{
			Embedding:   *emb,
			Score:       score,
			MatchedText: emb.Text,
		})
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// Get retrieves an embedding by ID.
func (s *FileStore) Get(id string) (*Embedding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	emb, exists := s.embeddings[id]
	if !exists {
		return nil, fmt.Errorf("embedding not found: %s", id)
	}

	// Update access tracking
	embCopy := *emb
	embCopy.Metadata.AccessCount++
	embCopy.Metadata.LastAccessed = time.Now()

	return &embCopy, nil
}

// Delete removes an embedding by ID.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.embeddings, id)

	if s.autoSave {
		return s.saveUnsafe()
	}

	return nil
}

// Update updates an existing embedding.
func (s *FileStore) Update(emb *Embedding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.embeddings[emb.ID]; !exists {
		return fmt.Errorf("embedding not found: %s", emb.ID)
	}

	emb.UpdatedAt = time.Now()
	s.embeddings[emb.ID] = emb

	if s.autoSave {
		return s.saveUnsafe()
	}

	return nil
}

// List lists all embeddings with optional filtering.
func (s *FileStore) List(filter func(*Embedding) bool) ([]*Embedding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Embedding
	for _, emb := range s.embeddings {
		if filter == nil || filter(emb) {
			results = append(results, emb)
		}
	}

	return results, nil
}

// Save manually saves the store to disk.
func (s *FileStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnsafe()
}

// saveUnsafe saves without locking (caller must hold lock).
func (s *FileStore) saveUnsafe() error {
	// Convert to slice for JSON
	embSlice := make([]*Embedding, 0, len(s.embeddings))
	for _, emb := range s.embeddings {
		embSlice = append(embSlice, emb)
	}

	data, err := json.MarshalIndent(embSlice, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal embeddings: %w", err)
	}

	if err := fileutil.WriteFileAtomic(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write embeddings file: %w", err)
	}

	return nil
}

// load loads embeddings from disk.
func (s *FileStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var embSlice []*Embedding
	if err := json.Unmarshal(data, &embSlice); err != nil {
		return fmt.Errorf("failed to unmarshal embeddings: %w", err)
	}

	// Convert to map
	s.embeddings = make(map[string]*Embedding)
	for _, emb := range embSlice {
		s.embeddings[emb.ID] = emb
	}

	return nil
}

// Close closes the store and saves if needed.
func (s *FileStore) Close() error {
	if s.autoSave {
		return s.Save()
	}
	return nil
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// contains checks if a Source is in a slice.
func contains(sources []Source, source Source) bool {
	for _, s := range sources {
		if s == source {
			return true
		}
	}
	return false
}

// containsType checks if a Type is in a slice.
func containsType(types []Type, typ Type) bool {
	for _, t := range types {
		if t == typ {
			return true
		}
	}
	return false
}

// textSimilarity calculates simple keyword-based similarity (for hybrid search).
func textSimilarity(query, text string) float64 {
	queryWords := strings.Fields(strings.ToLower(query))
	textWords := strings.Fields(strings.ToLower(text))

	if len(queryWords) == 0 || len(textWords) == 0 {
		return 0
	}

	matches := 0
	for _, qw := range queryWords {
		for _, tw := range textWords {
			if qw == tw || strings.Contains(tw, qw) || strings.Contains(qw, tw) {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(queryWords))
}
