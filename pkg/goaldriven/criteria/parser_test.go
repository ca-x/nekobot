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

func TestParseGeneratesHTTPStatusCriterion(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	set, err := parser.Parse(t.Context(), ParseInput{
		Goal:      "verify health endpoint",
		Natural:   "ensure url https://example.com/health returns 200",
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
	if item.Type != TypeHTTPCheck {
		t.Fatalf("expected http_check criterion, got %q", item.Type)
	}
	if targetURL, _ := item.Definition["url"].(string); targetURL != "https://example.com/health" {
		t.Fatalf("unexpected url definition: %+v", item.Definition)
	}
	if statusCode, _ := item.Definition["expect_status"].(int); statusCode != 200 {
		t.Fatalf("unexpected status definition: %+v", item.Definition)
	}
}

func TestParseGeneratesHTTPBodyCriterion(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	set, err := parser.Parse(t.Context(), ParseInput{
		Goal:      "verify status page",
		Natural:   "ensure url https://example.com/status contains READY",
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
	if item.Type != TypeHTTPCheck {
		t.Fatalf("expected http_check criterion, got %q", item.Type)
	}
	if bodyContains, _ := item.Definition["body_contains"].(string); bodyContains != "READY" {
		t.Fatalf("unexpected body_contains definition: %+v", item.Definition)
	}
}

func TestSchemaRejectsLocalHTTPCheckTarget(t *testing.T) {
	t.Parallel()

	schema := NewSchema()
	err := schema.Validate(Set{
		Criteria: []Item{
			{
				ID:       "http-1",
				Title:    "local check",
				Type:     TypeHTTPCheck,
				Scope:    shared.ExecutionScope{Kind: shared.ScopeServer, Source: "manual"},
				Required: true,
				Definition: map[string]any{
					"url":           "http://127.0.0.1:8080/health",
					"expect_status": 200,
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected localhost/private URL to be rejected")
	}
}

func TestSchemaRejectsMetadataLikeHTTPHost(t *testing.T) {
	t.Parallel()

	schema := NewSchema()
	err := schema.Validate(Set{
		Criteria: []Item{
			{
				ID:       "http-1",
				Title:    "metadata host",
				Type:     TypeHTTPCheck,
				Scope:    shared.ExecutionScope{Kind: shared.ScopeServer, Source: "manual"},
				Required: true,
				Definition: map[string]any{
					"url":           "http://metadata.google.internal/computeMetadata/v1/",
					"expect_status": 200,
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected metadata-like hostname to be rejected")
	}
}
