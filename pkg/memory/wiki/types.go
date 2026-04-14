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
	Aliases        []string  `yaml:"aliases,omitempty"`
	Confidence     string    `yaml:"confidence,omitempty"`
	Summary        string    `yaml:"summary,omitempty"`
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

// QueryOptions narrows wiki search.
type QueryOptions struct {
	Type PageType
	Tag  string
}

// WikiConfig stores schema conventions loaded from SCHEMA.md.
type WikiConfig struct {
	Domain        string
	TagTaxonomy   []string
	MinOutLinks   int
	SplitLines    int
	ArchivePolicy string
}

// LintResult summarizes wiki health issues.
type LintResult struct {
	BrokenLinks    []LinkIssue
	MissingIndex   []string
	OversizedPages []string
	OrphanPages    []string
	TagViolations  []TagViolation
	TotalIssues    int
}

// LinkIssue records a broken wikilink.
type LinkIssue struct {
	SourcePage string
	TargetLink string
}

// TagViolation records invalid page tags.
type TagViolation struct {
	Page string
	Tags []string
}
