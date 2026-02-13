// Package skills provides a pluggable skills system for extending agent capabilities.
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"nekobot/pkg/logger"
)

// Skill represents a pluggable skill that can be loaded by the agent.
type Skill struct {
	// Metadata from frontmatter
	ID          string                 `yaml:"id" json:"id"`
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Version     string                 `yaml:"version" json:"version"`
	Author      string                 `yaml:"author" json:"author"`
	Tags        []string               `yaml:"tags" json:"tags"`
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Metadata    map[string]interface{} `yaml:"metadata" json:"metadata"`

	// Skill content
	Instructions string `yaml:"-" json:"instructions"` // The actual skill prompt
	FilePath     string `yaml:"-" json:"file_path"`    // Path to skill file

	// Dependencies
	Dependencies []string `yaml:"dependencies" json:"dependencies,omitempty"`

	// Installation requirements
	Requirements *SkillRequirements `yaml:"requirements" json:"requirements,omitempty"`
}

// SkillRequirements defines what a skill needs to run.
type SkillRequirements struct {
	Tools     []string               `yaml:"tools" json:"tools,omitempty"`         // Required tools
	Env       []string               `yaml:"env" json:"env,omitempty"`             // Required env vars
	Binaries  []string               `yaml:"binaries" json:"binaries,omitempty"`   // Required binaries
	Languages map[string]string      `yaml:"languages" json:"languages,omitempty"` // Language versions
	Custom    map[string]interface{} `yaml:"custom" json:"custom,omitempty"`       // Custom requirements
}

// Manager manages skill discovery, loading, and execution.
type Manager struct {
	log              *logger.Logger
	skillsDir        string
	skills           map[string]*Skill // ID -> Skill
	mu               sync.RWMutex
	autoReload       bool
	watcher          *Watcher
	validator        *Validator
	eligibilityCheck *EligibilityChecker
	installer        *Installer
}

// NewManager creates a new skills manager.
func NewManager(log *logger.Logger, skillsDir string, autoReload bool) *Manager {
	return &Manager{
		log:              log,
		skillsDir:        skillsDir,
		skills:           make(map[string]*Skill),
		autoReload:       autoReload,
		validator:        NewValidator(),
		eligibilityCheck: NewEligibilityChecker(),
		installer:        NewInstaller(log),
	}
}

// Discover scans the skills directory and loads all valid skills.
func (m *Manager) Discover() error {
	m.log.Info("Discovering skills", logger.String("dir", m.skillsDir))

	// Create skills directory if it doesn't exist
	if err := os.MkdirAll(m.skillsDir, 0755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	// Scan for skill files (*.md)
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return fmt.Errorf("reading skills directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		skillPath := filepath.Join(m.skillsDir, entry.Name())
		skill, err := m.loadSkillFile(skillPath)
		if err != nil {
			m.log.Warn("Failed to load skill",
				logger.String("file", entry.Name()),
				logger.Error(err))
			continue
		}

		m.registerSkill(skill)
		count++
	}

	m.log.Info("Skills discovered", logger.Int("count", count))
	return nil
}

// loadSkillFile loads a skill from a markdown file with YAML frontmatter.
func (m *Manager) loadSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill file: %w", err)
	}

	content := string(data)

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

// registerSkill registers a skill with the manager.
func (m *Manager) registerSkill(skill *Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.skills[skill.ID] = skill
	m.log.Debug("Skill registered",
		logger.String("id", skill.ID),
		logger.String("name", skill.Name))
}

// Get retrieves a skill by ID.
func (m *Manager) Get(id string) (*Skill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skill, exists := m.skills[id]
	if !exists {
		return nil, fmt.Errorf("skill not found: %s", id)
	}

	return skill, nil
}

// List returns all discovered skills.
func (m *Manager) List() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*Skill, 0, len(m.skills))
	for _, skill := range m.skills {
		skills = append(skills, skill)
	}

	return skills
}

// ListEnabled returns all enabled skills.
func (m *Manager) ListEnabled() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*Skill, 0)
	for _, skill := range m.skills {
		if skill.Enabled {
			skills = append(skills, skill)
		}
	}

	return skills
}

// Enable enables a skill by ID.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	skill, exists := m.skills[id]
	if !exists {
		return fmt.Errorf("skill not found: %s", id)
	}

	skill.Enabled = true
	m.log.Info("Skill enabled", logger.String("id", id))
	return nil
}

// Disable disables a skill by ID.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	skill, exists := m.skills[id]
	if !exists {
		return fmt.Errorf("skill not found: %s", id)
	}

	skill.Enabled = false
	m.log.Info("Skill disabled", logger.String("id", id))
	return nil
}

// GetInstructions returns the combined instructions for all enabled skills.
func (m *Manager) GetInstructions() string {
	skills := m.ListEnabled()
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Available Skills\n\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("## %s\n\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", skill.Description))
		}
		sb.WriteString(skill.Instructions)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}

// Reload reloads all skills from disk.
func (m *Manager) Reload() error {
	m.log.Info("Reloading skills")

	// Clear existing skills
	m.mu.Lock()
	m.skills = make(map[string]*Skill)
	m.mu.Unlock()

	// Discover again
	return m.Discover()
}

// StartWatching starts watching skill files for changes.
func (m *Manager) StartWatching(ctx context.Context) error {
	if m.watcher == nil {
		var err error
		m.watcher, err = NewWatcher(m.log, m)
		if err != nil {
			return fmt.Errorf("creating watcher: %w", err)
		}
	}

	return m.watcher.Start(ctx)
}

// StopWatching stops watching skill files.
func (m *Manager) StopWatching() error {
	if m.watcher == nil {
		return nil
	}
	return m.watcher.Stop()
}

// WatchEvents returns the channel of skill change events.
func (m *Manager) WatchEvents() <-chan SkillChangeEvent {
	if m.watcher == nil {
		// Return closed channel if no watcher
		ch := make(chan SkillChangeEvent)
		close(ch)
		return ch
	}
	return m.watcher.Events()
}

// WatcherStatus returns the current watcher status.
func (m *Manager) WatcherStatus() *WatcherStatus {
	if m.watcher == nil {
		return nil
	}
	status := m.watcher.Status()
	return &status
}

// CheckRequirements checks if a skill's requirements are met.
func (m *Manager) CheckRequirements(ctx context.Context, skillID string) (bool, []string) {
	skill, err := m.Get(skillID)
	if err != nil {
		return false, []string{fmt.Sprintf("skill not found: %s", skillID)}
	}

	if skill.Requirements == nil {
		return true, nil
	}

	return m.eligibilityCheck.Check(skill)
}

// ValidateSkill validates a skill and returns diagnostics.
func (m *Manager) ValidateSkill(skill *Skill) []Diagnostic {
	return m.validator.Validate(skill)
}

// InstallDependencies installs dependencies for a skill.
func (m *Manager) InstallDependencies(ctx context.Context, skillID string) ([]InstallResult, error) {
	skill, err := m.Get(skillID)
	if err != nil {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	if skill.Requirements == nil {
		return nil, nil
	}

	// Parse install specs from requirements
	specs := ParseRequirementsToSpecs(skill.Requirements)
	if len(specs) == 0 {
		return nil, nil
	}

	m.log.Info("Installing dependencies for skill",
		logger.String("skill", skill.ID),
		logger.Int("count", len(specs)))

	var results []InstallResult
	for _, spec := range specs {
		result := m.installer.Install(ctx, spec)
		results = append(results, result)

		// Stop on first failure if not continuing
		if !result.Success {
			break
		}
	}

	return results, nil
}
