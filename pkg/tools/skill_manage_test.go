package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
)

func TestSkillManageToolCreateGetUpdateDelete(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	mgr := skills.NewManager(log, filepath.Join(t.TempDir(), "skills"), false)
	tool := NewSkillManageTool(mgr)
	ctx := context.Background()
	content := `---
id: extracted-playbook
name: Extracted Playbook
description: Saved from a successful workflow
---

Follow these exact steps.`

	result, err := tool.Execute(ctx, map[string]interface{}{"action": "create", "content": content})
	if err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if !strings.Contains(result, "Created workspace skill extracted-playbook") {
		t.Fatalf("unexpected create result: %s", result)
	}

	body, err := tool.Execute(ctx, map[string]interface{}{"action": "get", "skill_id": "extracted-playbook"})
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if !strings.Contains(body, "Follow these exact steps.") {
		t.Fatalf("unexpected get body: %s", body)
	}

	updated := strings.Replace(content, "exact", "updated", 1)
	result, err = tool.Execute(ctx, map[string]interface{}{"action": "update", "skill_id": "extracted-playbook", "content": updated})
	if err != nil {
		t.Fatalf("update skill: %v", err)
	}
	if !strings.Contains(result, "Updated workspace skill extracted-playbook") {
		t.Fatalf("unexpected update result: %s", result)
	}

	result, err = tool.Execute(ctx, map[string]interface{}{"action": "delete", "skill_id": "extracted-playbook"})
	if err != nil {
		t.Fatalf("delete skill: %v", err)
	}
	if !strings.Contains(result, "Deleted workspace skill extracted-playbook") {
		t.Fatalf("unexpected delete result: %s", result)
	}
}

func TestSkillManageToolPatchWriteFileAndRemoveFile(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	mgr := skills.NewManager(log, filepath.Join(t.TempDir(), "skills"), false)
	tool := NewSkillManageTool(mgr)
	ctx := context.Background()
	content := `---
id: ops-playbook
name: Ops Playbook
---

Original steps.`
	if _, err := tool.Execute(ctx, map[string]interface{}{"action": "create", "content": content}); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "patch",
		"skill_id":   "ops-playbook",
		"old_string": "Original",
		"new_string": "Updated",
	}); err != nil {
		t.Fatalf("patch skill: %v", err)
	}
	body, err := tool.Execute(ctx, map[string]interface{}{"action": "get", "skill_id": "ops-playbook"})
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if !strings.Contains(body, "Updated steps.") {
		t.Fatalf("expected patched body, got %q", body)
	}
	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action":    "write_file",
		"skill_id":  "ops-playbook",
		"file_path": "references/runbook.md",
		"content":   "# Runbook",
	}); err != nil {
		t.Fatalf("write_file: %v", err)
	}
	getOut, err := NewSkillTool(log, mgr).Execute(ctx, map[string]interface{}{
		"action":   "get",
		"skill_id": "ops-playbook",
	})
	if err != nil {
		t.Fatalf("skill get after write_file: %v", err)
	}
	if !strings.Contains(getOut, "references/runbook.md") {
		t.Fatalf("expected supporting file to be listed, got:\n%s", getOut)
	}
	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action":    "remove_file",
		"skill_id":  "ops-playbook",
		"file_path": "references/runbook.md",
	}); err != nil {
		t.Fatalf("remove_file: %v", err)
	}
}
