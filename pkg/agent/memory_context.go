package agent

import (
	"fmt"
	"strings"
)

// MemoryContextSource provides the read-side data needed to compose prompt memory context.
type MemoryContextSource interface {
	ReadWorkspaceMemory() string
	ReadLongTerm() string
	GetRecentDailyNotes(days int) string
}

// MemoryContextComposer assembles prompt-facing memory context from a read-only memory source.
type MemoryContextComposer struct {
	source  MemoryContextSource
	options MemoryContextOptions
}

// NewMemoryContextComposer creates a prompt memory composer for a source and options.
func NewMemoryContextComposer(source MemoryContextSource, options MemoryContextOptions) *MemoryContextComposer {
	return &MemoryContextComposer{
		source:  source,
		options: options,
	}
}

// Build renders the memory sections for prompt injection.
func (c *MemoryContextComposer) Build() string {
	if c == nil || c.source == nil {
		return ""
	}

	parts := make([]string, 0, 3)

	if c.options.IncludeWorkspaceMemory {
		workspaceMemory := strings.TrimSpace(c.source.ReadWorkspaceMemory())
		if workspaceMemory != "" {
			parts = append(parts, "## Workspace Memory\n\n"+workspaceMemory)
		}
	}

	if c.options.IncludeLongTerm {
		longTerm := strings.TrimSpace(c.source.ReadLongTerm())
		if longTerm != "" {
			parts = append(parts, "## Long-term Memory\n\n"+longTerm)
		}
	}

	if c.options.RecentDailyNoteDays > 0 {
		dailyNotes := strings.TrimSpace(c.source.GetRecentDailyNotes(c.options.RecentDailyNoteDays))
		if dailyNotes != "" {
			title := "## Today's Notes\n\n"
			if c.options.RecentDailyNoteDays > 1 {
				title = fmt.Sprintf("## Recent Daily Notes (Last %d Days)\n\n", c.options.RecentDailyNoteDays)
			}
			parts = append(parts, title+dailyNotes)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	joined := strings.Join(parts, "\n\n---\n\n")
	maxChars := c.options.MaxChars
	if maxChars <= 0 || len(joined) <= maxChars {
		return joined
	}
	if maxChars <= len("[Memory context truncated]") {
		return "[Memory context truncated]"
	}
	return strings.TrimSpace(joined[:maxChars-len("\n\n[Memory context truncated]")]) + "\n\n[Memory context truncated]"
}
