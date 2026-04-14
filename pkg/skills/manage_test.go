package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/logger"
)

func TestSaveWorkspaceSkillCreatesAndLoadsSkill(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	root := t.TempDir()
	mgr := NewManager(log, filepath.Join(root, "skills"), false)
	content := `---
id: reusable-flow
name: Reusable Flow
description: Captured workflow
---

Use this skill when the same workflow repeats.`

	skill, created, err := mgr.SaveWorkspaceSkill(content)
	if err != nil {
		t.Fatalf("save workspace skill: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if skill.ID != "reusable-flow" {
		t.Fatalf("unexpected skill id: %q", skill.ID)
	}
	if !strings.HasSuffix(skill.FilePath, filepath.Join("reusable-flow", skillFileName)) {
		t.Fatalf("unexpected skill path: %q", skill.FilePath)
	}
	loaded, err := mgr.Get("reusable-flow")
	if err != nil {
		t.Fatalf("get saved skill: %v", err)
	}
	if strings.TrimSpace(loaded.Instructions) != "Use this skill when the same workflow repeats." {
		t.Fatalf("unexpected instructions: %q", loaded.Instructions)
	}
}

func TestDeleteWorkspaceSkillRemovesSkill(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	root := t.TempDir()
	mgr := NewManager(log, filepath.Join(root, "skills"), false)
	content := `---
id: removable-skill
name: Removable Skill
---

Delete me.`
	if _, _, err := mgr.SaveWorkspaceSkill(content); err != nil {
		t.Fatalf("save workspace skill: %v", err)
	}
	if err := mgr.DeleteWorkspaceSkill("removable-skill"); err != nil {
		t.Fatalf("delete workspace skill: %v", err)
	}
	if _, err := mgr.Get("removable-skill"); err == nil {
		t.Fatal("expected deleted skill to be absent")
	}
}

func TestPatchWriteAndRemoveWorkspaceSkillFile(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	root := t.TempDir()
	mgr := NewManager(log, filepath.Join(root, "skills"), false)
	content := `---
id: patchable-skill
name: Patchable Skill
---

Original instructions.`
	if _, _, err := mgr.SaveWorkspaceSkill(content); err != nil {
		t.Fatalf("save workspace skill: %v", err)
	}
	if err := mgr.PatchWorkspaceSkillFile("patchable-skill", "", "Original", "Updated", false); err != nil {
		t.Fatalf("patch workspace skill: %v", err)
	}
	body, err := mgr.ReadSkillFile("patchable-skill", "")
	if err != nil {
		t.Fatalf("read skill body: %v", err)
	}
	if !strings.Contains(body, "Updated instructions.") {
		t.Fatalf("expected patched body, got %q", body)
	}

	if err := mgr.WriteWorkspaceSkillFile("patchable-skill", "references/guide.md", "# Guide"); err != nil {
		t.Fatalf("write supporting file: %v", err)
	}
	files, err := mgr.ListSupportingFiles("patchable-skill")
	if err != nil {
		t.Fatalf("list supporting files: %v", err)
	}
	if len(files) != 1 || files[0] != "references/guide.md" {
		t.Fatalf("unexpected supporting files: %+v", files)
	}
	if err := mgr.RemoveWorkspaceSkillFile("patchable-skill", "references/guide.md"); err != nil {
		t.Fatalf("remove supporting file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "patchable-skill", "references", "guide.md")); !os.IsNotExist(err) {
		t.Fatalf("expected supporting file to be removed, stat err=%v", err)
	}
}
