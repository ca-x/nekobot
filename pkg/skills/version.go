// Package skills provides skill versioning and change tracking.
package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// SkillVersion represents a version of a skill at a point in time.
type SkillVersion struct {
	SkillID      string    `json:"skill_id"`
	Version      string    `json:"version"`
	Timestamp    time.Time `json:"timestamp"`
	ContentHash  string    `json:"content_hash"`
	ChangeType   string    `json:"change_type"` // created, modified, deleted
	ChangeSummary string   `json:"change_summary,omitempty"`
}

// VersionHistory holds the version history for a skill.
type VersionHistory struct {
	SkillID  string          `json:"skill_id"`
	Versions []SkillVersion  `json:"versions"`
}

// VersionManager tracks skill versions over time.
type VersionManager struct {
	log        *logger.Logger
	versionsDir string
	histories   map[string]*VersionHistory // skillID -> history
}

// NewVersionManager creates a new version manager.
func NewVersionManager(log *logger.Logger, versionsDir string) *VersionManager {
	return &VersionManager{
		log:        log,
		versionsDir: versionsDir,
		histories:   make(map[string]*VersionHistory),
	}
}

// Initialize loads existing version histories from disk.
func (vm *VersionManager) Initialize() error {
	if err := os.MkdirAll(vm.versionsDir, 0755); err != nil {
		return fmt.Errorf("creating versions directory: %w", err)
	}

	entries, err := os.ReadDir(vm.versionsDir)
	if err != nil {
		return fmt.Errorf("reading versions directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(vm.versionsDir, entry.Name())
		history, err := vm.loadHistory(path)
		if err != nil {
			vm.log.Warn("Failed to load version history",
				zap.String("file", entry.Name()),
				zap.Error(err))
			continue
		}

		vm.histories[history.SkillID] = history
	}

	vm.log.Info("Loaded version histories",
		zap.Int("count", len(vm.histories)))

	return nil
}

// TrackChange records a change to a skill.
func (vm *VersionManager) TrackChange(skill *Skill, changeType, changeSummary string) error {
	history, exists := vm.histories[skill.ID]
	if !exists {
		history = &VersionHistory{
			SkillID:  skill.ID,
			Versions: make([]SkillVersion, 0),
		}
		vm.histories[skill.ID] = history
	}

	version := SkillVersion{
		SkillID:       skill.ID,
		Version:       skill.Version,
		Timestamp:     time.Now(),
		ContentHash:   computeContentHash(skill.Instructions),
		ChangeType:    changeType,
		ChangeSummary: changeSummary,
	}

	history.Versions = append(history.Versions, version)

	// Save updated history
	if err := vm.saveHistory(history); err != nil {
		return fmt.Errorf("saving version history: %w", err)
	}

	vm.log.Debug("Tracked skill change",
		zap.String("skill", skill.ID),
		zap.String("type", changeType))

	return nil
}

// GetHistory returns the version history for a skill.
func (vm *VersionManager) GetHistory(skillID string) (*VersionHistory, error) {
	history, exists := vm.histories[skillID]
	if !exists {
		return nil, fmt.Errorf("no version history for skill: %s", skillID)
	}

	return history, nil
}

// GetLatestVersion returns the latest version entry for a skill.
func (vm *VersionManager) GetLatestVersion(skillID string) (*SkillVersion, error) {
	history, err := vm.GetHistory(skillID)
	if err != nil {
		return nil, err
	}

	if len(history.Versions) == 0 {
		return nil, fmt.Errorf("no versions recorded for skill: %s", skillID)
	}

	return &history.Versions[len(history.Versions)-1], nil
}

// DetectChanges compares current skills with their last known versions.
func (vm *VersionManager) DetectChanges(skills map[string]*Skill) map[string]string {
	changes := make(map[string]string)

	for id, skill := range skills {
		currentHash := computeContentHash(skill.Instructions)

		latest, err := vm.GetLatestVersion(id)
		if err != nil {
			// No version history - this is a new skill
			changes[id] = "created"
			continue
		}

		if latest.ContentHash != currentHash {
			changes[id] = "modified"
		}
	}

	// Check for deleted skills
	for id := range vm.histories {
		if _, exists := skills[id]; !exists {
			changes[id] = "deleted"
		}
	}

	return changes
}

// ListVersions returns all versions of a skill, sorted by timestamp.
func (vm *VersionManager) ListVersions(skillID string) ([]SkillVersion, error) {
	history, err := vm.GetHistory(skillID)
	if err != nil {
		return nil, err
	}

	// Create a copy and sort by timestamp (newest first)
	versions := make([]SkillVersion, len(history.Versions))
	copy(versions, history.Versions)

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})

	return versions, nil
}

// GetChangesSince returns all changes since a specific timestamp.
func (vm *VersionManager) GetChangesSince(since time.Time) map[string][]SkillVersion {
	changes := make(map[string][]SkillVersion)

	for skillID, history := range vm.histories {
		skillChanges := make([]SkillVersion, 0)
		for _, version := range history.Versions {
			if version.Timestamp.After(since) {
				skillChanges = append(skillChanges, version)
			}
		}

		if len(skillChanges) > 0 {
			changes[skillID] = skillChanges
		}
	}

	return changes
}

// Prune removes old version entries, keeping only the most recent N versions per skill.
func (vm *VersionManager) Prune(keepCount int) error {
	for _, history := range vm.histories {
		if len(history.Versions) <= keepCount {
			continue
		}

		// Sort by timestamp (oldest first for pruning)
		sort.Slice(history.Versions, func(i, j int) bool {
			return history.Versions[i].Timestamp.Before(history.Versions[j].Timestamp)
		})

		// Keep only the most recent N
		history.Versions = history.Versions[len(history.Versions)-keepCount:]

		// Save pruned history
		if err := vm.saveHistory(history); err != nil {
			return fmt.Errorf("saving pruned history: %w", err)
		}

		vm.log.Debug("Pruned version history",
			zap.String("skill", history.SkillID),
			zap.Int("kept", keepCount))
	}

	return nil
}

// saveHistory saves a version history to disk.
func (vm *VersionManager) saveHistory(history *VersionHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling history: %w", err)
	}

	path := filepath.Join(vm.versionsDir, history.SkillID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing history file: %w", err)
	}

	return nil
}

// loadHistory loads a version history from disk.
func (vm *VersionManager) loadHistory(path string) (*VersionHistory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading history file: %w", err)
	}

	var history VersionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("unmarshaling history: %w", err)
	}

	return &history, nil
}
