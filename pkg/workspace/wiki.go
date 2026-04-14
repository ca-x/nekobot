package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var wikiBootstrapFiles = map[string]string{
	filepath.Join("wiki", "SCHEMA.md"): `# Wiki Schema

## Domain
- project: Nekobot
- purpose: Structured LLM-facing knowledge base

## Conventions
- Every wiki page starts with YAML frontmatter.
- Every durable page should be listed in [[index]].
- Every mutation should append an entry to [[log]].
- Use [[wikilinks]] between related pages (minimum 2 outbound links per page when possible).

## Tag Taxonomy
- memory
- research
- project
- decision
- people
- company

## Page Thresholds
- Split page: exceeds ~200 lines
- Archive page: content fully superseded
`,
	filepath.Join("wiki", "index.md"): `# Wiki Index

_No wiki pages yet._
`,
	filepath.Join("wiki", "log.md"): `# Wiki Log

`,
}

var wikiSubdirs = []string{
	"wiki",
	filepath.Join("wiki", "raw"),
	filepath.Join("wiki", "entities"),
	filepath.Join("wiki", "concepts"),
	filepath.Join("wiki", "comparisons"),
	filepath.Join("wiki", "queries"),
	filepath.Join("wiki", "_archive"),
}

func (m *Manager) ensureWikiScaffold() error {
	for _, subdir := range wikiSubdirs {
		path := filepath.Join(m.workspaceDir, subdir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create wiki subdir %s: %w", subdir, err)
		}
	}
	for relPath, content := range wikiBootstrapFiles {
		targetPath := filepath.Join(m.workspaceDir, relPath)
		if _, err := os.Stat(targetPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat wiki bootstrap %s: %w", relPath, err)
		}
		if err := os.WriteFile(targetPath, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			return fmt.Errorf("write wiki bootstrap %s: %w", relPath, err)
		}
		m.log.Info(fmt.Sprintf("Created workspace wiki file: %s", relPath))
	}
	return nil
}
