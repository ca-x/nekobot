package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
	"nekobot/pkg/fileutil"
)

const skillFileName = "SKILL.md"

var allowedSkillSubdirs = map[string]struct{}{
	"references": {},
	"templates":  {},
	"scripts":    {},
	"assets":     {},
}

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

// ReadSkillFile returns the content of SKILL.md or a supporting file within a workspace skill.
func (m *Manager) ReadSkillFile(id, filePath string) (string, error) {
	skill, err := m.GetByName(id)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
		if strings.TrimSpace(filePath) != "" {
			return "", fmt.Errorf("builtin skill supporting files are not addressable")
		}
		return ReadBuiltinSkillContent(skill.FilePath)
	}
	target, err := m.resolveSkillTarget(skill, filePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("read skill file %s: %w", target, err)
	}
	return string(data), nil
}

// PatchWorkspaceSkillFile performs a targeted string replacement in SKILL.md or a supporting file.
func (m *Manager) PatchWorkspaceSkillFile(id, filePath, oldString, newString string, replaceAll bool) error {
	if m == nil {
		return fmt.Errorf("skills manager is nil")
	}
	oldString = strings.TrimSpace(oldString)
	if oldString == "" {
		return fmt.Errorf("old_string is required")
	}
	skill, err := m.GetByName(id)
	if err != nil {
		return err
	}
	if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
		return fmt.Errorf("builtin skill %s cannot be patched in place", skill.ID)
	}
	target, err := m.resolveSkillTarget(skill, filePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read skill file %s: %w", target, err)
	}
	content := string(data)
	count := strings.Count(content, oldString)
	if count == 0 {
		return fmt.Errorf("old_string not found")
	}
	if count > 1 && !replaceAll {
		return fmt.Errorf("old_string matched %d times; use replace_all to patch every occurrence", count)
	}
	replaced := content
	if replaceAll {
		replaced = strings.ReplaceAll(content, oldString, newString)
	} else {
		replaced = strings.Replace(content, oldString, newString, 1)
	}
	if err := fileutil.WriteFileAtomic(target, []byte(replaced), 0o644); err != nil {
		return fmt.Errorf("write patched skill file: %w", err)
	}
	if filepath.Base(target) == skillFileName {
		return m.Discover()
	}
	return nil
}

// WriteWorkspaceSkillFile writes a supporting file beneath one workspace skill.
func (m *Manager) WriteWorkspaceSkillFile(id, filePath, content string) error {
	if m == nil {
		return fmt.Errorf("skills manager is nil")
	}
	skill, err := m.GetByName(id)
	if err != nil {
		return err
	}
	if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
		return fmt.Errorf("builtin skill %s cannot be modified in place", skill.ID)
	}
	target, err := m.resolveSkillTarget(skill, filePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create skill file directory: %w", err)
	}
	if err := fileutil.WriteFileAtomic(target, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write skill file: %w", err)
	}
	return nil
}

// RemoveWorkspaceSkillFile removes a supporting file beneath one workspace skill.
func (m *Manager) RemoveWorkspaceSkillFile(id, filePath string) error {
	if m == nil {
		return fmt.Errorf("skills manager is nil")
	}
	skill, err := m.GetByName(id)
	if err != nil {
		return err
	}
	if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
		return fmt.Errorf("builtin skill %s cannot be modified in place", skill.ID)
	}
	target, err := m.resolveSkillTarget(skill, filePath)
	if err != nil {
		return err
	}
	if filepath.Base(target) == skillFileName {
		return fmt.Errorf("remove the entire skill instead of deleting SKILL.md directly")
	}
	if err := os.Remove(target); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill file not found: %s", filePath)
		}
		return fmt.Errorf("remove skill file: %w", err)
	}
	parent := filepath.Dir(target)
	if parent != filepath.Dir(skill.FilePath) {
		_ = os.Remove(parent)
	}
	return nil
}

// ListSupportingFiles returns relative supporting file paths for one skill.
func (m *Manager) ListSupportingFiles(id string) ([]string, error) {
	skill, err := m.GetByName(id)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://") {
		return nil, nil
	}
	root := filepath.Dir(skill.FilePath)
	var paths []string
	for subdir := range allowedSkillSubdirs {
		base := filepath.Join(root, subdir)
		_ = filepath.Walk(base, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err == nil {
				paths = append(paths, filepath.ToSlash(rel))
			}
			return nil
		})
	}
	sort.Strings(paths)
	return paths, nil
}

func (m *Manager) resolveSkillTarget(skill *Skill, filePath string) (string, error) {
	if skill == nil {
		return "", fmt.Errorf("skill is nil")
	}
	root := filepath.Dir(strings.TrimSpace(skill.FilePath))
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return skill.FilePath, nil
	}
	cleaned := filepath.Clean(trimmed)
	parts := strings.Split(filepath.ToSlash(cleaned), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("file_path must include an allowed subdirectory and filename")
	}
	if _, ok := allowedSkillSubdirs[parts[0]]; !ok {
		return "", fmt.Errorf("file_path must be under references/, templates/, scripts/, or assets/")
	}
	target := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("resolve skill file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file_path must stay within the skill directory")
	}
	return target, nil
}
