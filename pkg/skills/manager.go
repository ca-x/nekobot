// Package skills provides a pluggable skills system for extending agent capabilities.
package skills

import (
	"context"
	"fmt"
	"html"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"

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
	Always      bool                   `yaml:"always" json:"always"`
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
	loader           *MultiPathLoader
	snapshotMgr      *SnapshotManager
	versionMgr       *VersionManager
}

// NewManager creates a new skills manager.
func NewManager(log *logger.Logger, skillsDir string, autoReload bool) *Manager {
	// Determine workspace directory (parent of skills dir)
	workspaceDir := filepath.Dir(skillsDir)

	// Create snapshot and version directories
	snapshotsDir := filepath.Join(workspaceDir, ".nekobot", "snapshots")
	versionsDir := filepath.Join(workspaceDir, ".nekobot", "versions")

	mgr := &Manager{
		log:              log,
		skillsDir:        skillsDir,
		skills:           make(map[string]*Skill),
		autoReload:       autoReload,
		validator:        NewValidator(),
		eligibilityCheck: NewEligibilityChecker(),
		installer:        NewInstaller(log),
		loader:           NewMultiPathLoader(log, workspaceDir),
		snapshotMgr:      NewSnapshotManager(log, snapshotsDir),
		versionMgr:       NewVersionManager(log, versionsDir),
	}

	// Initialize version manager
	if err := mgr.versionMgr.Initialize(); err != nil {
		log.Warn("Failed to initialize version manager", zap.Error(err))
	}

	return mgr
}

// Discover scans the skills directory and loads all valid skills.
func (m *Manager) Discover() error {
	m.log.Info("Discovering skills from all sources")

	// Load skills from all sources using multi-path loader
	skills, err := m.loader.LoadAll()
	if err != nil {
		return fmt.Errorf("loading skills: %w", err)
	}

	// Register all skills
	m.mu.Lock()
	m.skills = skills
	m.mu.Unlock()

	// Detect changes and track versions
	changes := m.versionMgr.DetectChanges(skills)
	for id, changeType := range changes {
		if skill, exists := skills[id]; exists {
			if err := m.versionMgr.TrackChange(skill, changeType, "Auto-detected on discovery"); err != nil {
				m.log.Warn("Failed to track skill change",
					zap.String("skill", id),
					zap.Error(err))
			}
		}
	}

	m.log.Info("Skills discovered",
		zap.Int("total", len(skills)),
		zap.Int("changes", len(changes)))

	return nil
}

// registerSkill registers a skill with the manager.
func (m *Manager) registerSkill(skill *Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.skills[skill.ID] = skill
	m.log.Debug("Skill registered",
		zap.String("id", skill.ID),
		zap.String("name", skill.Name))
}

// CreateSnapshot creates a snapshot of current skill state.
func (m *Manager) CreateSnapshot(metadata map[string]string) (*Snapshot, error) {
	m.mu.RLock()
	skillsCopy := make(map[string]*Skill, len(m.skills))
	for k, v := range m.skills {
		skillsCopy[k] = v
	}
	m.mu.RUnlock()

	return m.snapshotMgr.Create(skillsCopy, metadata)
}

// ListSnapshots returns all available snapshots.
func (m *Manager) ListSnapshots() ([]*Snapshot, error) {
	return m.snapshotMgr.List()
}

// RestoreSnapshot restores skills from a snapshot.
func (m *Manager) RestoreSnapshot(id string) error {
	skills, err := m.snapshotMgr.Restore(id)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.skills = skills
	m.mu.Unlock()

	m.log.Info("Restored skills from snapshot",
		zap.String("id", id),
		zap.Int("skills", len(skills)))

	return nil
}

// GetVersionHistory returns version history for a skill.
func (m *Manager) GetVersionHistory(skillID string) (*VersionHistory, error) {
	return m.versionMgr.GetHistory(skillID)
}

// GetSkillSources returns all configured skill source paths.
func (m *Manager) GetSkillSources() []SkillSource {
	return m.loader.GetSources()
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

// ListEligibleEnabled returns all enabled skills that meet system requirements.
func (m *Manager) ListEligibleEnabled() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*Skill, 0)
	for _, skill := range m.skills {
		if !skill.Enabled {
			continue
		}

		// Check eligibility
		eligible, reasons := m.eligibilityCheck.Check(skill)
		if !eligible {
			m.log.Debug("Skill not eligible",
				zap.String("skill", skill.ID),
				zap.Strings("reasons", reasons))
			continue
		}

		skills = append(skills, skill)
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
	m.log.Info("Skill enabled", zap.String("id", id))
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
	m.log.Info("Skill disabled", zap.String("id", id))
	return nil
}

// ListAlwaysEligible returns always-on skills that meet system requirements.
func (m *Manager) ListAlwaysEligible() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*Skill, 0)
	for _, skill := range m.skills {
		if !skill.Always || !skill.Enabled {
			continue
		}

		eligible, reasons := m.eligibilityCheck.Check(skill)
		if !eligible {
			m.log.Debug("Always skill not eligible",
				zap.String("skill", skill.ID),
				zap.Strings("reasons", reasons))
			continue
		}

		skills = append(skills, skill)
	}

	return skills
}

// GetInstructions returns the combined instructions for all enabled and eligible skills.
func (m *Manager) GetInstructions() string {
	eligibleEnabled := m.ListEligibleEnabled()
	regularSkills := make([]*Skill, 0, len(eligibleEnabled))
	for _, skill := range eligibleEnabled {
		if !skill.Always {
			regularSkills = append(regularSkills, skill)
		}
	}

	alwaysSkills := m.ListAlwaysEligible()
	if len(regularSkills) == 0 && len(alwaysSkills) == 0 {
		return ""
	}

	sortSkillsByName(regularSkills)
	sortSkillsByName(alwaysSkills)

	var sb strings.Builder

	if len(alwaysSkills) > 0 {
		sb.WriteString("# Always Skills\n\n")
		for i, skill := range alwaysSkills {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(formatSkillXML(skill))
		}
		sb.WriteString("\n\n")
	}

	if len(regularSkills) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString("# Available Skills\n\n")
		sb.WriteString(formatSkillSummaryXML(regularSkills))
		sb.WriteString("\n\n")
		sb.WriteString("Use the skill tool with action \"invoke\" and the skill_id to load detailed instructions when needed.")
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}

func sortSkillsByName(skills []*Skill) {
	sort.Slice(skills, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(skills[i].Name))
		right := strings.ToLower(strings.TrimSpace(skills[j].Name))
		if left == right {
			return skills[i].ID < skills[j].ID
		}
		return left < right
	})
}

func formatSkillSummaryXML(skills []*Skill) string {
	var sb strings.Builder
	sb.WriteString("<skills>\n")
	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf(
			"  <skill id=\"%s\" name=\"%s\" instructions_length=\"%s\">\n",
			xmlEscape(skill.ID),
			xmlEscape(skill.Name),
			strconv.Itoa(len([]rune(skill.Instructions))),
		))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", xmlEscape(skill.Description)))
		}
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</skills>")
	return sb.String()
}

func formatSkillXML(skill *Skill) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"<skill id=\"%s\" name=\"%s\" always=\"true\">\n",
		xmlEscape(skill.ID),
		xmlEscape(skill.Name),
	))
	if skill.Description != "" {
		sb.WriteString(fmt.Sprintf("  <description>%s</description>\n", xmlEscape(skill.Description)))
	}
	sb.WriteString("  <instructions>\n")
	sb.WriteString(xmlEscape(skill.Instructions))
	sb.WriteString("\n  </instructions>\n")
	sb.WriteString("</skill>")
	return sb.String()
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
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
		zap.String("skill", skill.ID),
		zap.Int("count", len(specs)))

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
