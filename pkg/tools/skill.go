package tools

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
)

// SkillTool provides skill invocation for the agent.
type SkillTool struct {
	log     *logger.Logger
	manager *skills.Manager
}

// NewSkillTool creates a new skill tool.
func NewSkillTool(log *logger.Logger, manager *skills.Manager) *SkillTool {
	return &SkillTool{
		log:     log,
		manager: manager,
	}
}

// Name returns the tool name.
func (t *SkillTool) Name() string {
	return "skill"
}

// Description returns the tool description.
func (t *SkillTool) Description() string {
	return `Query and invoke specialized skills. Actions: "list" returns all skills with IDs, names, and descriptions. "get" returns metadata, eligibility, and instructions preview for a skill_id. "invoke" returns full instructions to follow for a skill_id.`
}

// Parameters returns the tool parameters schema.
func (t *SkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "get", "invoke"},
				"description": "Action to perform: list, get, or invoke",
			},
			"skill_id": map[string]interface{}{
				"type":        "string",
				"description": "Skill ID (required for get and invoke)",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Additional context for skill invocation (optional for invoke)",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the skill tool.
func (t *SkillTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	if t.manager == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}

	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter is required")
	}

	switch action {
	case "list":
		return t.list()
	case "get":
		return t.get(params)
	case "invoke":
		return t.invoke(params)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// list lists all available skills.
func (t *SkillTool) list() (string, error) {
	allSkills := t.manager.List()
	if len(allSkills) == 0 {
		return "No skills available", nil
	}

	// Group by enabled status
	enabled := []*skills.Skill{}
	disabled := []*skills.Skill{}

	for _, skill := range allSkills {
		if skill.Enabled {
			enabled = append(enabled, skill)
		} else {
			disabled = append(disabled, skill)
		}
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Available Skills (%d total):\n\n", len(allSkills))
	sb.WriteString("Progressive disclosure:\n")
	sb.WriteString("- use `skill list` to scan names and short descriptions\n")
	sb.WriteString("- use `skill get <skill_id>` to inspect one skill in detail\n")
	sb.WriteString("- use `skill invoke <skill_id>` to load its full instructions\n")
	sb.WriteString("- use `skill_manage` for create/update/patch/file operations\n\n")

	// Show enabled skills
	if len(enabled) > 0 {
		_, _ = fmt.Fprintf(&sb, "## ENABLED (%d)\n\n", len(enabled))
		for _, skill := range enabled {
			_, _ = fmt.Fprintf(&sb, "- **%s** (ID: %s)\n", skill.Name, skill.ID)
			if skill.Description != "" {
				_, _ = fmt.Fprintf(&sb, "  %s\n", skill.Description)
			}
		}
		sb.WriteString("\n")
	}

	// Show disabled skills
	if len(disabled) > 0 {
		_, _ = fmt.Fprintf(&sb, "## DISABLED (%d)\n\n", len(disabled))
		for _, skill := range disabled {
			_, _ = fmt.Fprintf(&sb, "- **%s** (ID: %s)\n", skill.Name, skill.ID)
			if skill.Description != "" {
				_, _ = fmt.Fprintf(&sb, "  %s\n", skill.Description)
			}
			// Show why disabled (if requirements exist)
			if skill.Requirements != nil {
				eligible, reasons := t.manager.CheckRequirements(context.Background(), skill.ID)
				if !eligible {
					_, _ = fmt.Fprintf(&sb, "  ⚠️  Missing requirements: %s\n", strings.Join(reasons, ", "))
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nUse `skill get <skill_id>` for detailed information.\n")
	sb.WriteString("Use `skill invoke <skill_id>` to execute a skill.\n")

	return sb.String(), nil
}

// get retrieves detailed information about a skill.
func (t *SkillTool) get(params map[string]interface{}) (string, error) {
	skillID, ok := params["skill_id"].(string)
	if !ok || skillID == "" {
		return "", fmt.Errorf("skill_id parameter is required for get")
	}

	skill, err := t.manager.Get(skillID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "# %s\n\n", skill.Name)
	_, _ = fmt.Fprintf(&sb, "**ID:** %s\n", skill.ID)
	_, _ = fmt.Fprintf(&sb, "**Status:** %s\n\n", enabledStatus(skill.Enabled))

	if skill.Description != "" {
		_, _ = fmt.Fprintf(&sb, "**Description:**\n%s\n\n", skill.Description)
	}

	// Show requirements
	if skill.Requirements != nil {
		sb.WriteString("**Requirements:**\n")

		if len(skill.Requirements.Binaries) > 0 {
			_, _ = fmt.Fprintf(&sb, "- Binaries: %s\n", strings.Join(skill.Requirements.Binaries, ", "))
		}
		if len(skill.Requirements.Env) > 0 {
			_, _ = fmt.Fprintf(&sb, "- Environment Variables: %s\n", strings.Join(skill.Requirements.Env, ", "))
		}
		if len(skill.Requirements.Languages) > 0 {
			for lang, version := range skill.Requirements.Languages {
				_, _ = fmt.Fprintf(&sb, "- %s: %s\n", lang, version)
			}
		}
		if len(skill.Requirements.ConfigPaths) > 0 {
			_, _ = fmt.Fprintf(&sb, "- Config Paths: %s\n", strings.Join(skill.Requirements.ConfigPaths, ", "))
		}
		if len(skill.Requirements.PythonPackages) > 0 {
			_, _ = fmt.Fprintf(&sb, "- Python Packages: %s\n", strings.Join(skill.Requirements.PythonPackages, ", "))
		}
		if len(skill.Requirements.NodePackages) > 0 {
			_, _ = fmt.Fprintf(&sb, "- Node Packages: %s\n", strings.Join(skill.Requirements.NodePackages, ", "))
		}

		// Check if requirements are met
		report, err := t.manager.CheckRequirementsReport(skill.ID)
		if err != nil {
			return "", err
		}
		if !report.Eligible {
			sb.WriteString("\n⚠️  **Missing Requirements:**\n")
			for _, reason := range report.Reasons {
				_, _ = fmt.Fprintf(&sb, "- %s\n", reason)
			}
		} else {
			sb.WriteString("\n✅ All requirements met\n")
		}
		sb.WriteString("\n")
	}

	// Show instructions preview (first 500 chars)
	if skill.Instructions != "" {
		preview := skill.Instructions
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		_, _ = fmt.Fprintf(&sb, "**Instructions Preview:**\n```\n%s\n```\n\n", preview)
		_, _ = fmt.Fprintf(&sb, "Full instructions: %d characters\n", len(skill.Instructions))
	}
	if supportingFiles, err := t.manager.ListSupportingFiles(skill.ID); err == nil && len(supportingFiles) > 0 {
		sb.WriteString("\n**Supporting Files:**\n")
		for _, item := range supportingFiles {
			_, _ = fmt.Fprintf(&sb, "- %s\n", item)
		}
	}

	return sb.String(), nil
}

// invoke invokes a skill and returns its instructions.
func (t *SkillTool) invoke(params map[string]interface{}) (string, error) {
	skillID, ok := params["skill_id"].(string)
	if !ok || skillID == "" {
		return "", fmt.Errorf("skill_id parameter is required for invoke")
	}

	userContext, _ := params["context"].(string)

	skill, err := t.manager.Get(skillID)
	if err != nil {
		return "", err
	}

	if !skill.Enabled {
		return "", fmt.Errorf("skill '%s' is disabled (missing requirements)", skill.ID)
	}

	t.log.Info("Invoking skill",
		zap.String("skill_id", skill.ID),
		zap.String("skill_name", skill.Name))

	// Build response with skill instructions
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Skill '%s' invoked successfully.\n\n", skill.Name)
	sb.WriteString("You should now follow the instructions below to complete the task:\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString(skill.Instructions)
	sb.WriteString("\n\n---\n\n")

	if userContext != "" {
		_, _ = fmt.Fprintf(&sb, "**User Context:**\n%s\n\n", userContext)
	}

	sb.WriteString("Apply the above instructions to handle the user's request.")

	return sb.String(), nil
}

// enabledStatus returns a human-readable status string.
func enabledStatus(enabled bool) string {
	if enabled {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}
