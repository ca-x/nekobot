package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/skills"
)

// SkillManageTool provides agent-managed workspace skill CRUD inspired by Hermes skill_manage.
type SkillManageTool struct {
	manager *skills.Manager
}

// NewSkillManageTool creates a new skill management tool.
func NewSkillManageTool(manager *skills.Manager) *SkillManageTool {
	return &SkillManageTool{manager: manager}
}

func (t *SkillManageTool) Name() string { return "skill_manage" }

func (t *SkillManageTool) Description() string {
	return "Create, update, inspect, and delete workspace skills. Use this to turn a successful reusable workflow into a durable SKILL.md-based skill."
}

func (t *SkillManageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"get", "create", "update", "patch", "delete", "write_file", "remove_file"},
				"description": "Action to perform on workspace skills",
			},
			"skill_id": map[string]interface{}{
				"type":        "string",
				"description": "Skill id/name for get, update, or delete",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Full SKILL.md content for create or update",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Optional supporting file path under references/, templates/, scripts/, or assets/",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "Target text to replace for patch",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "Replacement text for patch (can be empty to delete the matched text)",
			},
			"replace_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Replace all matches when patching",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SkillManageTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	_ = ctx
	if t == nil || t.manager == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}
	action, _ := params["action"].(string)
	action = strings.TrimSpace(strings.ToLower(action))
	skillID, _ := params["skill_id"].(string)
	skillID = strings.TrimSpace(skillID)
	content, _ := params["content"].(string)
	filePath, _ := params["file_path"].(string)
	oldString, _ := params["old_string"].(string)
	newString, newStringProvided := params["new_string"].(string)
	replaceAll, _ := params["replace_all"].(bool)

	switch action {
	case "get":
		if skillID == "" {
			return "", fmt.Errorf("skill_id is required for get")
		}
		body, err := t.manager.ReadSkillFile(skillID, filePath)
		if err != nil {
			return "", err
		}
		return body, nil
	case "create":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content is required for create")
		}
		preview, err := t.manager.PreviewSkillContent(content)
		if err != nil {
			return "", err
		}
		if _, err := t.manager.GetByName(preview.ID); err == nil {
			return "", fmt.Errorf("skill %s already exists; use update", preview.ID)
		}
		skill, created, err := t.manager.SaveWorkspaceSkill(content)
		if err != nil {
			return "", err
		}
		if !created {
			return "", fmt.Errorf("skill %s already exists; use update", skill.ID)
		}
		return fmt.Sprintf("Created workspace skill %s at %s", skill.ID, skill.FilePath), nil
	case "update":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("content is required for update")
		}
		preview, err := t.manager.PreviewSkillContent(content)
		if err != nil {
			return "", err
		}
		targetID := preview.ID
		if skillID != "" {
			existing, err := t.manager.GetByName(skillID)
			if err != nil {
				return "", err
			}
			if strings.HasPrefix(strings.TrimSpace(existing.FilePath), "builtin://") {
				return "", fmt.Errorf("builtin skill %s cannot be updated in place", existing.ID)
			}
			if existing.ID != preview.ID {
				return "", fmt.Errorf("update content skill id %s does not match target %s", preview.ID, existing.ID)
			}
			targetID = existing.ID
		} else if _, err := t.manager.GetByName(preview.ID); err != nil {
			return "", fmt.Errorf("skill %s does not exist yet; use create", preview.ID)
		}
		skill, created, err := t.manager.SaveWorkspaceSkill(content)
		if err != nil {
			return "", err
		}
		if created {
			return "", fmt.Errorf("skill %s does not exist yet; use create", targetID)
		}
		return fmt.Sprintf("Updated workspace skill %s at %s", skill.ID, skill.FilePath), nil
	case "patch":
		if skillID == "" {
			return "", fmt.Errorf("skill_id is required for patch")
		}
		if strings.TrimSpace(oldString) == "" {
			return "", fmt.Errorf("old_string is required for patch")
		}
		if !newStringProvided {
			return "", fmt.Errorf("new_string is required for patch")
		}
		if err := t.manager.PatchWorkspaceSkillFile(skillID, filePath, oldString, newString, replaceAll); err != nil {
			return "", err
		}
		target := strings.TrimSpace(filePath)
		if target == "" {
			target = "SKILL.md"
		}
		return fmt.Sprintf("Patched %s in skill %s", target, skillID), nil
	case "delete":
		if skillID == "" {
			return "", fmt.Errorf("skill_id is required for delete")
		}
		skill, err := t.manager.GetByName(skillID)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
			return "", fmt.Errorf("builtin skill %s cannot be deleted", skill.ID)
		}
		if err := t.manager.DeleteWorkspaceSkill(skill.ID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted workspace skill %s", skill.ID), nil
	case "write_file":
		if skillID == "" {
			return "", fmt.Errorf("skill_id is required for write_file")
		}
		if strings.TrimSpace(filePath) == "" {
			return "", fmt.Errorf("file_path is required for write_file")
		}
		if err := t.manager.WriteWorkspaceSkillFile(skillID, filePath, content); err != nil {
			return "", err
		}
		return fmt.Sprintf("Wrote %s in skill %s", filePath, skillID), nil
	case "remove_file":
		if skillID == "" {
			return "", fmt.Errorf("skill_id is required for remove_file")
		}
		if strings.TrimSpace(filePath) == "" {
			return "", fmt.Errorf("file_path is required for remove_file")
		}
		if err := t.manager.RemoveWorkspaceSkillFile(skillID, filePath); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed %s from skill %s", filePath, skillID), nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}
