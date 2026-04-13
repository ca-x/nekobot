package wiki

import "time"

// PageType identifies the kind of wiki page.
type PageType string

const (
	PageTypeEntity     PageType = "entity"
	PageTypeConcept    PageType = "concept"
	PageTypeComparison PageType = "comparison"
	PageTypeQuery      PageType = "query"
	PageTypeSummary    PageType = "summary"
)

// Page stores one wiki page with frontmatter and markdown body.
type Page struct {
	Title          string    `yaml:"title"`
	Created        time.Time `yaml:"created"`
	Updated        time.Time `yaml:"updated"`
	Type           PageType  `yaml:"type"`
	Tags           []string  `yaml:"tags"`
	Sources        []string  `yaml:"sources"`
	Contradictions []string  `yaml:"contradictions,omitempty"`

	Body string `yaml:"-"`

	FilePath string   `yaml:"-"`
	OutLinks []string `yaml:"-"`
}

// IndexEntry represents one entry in wiki/index.md.
type IndexEntry struct {
	Title   string
	Path    string
	Type    PageType
	Summary string
	Updated time.Time
}

// LogEntry represents one append-only wiki log event.
type LogEntry struct {
	Date    time.Time
	Action  string
	Subject string
	Details []string
}

// SearchResult represents one wiki query result.
type SearchResult struct {
	Page    Page
	Summary string
	Score   int
}
