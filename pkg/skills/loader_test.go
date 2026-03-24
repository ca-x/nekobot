package skills

import (
	"os"
	"path/filepath"
	"testing"

	"nekobot/pkg/logger"
)

func TestParseSkillContentAlwaysFromTopLevel(t *testing.T) {
	content := `---
id: top-level-always
name: top-level
always: true
---

Top-level always instructions`

	skill, err := parseSkillContent(content, "/tmp/top-level/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if !skill.Always {
		t.Fatalf("expected always=true from top-level frontmatter")
	}
}

func TestParseSkillContentAlwaysFromOpenclawMetadata(t *testing.T) {
	content := `---
id: metadata-always
name: metadata
metadata:
  openclaw:
    always: true
---

Metadata always instructions`

	skill, err := parseSkillContent(content, "/tmp/metadata/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if !skill.Always {
		t.Fatalf("expected always=true from metadata.openclaw.always")
	}
}

func TestParseSkillContentAlwaysDefaultsFalse(t *testing.T) {
	content := `---
id: no-always
name: no-always
---

No always instructions`

	skill, err := parseSkillContent(content, "/tmp/no-always/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if skill.Always {
		t.Fatalf("expected always=false when not configured")
	}
}

func TestLoadFromSourceLoadsDirectorySkillMarkdown(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	sourceRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "weather"), 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
id: weather
name: Weather
---

Weather instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "weather", "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	loader := &MultiPathLoader{
		log:     log,
		sources: nil,
	}
	loaded, err := loader.loadFromSource(SkillSource{
		Path: sourceRoot,
		Type: SourceLocal,
	})
	if err != nil {
		t.Fatalf("load from source: %v", err)
	}

	skill, ok := loaded["weather"]
	if !ok {
		t.Fatalf("expected directory skill to be discovered")
	}
	if want := filepath.Join(sourceRoot, "weather", "SKILL.md"); skill.FilePath != want {
		t.Fatalf("expected file path %q, got %q", want, skill.FilePath)
	}
}

func TestLoadFromSourceLoadsRootAndDirectorySkills(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	sourceRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "weather"), 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	rootSkill := `---
id: root-note
name: Root Note
---

Root instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "root-note.md"), []byte(rootSkill), 0644); err != nil {
		t.Fatalf("write root skill file: %v", err)
	}

	dirSkill := `---
id: weather
name: Weather
---

Weather instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "weather", "SKILL.md"), []byte(dirSkill), 0644); err != nil {
		t.Fatalf("write directory skill file: %v", err)
	}

	loader := &MultiPathLoader{
		log:     log,
		sources: nil,
	}
	loaded, err := loader.loadFromSource(SkillSource{
		Path: sourceRoot,
		Type: SourceWorkspace,
	})
	if err != nil {
		t.Fatalf("load from source: %v", err)
	}

	if _, ok := loaded["root-note"]; !ok {
		t.Fatalf("expected root markdown skill to be discovered")
	}
	if _, ok := loaded["weather"]; !ok {
		t.Fatalf("expected directory SKILL.md skill to be discovered")
	}
}
