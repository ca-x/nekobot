package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"nekobot/pkg/fileutil"
)

const skillFileName = "SKILL.md"

// PreviewSkillContent parses and validates SKILL.md content without writing it.
func (m *Manager) PreviewSkillContent(content string) (*Skill, error) {
	if m == nil {
		return nil, fmt.Errorf("skills manager is nil")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("skill content is required")
	}

	skill, err := parseSkillContent(content, skillFileName)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("parsed skill is nil")
	}
	if m.validator == nil {
		return nil, fmt.Errorf("skill validator is unavailable")
	}
	if diags := m.validator.Validate(skill); len(diags) > 0 {
		var errs []string
		for _, diag := range diags {
			if diag.Severity == DiagnosticError {
				errs = append(errs, diag.Message)
			}
		}
		if len(errs) > 0 {
			return nil, fmt.Errorf("invalid skill: %s", strings.Join(errs, "; "))
		}
	}
	return skill, nil
}

// GetByName returns a skill by id or display name.
func (m *Manager) GetByName(name string) (*Skill, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	if skill, err := m.Get(trimmed); err == nil {
		return skill, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, skill := range m.skills {
		if skill == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(skill.Name), trimmed) {
			return skill, nil
		}
	}
	return nil, fmt.Errorf("skill not found: %s", trimmed)
}

// WorkspaceSkillPath returns the canonical writable SKILL.md path for one skill id.
func (m *Manager) WorkspaceSkillPath(id string) (string, error) {
	if m == nil {
		return "", fmt.Errorf("skills manager is nil")
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("skill id is required")
	}
	if m.validator == nil {
		return "", fmt.Errorf("skill validator is unavailable")
	}
	if diags := m.validator.ValidateID(trimmed); len(diags) > 0 {
		for _, diag := range diags {
			if diag.Severity == DiagnosticError {
				return "", fmt.Errorf("%s", diag.Message)
			}
		}
	}
	return filepath.Join(m.skillsDir, trimmed, skillFileName), nil
}

// SaveWorkspaceSkill writes or replaces a workspace skill, tracks snapshot/version data,
// and refreshes the in-memory registry.
func (m *Manager) SaveWorkspaceSkill(content string) (*Skill, bool, error) {
	if m == nil {
		return nil, false, fmt.Errorf("skills manager is nil")
	}
	content = strings.TrimSpace(content)
	skill, err := m.PreviewSkillContent(content)
	if err != nil {
		return nil, false, err
	}

	targetPath, err := m.WorkspaceSkillPath(skill.ID)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, false, fmt.Errorf("create skill directory: %w", err)
	}

	created := true
	if _, statErr := os.Stat(targetPath); statErr == nil {
		created = false
	} else if !os.IsNotExist(statErr) {
		return nil, false, fmt.Errorf("stat skill path: %w", statErr)
	}

	if _, snapErr := m.CreateSnapshot(map[string]string{
		"action":   "save_skill",
		"skill_id": skill.ID,
		"target":   targetPath,
	}); snapErr != nil {
		m.log.Warn("Failed to create skill snapshot before save", zap.Error(snapErr))
	}

	if err := fileutil.WriteFileAtomic(targetPath, []byte(content+"\n"), 0o644); err != nil {
		return nil, false, fmt.Errorf("write skill file: %w", err)
	}

	skill.FilePath = targetPath
	m.registerSkill(skill)

	changeType := "modified"
	changeSummary := "Updated workspace skill via skill management"
	if created {
		changeType = "created"
		changeSummary = "Created workspace skill via skill management"
	}
	if m.versionMgr != nil {
		if err := m.versionMgr.TrackChange(skill, changeType, changeSummary); err != nil {
			m.log.Warn("Failed to track skill change", zap.Error(err))
		}
	}

	return skill, created, nil
}

// DeleteWorkspaceSkill removes a workspace skill file and refreshes discovered skills.
func (m *Manager) DeleteWorkspaceSkill(id string) error {
	if m == nil {
		return fmt.Errorf("skills manager is nil")
	}
	targetPath, err := m.WorkspaceSkillPath(id)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(targetPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("workspace skill not found: %s", strings.TrimSpace(id))
		}
		return fmt.Errorf("stat skill path: %w", statErr)
	}
	if _, snapErr := m.CreateSnapshot(map[string]string{
		"action":   "delete_skill",
		"skill_id": strings.TrimSpace(id),
		"target":   targetPath,
	}); snapErr != nil {
		m.log.Warn("Failed to create skill snapshot before delete", zap.Error(snapErr))
	}
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("delete skill file: %w", err)
	}
	_ = os.Remove(filepath.Dir(targetPath))
	return m.Discover()
}
