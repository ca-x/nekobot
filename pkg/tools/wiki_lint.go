package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/memory/wiki"
)

// WikiLintTool runs health checks over workspace/wiki.
type WikiLintTool struct {
	manager *wiki.LintManager
}

// NewWikiLintTool creates a wiki lint tool for one workspace.
func NewWikiLintTool(workspace string) *WikiLintTool {
	return &WikiLintTool{
		manager: wiki.NewLintManager(wiki.DefaultWikiDir(workspace)),
	}
}

// Name returns the tool name.
func (t *WikiLintTool) Name() string {
	return "wiki_lint"
}

// Description returns the tool description.
func (t *WikiLintTool) Description() string {
	return "Run health checks on the structured LLM Wiki knowledge base."
}

// Parameters returns the tool parameter schema.
func (t *WikiLintTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute runs the wiki lint tool.
func (t *WikiLintTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	_ = ctx
	_ = args
	if t == nil || t.manager == nil {
		return "", fmt.Errorf("wiki lint tool not initialized")
	}
	result, err := t.manager.Run()
	if err != nil {
		return "", fmt.Errorf("wiki lint: %w", err)
	}
	if result.TotalIssues == 0 {
		return "Wiki lint passed: no issues found.", nil
	}

	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "Wiki lint found %d issue(s).\n", result.TotalIssues)
	for _, issue := range result.BrokenLinks {
		_, _ = fmt.Fprintf(&b, "- Broken link: %s -> %s\n", issue.SourcePage, issue.TargetLink)
	}
	for _, page := range result.OrphanPages {
		_, _ = fmt.Fprintf(&b, "- Too few links: %s\n", page)
	}
	for _, page := range result.OversizedPages {
		_, _ = fmt.Fprintf(&b, "- Oversized page: %s\n", page)
	}
	for _, violation := range result.TagViolations {
		_, _ = fmt.Fprintf(&b, "- Invalid tags on %s: %s\n", violation.Page, strings.Join(violation.Tags, ", "))
	}
	return strings.TrimSpace(b.String()), nil
}
