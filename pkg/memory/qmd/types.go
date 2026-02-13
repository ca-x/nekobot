// Package qmd provides QMD (Query Markdown) integration for semantic search.
package qmd

import (
	"time"
)

// Config defines QMD configuration.
type Config struct {
	// Enabled indicates if QMD integration is active.
	Enabled bool

	// Command is the QMD executable name or path.
	Command string

	// IncludeDefault includes the default workspace memory collection.
	IncludeDefault bool

	// Paths defines custom collections to index.
	Paths []CollectionPath

	// Sessions configuration for session export.
	Sessions SessionsConfig

	// Update configuration for automatic updates.
	Update UpdateConfig
}

// CollectionPath defines a collection to index.
type CollectionPath struct {
	// Name is the collection identifier.
	Name string

	// Path is the directory to index.
	Path string

	// Pattern is the glob pattern for matching files (e.g., "**/*.md").
	Pattern string
}

// SessionsConfig defines session export configuration.
type SessionsConfig struct {
	// Enabled indicates if session export is active.
	Enabled bool

	// ExportDir is where exported session markdown files are stored.
	ExportDir string

	// RetentionDays is how long to keep exported sessions (0 = forever).
	RetentionDays int
}

// UpdateConfig defines automatic update configuration.
type UpdateConfig struct {
	// OnBoot updates all collections on agent start.
	OnBoot bool

	// Interval is the duration between scheduled updates (e.g., "30m").
	Interval string

	// CommandTimeout is the timeout for individual QMD commands.
	CommandTimeout string

	// UpdateTimeout is the timeout for full update operations.
	UpdateTimeout string
}

// Collection represents a QMD collection.
type Collection struct {
	// Name is the collection identifier.
	Name string

	// Path is the directory being indexed.
	Path string

	// Pattern is the file matching pattern.
	Pattern string

	// DocumentCount is the number of indexed documents.
	DocumentCount int

	// LastUpdated is when the collection was last updated.
	LastUpdated time.Time
}

// SearchResult represents a search result from QMD.
type SearchResult struct {
	// Path is the file path.
	Path string

	// Score is the relevance score (0-1).
	Score float64

	// Snippet is a text snippet from the document.
	Snippet string

	// Metadata contains additional information.
	Metadata map[string]string
}

// Status represents QMD system status.
type Status struct {
	// Available indicates if QMD is installed and accessible.
	Available bool

	// Version is the QMD version string.
	Version string

	// Collections is the list of active collections.
	Collections []Collection

	// LastUpdate is when collections were last updated.
	LastUpdate time.Time

	// Error contains any error message.
	Error string
}
