package prompt

import (
	"strings"
	"testing"
)

type stubContextSource struct{}

func (stubContextSource) ReadWorkspaceMemory() string         { return "workspace note" }
func (stubContextSource) ReadLongTerm() string                { return "long-term note" }
func (stubContextSource) ReadActiveLearnings() string         { return "active learning" }
func (stubContextSource) GetRecentDailyNotes(days int) string { return "daily notes" }

func TestContextComposerBuildWrapsMemoryInFence(t *testing.T) {
	composer := NewContextComposer(stubContextSource{}, ContextOptions{
		IncludeWorkspaceMemory: true,
		IncludeLongTerm:        true,
		IncludeActiveLearnings: true,
		RecentDailyNoteDays:    1,
	})
	out := composer.Build()
	if !strings.Contains(out, "## Recalled Memory Context") {
		t.Fatalf("expected recalled memory heading, got: %s", out)
	}
	if !strings.Contains(out, "```memory") {
		t.Fatalf("expected memory fence, got: %s", out)
	}
	if !strings.Contains(out, "workspace note") || !strings.Contains(out, "long-term note") {
		t.Fatalf("expected memory content inside block, got: %s", out)
	}
}
