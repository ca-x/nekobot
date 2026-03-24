package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
)

func TestSkillToolGetIncludesStructuredMissingRequirements(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	content := `---
id: skill-report
name: Skill Report
enabled: true
requirements:
  config_paths:
    - channels.discord
  python_packages:
    - requests
  node_packages:
    - typescript
---

These are enough instructions to avoid the short warning threshold in tests.`
	if err := os.WriteFile(filepath.Join(skillsDir, "skill-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	manager := skills.NewManager(log, skillsDir, false)
	manager.SetPythonPackageInstalled(func(pkg string) bool {
		return false
	})
	manager.SetNodePackageInstalled(func(pkg string) bool {
		return false
	})
	manager.SetConfigPathExists(func(path string) bool {
		return false
	})
	if err := manager.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}

	tool := NewSkillTool(log, manager)
	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "get",
		"skill_id": "skill-report",
	})
	if err != nil {
		t.Fatalf("execute get: %v", err)
	}

	for _, fragment := range []string{
		"Python Packages: requests",
		"Node Packages: typescript",
		"Config Paths: channels.discord",
		"missing python packages: requests",
		"missing node packages: typescript",
		"missing config paths: channels.discord",
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, out)
		}
	}
}
