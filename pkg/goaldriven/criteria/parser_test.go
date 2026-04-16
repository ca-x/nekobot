package criteria

import (
	"testing"

	"nekobot/pkg/goaldriven/shared"
)

func TestParseGeneratesFileExistsCriterion(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	set, err := parser.Parse(t.Context(), ParseInput{
		Goal:      "verify artifact",
		Natural:   "ensure file /tmp/result.txt exists",
		Scope:     &shared.ExecutionScope{Kind: shared.ScopeServer, Source: "manual"},
		RiskLevel: shared.RiskBalanced,
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(set.Criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %+v", set.Criteria)
	}
	item := set.Criteria[0]
	if item.Type != TypeFileExists {
		t.Fatalf("expected file_exists criterion, got %q", item.Type)
	}
	if path, _ := item.Definition["path"].(string); path != "/tmp/result.txt" {
		t.Fatalf("expected /tmp/result.txt, got %+v", item.Definition)
	}
}

func TestParseGeneratesFileContainsCriterion(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	set, err := parser.Parse(t.Context(), ParseInput{
		Goal:      "verify report content",
		Natural:   "ensure file /tmp/report.txt contains SUCCESS",
		Scope:     &shared.ExecutionScope{Kind: shared.ScopeServer, Source: "manual"},
		RiskLevel: shared.RiskBalanced,
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(set.Criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %+v", set.Criteria)
	}
	item := set.Criteria[0]
	if item.Type != TypeFileContains {
		t.Fatalf("expected file_contains criterion, got %q", item.Type)
	}
	if path, _ := item.Definition["path"].(string); path != "/tmp/report.txt" {
		t.Fatalf("expected /tmp/report.txt, got %+v", item.Definition)
	}
	if needle, _ := item.Definition["contains"].(string); needle != "SUCCESS" {
		t.Fatalf("expected SUCCESS needle, got %+v", item.Definition)
	}
}

func TestParseDoesNotInferCommandInsideFileContainsAssertion(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	set, err := parser.Parse(t.Context(), ParseInput{
		Goal:      "verify report content",
		Natural:   "ensure file /tmp/report.txt contains go build ./cmd/nekobot",
		Scope:     &shared.ExecutionScope{Kind: shared.ScopeServer, Source: "manual"},
		RiskLevel: shared.RiskBalanced,
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(set.Criteria) != 1 {
		t.Fatalf("expected exactly one criterion, got %+v", set.Criteria)
	}
	item := set.Criteria[0]
	if item.Type != TypeFileContains {
		t.Fatalf("expected file_contains only, got %q", item.Type)
	}
	if needle, _ := item.Definition["contains"].(string); needle != "go build ./cmd/nekobot" {
		t.Fatalf("expected file_contains needle to keep full text, got %+v", item.Definition)
	}
}
