package skills

import "testing"

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
