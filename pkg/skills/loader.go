// Package skills provides multi-path skill loading with priority.
package skills

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"nekobot/pkg/logger"
)

//go:embed builtin/*
var builtinSkillsFS embed.FS

// SkillSource represents a source directory for skills with priority.
type SkillSource struct {
	Path     string
	Priority int
	Type     SourceType
}

// SourceType indicates the type of skill source.
type SourceType string

const (
	SourceBuiltin    SourceType = "builtin"
	SourceExecutable SourceType = "executable"
	SourceGlobal     SourceType = "global"
	SourceWorkspace  SourceType = "workspace"
	SourceLocal      SourceType = "local"
)

// MultiPathLoader handles loading skills from multiple directories with priority.
type MultiPathLoader struct {
	log     *logger.Logger
	sources []SkillSource
}

// NewMultiPathLoader creates a new multi-path skill loader.
func NewMultiPathLoader(log *logger.Logger, workspaceDir string) *MultiPathLoader {
	loader := &MultiPathLoader{
		log:     log,
		sources: make([]SkillSource, 0),
	}

	// Build load order (later overrides earlier)
	loader.addDefaultSources(workspaceDir)

	return loader
}

// addDefaultSources adds the default skill source paths with priority.
func (l *MultiPathLoader) addDefaultSources(workspaceDir string) {
	// 1. Embedded built-in skills (priority 0)
	l.AddSource(SkillSource{
		Path:     "builtin",
		Priority: 0,
		Type:     SourceBuiltin,
	})

	// 2. Executable directory (priority 10)
	if execDir, err := getExecutableDir(); err == nil {
		l.AddSource(SkillSource{
			Path:     filepath.Join(execDir, "skills"),
			Priority: 10,
			Type:     SourceExecutable,
		})
	}

	// 3. Global config directory (priority 20)
	if home, err := os.UserHomeDir(); err == nil {
		l.AddSource(SkillSource{
			Path:     filepath.Join(home, ".nekobot", "skills"),
			Priority: 20,
			Type:     SourceGlobal,
		})
	}

	// 4. Workspace hidden directory (priority 30)
	if workspaceDir != "" {
		l.AddSource(SkillSource{
			Path:     filepath.Join(workspaceDir, ".nekobot", "skills"),
			Priority: 30,
			Type:     SourceWorkspace,
		})

		// 5. Workspace directory (priority 40)
		l.AddSource(SkillSource{
			Path:     filepath.Join(workspaceDir, "skills"),
			Priority: 40,
			Type:     SourceWorkspace,
		})
	}

	// 6. Current directory (priority 50 - highest)
	if cwd, err := os.Getwd(); err == nil {
		l.AddSource(SkillSource{
			Path:     filepath.Join(cwd, "skills"),
			Priority: 50,
			Type:     SourceLocal,
		})
	}
}

// AddSource adds a custom skill source.
func (l *MultiPathLoader) AddSource(source SkillSource) {
	l.sources = append(l.sources, source)
	l.log.Debug("Added skill source",
		zap.String("path", source.Path),
		zap.Int("priority", source.Priority),
		zap.String("type", string(source.Type)))
}

// LoadAll loads skills from all sources, with later sources overriding earlier ones.
func (l *MultiPathLoader) LoadAll() (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	// Process sources in priority order (ascending)
	for _, source := range l.sources {
		sourceSkills, err := l.loadFromSource(source)
		if err != nil {
			l.log.Warn("Failed to load from source",
				zap.String("path", source.Path),
				zap.Error(err))
			continue
		}

		// Merge skills - later sources override earlier ones
		for id, skill := range sourceSkills {
			if existing, exists := skills[id]; exists {
				l.log.Info("Skill overridden",
					zap.String("id", id),
					zap.String("from", existing.FilePath),
					zap.String("to", skill.FilePath))
			}
			skills[id] = skill
		}
	}

	l.log.Info("Loaded skills from all sources",
		zap.Int("total", len(skills)),
		zap.Int("sources", len(l.sources)))

	return skills, nil
}

// loadFromSource loads skills from a single source.
func (l *MultiPathLoader) loadFromSource(source SkillSource) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	// Handle built-in embedded skills
	if source.Type == SourceBuiltin {
		return l.loadBuiltinSkills()
	}

	// Handle filesystem sources
	if _, err := os.Stat(source.Path); os.IsNotExist(err) {
		// Directory doesn't exist, skip
		return skills, nil
	}

	entries, err := os.ReadDir(source.Path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		skillPath := filepath.Join(source.Path, entry.Name())
		skill, err := loadSkillFile(skillPath)
		if err != nil {
			l.log.Warn("Failed to load skill",
				zap.String("file", skillPath),
				zap.Error(err))
			continue
		}

		skills[skill.ID] = skill
		l.log.Debug("Loaded skill",
			zap.String("id", skill.ID),
			zap.String("path", skillPath))
	}

	return skills, nil
}

// loadBuiltinSkills loads embedded built-in skills.
func (l *MultiPathLoader) loadBuiltinSkills() (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	entries, err := builtinSkillsFS.ReadDir("builtin")
	if err != nil {
		// No built-in skills embedded, that's okay
		return skills, nil
	}

	for _, entry := range entries {
		// Each skill is in its own directory with a SKILL.md file
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillMDPath := filepath.Join("builtin", skillName, "SKILL.md")

		data, err := builtinSkillsFS.ReadFile(skillMDPath)
		if err != nil {
			// No SKILL.md in this directory, skip
			l.log.Debug("Skipping builtin directory without SKILL.md",
				zap.String("dir", skillName))
			continue
		}

		skill, err := parseSkillContent(string(data), fmt.Sprintf("builtin://%s", skillName))
		if err != nil {
			l.log.Warn("Failed to parse built-in skill",
				zap.String("dir", skillName),
				zap.Error(err))
			continue
		}

		skills[skill.ID] = skill
		l.log.Debug("Loaded built-in skill",
			zap.String("id", skill.ID),
			zap.String("dir", skillName))
	}

	return skills, nil
}

// GetSources returns all configured skill sources.
func (l *MultiPathLoader) GetSources() []SkillSource {
	return l.sources
}

// Helper functions

// getExecutableDir returns the directory containing the executable.
func getExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	return filepath.Dir(exe), nil
}

// loadSkillFile loads a skill from a file path (moved from manager.go).
func loadSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill file: %w", err)
	}

	return parseSkillContent(string(data), path)
}

// parseSkillContent parses skill content from string.
func parseSkillContent(content, path string) (*Skill, error) {
	// Parse frontmatter (YAML between --- markers)
	var frontmatter string
	var instructions string

	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---\n", 2)
		if len(parts) == 2 {
			frontmatter = parts[0]
			instructions = strings.TrimSpace(parts[1])
		} else {
			// No closing ---, treat entire file as instructions
			instructions = content
		}
	} else {
		// No frontmatter, entire file is instructions
		instructions = content
	}

	// Parse YAML frontmatter
	skill := &Skill{
		Enabled: true, // Default to enabled
	}

	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), skill); err != nil {
			return nil, fmt.Errorf("parsing frontmatter: %w", err)
		}
	}

	// Set default ID from filename if not specified
	if skill.ID == "" {
		skill.ID = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	// Set default name from ID if not specified
	if skill.Name == "" {
		skill.Name = skill.ID
	}

	skill.Instructions = instructions
	skill.FilePath = path

	return skill, nil
}

// CheckEligibility checks if a skill is eligible to run on current system.
func CheckEligibility(skill *Skill) (bool, []string) {
	if skill.Requirements == nil {
		return true, nil
	}

	var reasons []string

	// Check OS requirements
	if osReqs, ok := skill.Requirements.Custom["os"].([]interface{}); ok {
		currentOS := runtime.GOOS
		eligible := false
		for _, osReq := range osReqs {
			if osStr, ok := osReq.(string); ok && osStr == currentOS {
				eligible = true
				break
			}
		}
		if !eligible {
			reasons = append(reasons, fmt.Sprintf("OS not supported (current: %s)", currentOS))
		}
	}

	// Check architecture requirements
	if archReqs, ok := skill.Requirements.Custom["arch"].([]interface{}); ok {
		currentArch := runtime.GOARCH
		eligible := false
		for _, archReq := range archReqs {
			if archStr, ok := archReq.(string); ok && archStr == currentArch {
				eligible = true
				break
			}
		}
		if !eligible {
			reasons = append(reasons, fmt.Sprintf("Architecture not supported (current: %s)", currentArch))
		}
	}

	return len(reasons) == 0, reasons
}
