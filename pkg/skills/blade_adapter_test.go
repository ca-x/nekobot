package skills

import (
	"os"
	"path/filepath"
	"testing"

	bladeskills "github.com/go-kratos/blades/skills"
)

func TestBladesSkillAdapter_Name(t *testing.T) {
	tests := []struct {
		name     string
		skill    *Skill
		expected string
	}{
		{
			name:     "uses ID when available",
			skill:    &Skill{ID: "git-commit", Name: "Git Commit"},
			expected: "git-commit",
		},
		{
			name:     "falls back to Name when ID empty",
			skill:    &Skill{Name: "fallback-name"},
			expected: "fallback-name",
		},
		{
			name:     "nil skill returns empty",
			skill:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewBladesSkillAdapter(tt.skill)
			if got := adapter.Name(); got != tt.expected {
				t.Errorf("Name() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBladesSkillAdapter_Description(t *testing.T) {
	adapter := NewBladesSkillAdapter(&Skill{Description: "A helpful skill"})
	if got := adapter.Description(); got != "A helpful skill" {
		t.Errorf("Description() = %q, want %q", got, "A helpful skill")
	}

	nilAdapter := NewBladesSkillAdapter(nil)
	if got := nilAdapter.Description(); got != "" {
		t.Errorf("Description() on nil = %q, want empty", got)
	}
}

func TestBladesSkillAdapter_Instruction(t *testing.T) {
	adapter := NewBladesSkillAdapter(&Skill{Instructions: "Do the thing"})
	if got := adapter.Instruction(); got != "Do the thing" {
		t.Errorf("Instruction() = %q, want %q", got, "Do the thing")
	}

	nilAdapter := NewBladesSkillAdapter(nil)
	if got := nilAdapter.Instruction(); got != "" {
		t.Errorf("Instruction() on nil = %q, want empty", got)
	}
}

func TestBladesSkillAdapter_Frontmatter(t *testing.T) {
	skill := &Skill{
		ID:      "test-skill",
		Name:    "Test Skill",
		Version: "1.2.0",
		Author:  "tester",
		Tags:    []string{"go", "testing"},
		Enabled: true,
		Always:  false,
		Metadata: map[string]interface{}{
			"priority": 5,
			"category": "dev",
			"nested": map[string]interface{}{
				"key1": "val1",
				"key2": 42,
			},
		},
		Description: "A test skill",
	}

	adapter := NewBladesSkillAdapter(skill)
	fm := adapter.Frontmatter()

	if fm.Name != "test-skill" {
		t.Errorf("Frontmatter.Name = %q, want %q", fm.Name, "test-skill")
	}
	if fm.Description != "A test skill" {
		t.Errorf("Frontmatter.Description = %q, want %q", fm.Description, "A test skill")
	}

	// Check structured fields in metadata.
	checks := map[string]string{
		"display_name": "Test Skill",
		"version":      "1.2.0",
		"author":       "tester",
		"tags":         "go,testing",
		"enabled":      "true",
		"always":       "false",
		"priority":     "5",
		"category":     "dev",
		"nested.key1":  "val1",
		"nested.key2":  "42",
	}

	for key, expected := range checks {
		got, ok := fm.Metadata[key]
		if !ok {
			t.Errorf("Frontmatter.Metadata missing key %q", key)
			continue
		}
		if got != expected {
			t.Errorf("Frontmatter.Metadata[%q] = %q, want %q", key, got, expected)
		}
	}
}

func TestBladesSkillAdapter_Frontmatter_NilMetadata(t *testing.T) {
	skill := &Skill{
		ID:       "minimal-skill",
		Enabled:  false,
		Always:   true,
		Metadata: nil,
	}

	adapter := NewBladesSkillAdapter(skill)
	fm := adapter.Frontmatter()

	if fm.Metadata["enabled"] != "false" {
		t.Errorf("enabled = %q, want %q", fm.Metadata["enabled"], "false")
	}
	if fm.Metadata["always"] != "true" {
		t.Errorf("always = %q, want %q", fm.Metadata["always"], "true")
	}
}

func TestBladesSkillAdapter_Frontmatter_NilSkill(t *testing.T) {
	adapter := NewBladesSkillAdapter(nil)
	fm := adapter.Frontmatter()

	if fm.Name != "" || fm.Description != "" || len(fm.Metadata) != 0 {
		t.Errorf("Frontmatter on nil skill should be zero-like, got %+v", fm)
	}
}

func TestBladesSkillAdapter_Resources_BuiltinSkill(t *testing.T) {
	skill := &Skill{
		ID:       "builtin-skill",
		FilePath: "builtin://some-skill",
	}

	adapter := NewBladesSkillAdapter(skill)
	resources := adapter.Resources()

	if len(resources.References) != 0 {
		t.Errorf("builtin skill should have no references, got %d", len(resources.References))
	}
	if len(resources.Scripts) != 0 {
		t.Errorf("builtin skill should have no scripts, got %d", len(resources.Scripts))
	}
}

func TestBladesSkillAdapter_Resources_FilesystemSkill(t *testing.T) {
	// Create temp directory structure.
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "my-skill")
	refsDir := filepath.Join(skillDir, "references")
	scriptsDir := filepath.Join(skillDir, "scripts")

	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(refsDir, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "setup.sh"), []byte("#!/bin/bash\necho ok"), 0o755); err != nil {
		t.Fatal(err)
	}

	skill := &Skill{
		ID:       "my-skill",
		FilePath: filepath.Join(skillDir, "SKILL.md"),
	}

	adapter := NewBladesSkillAdapter(skill)
	resources := adapter.Resources()

	if content, ok := resources.References["guide.md"]; !ok || content != "# Guide" {
		t.Errorf("references[guide.md] = %q, ok=%v", content, ok)
	}
	if content, ok := resources.Scripts["setup.sh"]; !ok || content != "#!/bin/bash\necho ok" {
		t.Errorf("scripts[setup.sh] = %q, ok=%v", content, ok)
	}
}

func TestBladesSkillAdapter_Resources_NilSkill(t *testing.T) {
	adapter := NewBladesSkillAdapter(nil)
	resources := adapter.Resources()

	if len(resources.References) != 0 || len(resources.Scripts) != 0 || len(resources.Assets) != 0 {
		t.Errorf("nil skill should return empty resources")
	}
}

func TestBladesSkillAdapter_Resources_NoFilePath(t *testing.T) {
	adapter := NewBladesSkillAdapter(&Skill{ID: "no-path"})
	resources := adapter.Resources()

	if len(resources.References) != 0 {
		t.Errorf("skill with no filepath should return empty resources")
	}
}

func TestFlattenMap(t *testing.T) {
	src := map[string]interface{}{
		"simple": "value",
		"number": 42,
		"nested": map[string]interface{}{
			"deep": "inside",
			"deeper": map[string]interface{}{
				"level": 3,
			},
		},
	}

	dst := make(map[string]string)
	flattenMap("", src, dst)

	checks := map[string]string{
		"simple":             "value",
		"number":             "42",
		"nested.deep":        "inside",
		"nested.deeper.level": "3",
	}

	for key, expected := range checks {
		got, ok := dst[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if got != expected {
			t.Errorf("dst[%q] = %q, want %q", key, got, expected)
		}
	}
}

func TestFlattenMap_WithPrefix(t *testing.T) {
	src := map[string]interface{}{"key": "val"}
	dst := make(map[string]string)
	flattenMap("prefix", src, dst)

	if dst["prefix.key"] != "val" {
		t.Errorf("dst[prefix.key] = %q, want %q", dst["prefix.key"], "val")
	}
}

func TestFlattenMap_NilMap(t *testing.T) {
	dst := make(map[string]string)
	flattenMap("", nil, dst)

	if len(dst) != 0 {
		t.Errorf("flattenMap(nil) should produce no entries, got %d", len(dst))
	}
}

func TestBladesSkillAdapter_ImplementsInterfaces(t *testing.T) {
	adapter := NewBladesSkillAdapter(&Skill{ID: "test-skill", Description: "test"})

	var _ bladeskills.Skill = adapter
	var _ bladeskills.FrontmatterProvider = adapter
	var _ bladeskills.ResourcesProvider = adapter
}
