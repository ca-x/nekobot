package memory

import "time"

// Source represents where a memory entry originates.
type Source string

const (
	// SourceLongTerm is from MEMORY.md and related files
	SourceLongTerm Source = "longterm"
	// SourceSession is from conversation history
	SourceSession Source = "session"
	// SourceDaily is from daily notes (YYYY-MM-DD.md)
	SourceDaily Source = "daily"
)

// Type represents the type of memory content.
type Type string

const (
	// TypeFact is a factual piece of information
	TypeFact Type = "fact"
	// TypePreference is user preference or setting
	TypePreference Type = "preference"
	// TypeContext is situational context
	TypeContext Type = "context"
	// TypeConversation is conversation summary
	TypeConversation Type = "conversation"
)

// Embedding represents a vector embedding with metadata.
type Embedding struct {
	ID        string    `json:"id"`
	Vector    []float32 `json:"vector"`
	Dimension int       `json:"dimension"`
	Text      string    `json:"text"`
	Source    Source    `json:"source"`
	Type      Type      `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  Metadata  `json:"metadata"`
}

// Metadata contains additional information about a memory.
type Metadata struct {
	// FilePath is the source file path (if from file)
	FilePath string `json:"file_path,omitempty"`
	// LineNumber is the line in the source file
	LineNumber int `json:"line_number,omitempty"`
	// SessionKey is the session identifier (if from session)
	SessionKey string `json:"session_key,omitempty"`
	// Tags are user-defined tags
	Tags []string `json:"tags,omitempty"`
	// Importance is a user-assigned importance score (0-1)
	Importance float64 `json:"importance,omitempty"`
	// AccessCount tracks how often this memory was accessed
	AccessCount int `json:"access_count,omitempty"`
	// LastAccessed is when this memory was last retrieved
	LastAccessed time.Time `json:"last_accessed,omitempty"`
}

// SearchResult represents a memory search result with relevance score.
type SearchResult struct {
	Embedding
	Score       float64 `json:"score"`
	MatchedText string  `json:"matched_text"`
	Highlight   string  `json:"highlight,omitempty"`
}

// SearchOptions configures memory search behavior.
type SearchOptions struct {
	// Limit is the maximum number of results to return
	Limit int `json:"limit"`
	// MinScore is the minimum similarity score (0-1)
	MinScore float64 `json:"min_score"`
	// Sources filters which memory sources to search
	Sources []Source `json:"sources,omitempty"`
	// Types filters which memory types to search
	Types []Type `json:"types,omitempty"`
	// Hybrid enables hybrid vector+keyword search
	Hybrid bool `json:"hybrid"`
	// VectorWeight is the weight for vector similarity in hybrid search (0-1)
	VectorWeight float64 `json:"vector_weight"`
	// TextWeight is the weight for keyword match in hybrid search (0-1)
	TextWeight float64 `json:"text_weight"`
}

// DefaultSearchOptions returns sensible default search options.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit:        10,
		MinScore:     0.7,
		Sources:      nil, // All sources
		Types:        nil, // All types
		Hybrid:       true,
		VectorWeight: 0.7,
		TextWeight:   0.3,
	}
}

// EmbeddingProvider defines the interface for generating embeddings.
type EmbeddingProvider interface {
	// Embed generates a single embedding
	Embed(text string) ([]float32, error)
	// EmbedBatch generates multiple embeddings in one call
	EmbedBatch(texts []string) ([][]float32, error)
	// Dimension returns the dimension of embeddings
	Dimension() int
	// MaxBatchSize returns the maximum batch size
	MaxBatchSize() int
}

// Store defines the interface for memory storage.
type Store interface {
	// Add adds a memory to the store
	Add(embedding *Embedding) error
	// AddBatch adds multiple memories in one transaction
	AddBatch(embeddings []*Embedding) error
	// Search performs similarity search
	Search(query []float32, opts SearchOptions) ([]*SearchResult, error)
	// Get retrieves a memory by ID
	Get(id string) (*Embedding, error)
	// Delete removes a memory by ID
	Delete(id string) error
	// Update updates an existing memory
	Update(embedding *Embedding) error
	// List lists all memories with optional filtering
	List(filter func(*Embedding) bool) ([]*Embedding, error)
	// Close closes the store
	Close() error
}
