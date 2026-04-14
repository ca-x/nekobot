package wiki

import "testing"

func TestParseSchemaExtractsTaxonomyAndThresholds(t *testing.T) {
	content := `# Wiki Schema

## Domain
- project: Nekobot

## Rules
- Every wiki page starts with YAML frontmatter.
- Use [[wikilinks]] to link between pages (minimum 2 outbound links per page)

## Tag Taxonomy
- memory
- research

## Page Thresholds
- Split page: exceeds ~200 lines
- Archive page: content fully superseded
`

	cfg, err := ParseSchema(content)
	if err != nil {
		t.Fatalf("ParseSchema failed: %v", err)
	}
	if cfg.Domain == "" {
		t.Fatal("expected domain to be populated")
	}
	if !cfg.IsValidTag("memory") {
		t.Fatal("expected memory tag to be valid")
	}
	if cfg.IsValidTag("unknown") {
		t.Fatal("expected unknown tag to be invalid")
	}
	if cfg.MinOutLinks != 2 {
		t.Fatalf("expected min outlinks 2, got %d", cfg.MinOutLinks)
	}
	if cfg.SplitLines != 200 {
		t.Fatalf("expected split lines 200, got %d", cfg.SplitLines)
	}
}
