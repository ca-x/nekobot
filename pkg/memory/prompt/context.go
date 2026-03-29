package prompt

import (
	"fmt"
	"strings"
)

// ContextSource provides the read-side data needed to compose prompt memory context.
type ContextSource interface {
	ReadWorkspaceMemory() string
	ReadLongTerm() string
	ReadActiveLearnings() string
	GetRecentDailyNotes(days int) string
}

// ContextComposer assembles prompt-facing memory context from a read-only memory source.
type ContextComposer struct {
	source  ContextSource
	options ContextOptions
}

// NewContextComposer creates a prompt memory composer for a source and options.
func NewContextComposer(source ContextSource, options ContextOptions) *ContextComposer {
	return &ContextComposer{
		source:  source,
		options: options,
	}
}

// Build renders the memory sections for prompt injection.
func (c *ContextComposer) Build() string {
	if c == nil || c.source == nil {
		return ""
	}

	parts := make([]string, 0, 4)

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

	if c.options.IncludeActiveLearnings {
		activeLearnings := strings.TrimSpace(c.source.ReadActiveLearnings())
		if activeLearnings != "" {
			if !strings.HasPrefix(activeLearnings, "## ") {
				activeLearnings = "## Active Learnings\n\n" + activeLearnings
			}
			parts = append(parts, activeLearnings)
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
