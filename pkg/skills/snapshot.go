// Package skills provides skill snapshot functionality for debugging and rollback.
package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Snapshot represents an immutable snapshot of skill state.
type Snapshot struct {
	ID        string               `json:"id"`
	Timestamp time.Time            `json:"timestamp"`
	Skills    []SnapshotSkill      `json:"skills"`
	Metadata  map[string]string    `json:"metadata,omitempty"`
}

// SnapshotSkill represents a skill in a snapshot.
type SnapshotSkill struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	FilePath     string `json:"file_path"`
	ContentHash  string `json:"content_hash"`
	Instructions string `json:"instructions,omitempty"` // Optional: store full content
}

// SnapshotManager handles creating and restoring skill snapshots.
type SnapshotManager struct {
	log          *logger.Logger
	snapshotsDir string
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(log *logger.Logger, snapshotsDir string) *SnapshotManager {
	return &SnapshotManager{
		log:          log,
		snapshotsDir: snapshotsDir,
	}
}

// Create creates a snapshot of current skills.
func (sm *SnapshotManager) Create(skills map[string]*Skill, metadata map[string]string) (*Snapshot, error) {
	// Create snapshots directory if it doesn't exist
	if err := os.MkdirAll(sm.snapshotsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshots directory: %w", err)
	}

	// Build snapshot
	snapshot := &Snapshot{
		ID:        generateSnapshotID(),
		Timestamp: time.Now(),
		Skills:    make([]SnapshotSkill, 0, len(skills)),
		Metadata:  metadata,
	}

	// Add skills to snapshot
	for _, skill := range skills {
		hash := computeContentHash(skill.Instructions)
		snapshot.Skills = append(snapshot.Skills, SnapshotSkill{
			ID:           skill.ID,
			Name:         skill.Name,
			Version:      skill.Version,
			Enabled:      skill.Enabled,
			FilePath:     skill.FilePath,
			ContentHash:  hash,
			Instructions: skill.Instructions, // Store full content
		})
	}

	// Sort skills by ID for consistency
	sort.Slice(snapshot.Skills, func(i, j int) bool {
		return snapshot.Skills[i].ID < snapshot.Skills[j].ID
	})

	// Save snapshot to disk
	if err := sm.save(snapshot); err != nil {
		return nil, fmt.Errorf("saving snapshot: %w", err)
	}

	sm.log.Info("Created skill snapshot",
		zap.String("id", snapshot.ID),
		zap.Int("skills", len(snapshot.Skills)))

	return snapshot, nil
}

// List returns all available snapshots.
func (sm *SnapshotManager) List() ([]*Snapshot, error) {
	entries, err := os.ReadDir(sm.snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Snapshot{}, nil
		}
		return nil, fmt.Errorf("reading snapshots directory: %w", err)
	}

	snapshots := make([]*Snapshot, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(sm.snapshotsDir, entry.Name())
		snapshot, err := sm.load(path)
		if err != nil {
			sm.log.Warn("Failed to load snapshot",
				zap.String("file", entry.Name()),
				zap.Error(err))
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// Get retrieves a specific snapshot by ID.
func (sm *SnapshotManager) Get(id string) (*Snapshot, error) {
	path := filepath.Join(sm.snapshotsDir, id+".json")
	return sm.load(path)
}

// Delete removes a snapshot.
func (sm *SnapshotManager) Delete(id string) error {
	path := filepath.Join(sm.snapshotsDir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting snapshot: %w", err)
	}

	sm.log.Info("Deleted snapshot", zap.String("id", id))
	return nil
}

// Restore restores skills from a snapshot.
// Note: This returns the skill state from the snapshot but does not write files.
// The caller is responsible for deciding how to apply the restoration.
func (sm *SnapshotManager) Restore(id string) (map[string]*Skill, error) {
	snapshot, err := sm.Get(id)
	if err != nil {
		return nil, fmt.Errorf("loading snapshot: %w", err)
	}

	skills := make(map[string]*Skill)
	for _, ss := range snapshot.Skills {
		skill := &Skill{
			ID:           ss.ID,
			Name:         ss.Name,
			Version:      ss.Version,
			Enabled:      ss.Enabled,
			Instructions: ss.Instructions,
			FilePath:     ss.FilePath,
		}
		skills[skill.ID] = skill
	}

	sm.log.Info("Restored skills from snapshot",
		zap.String("id", id),
		zap.Int("skills", len(skills)))

	return skills, nil
}

// Compare compares current skills with a snapshot.
func (sm *SnapshotManager) Compare(currentSkills map[string]*Skill, snapshotID string) (*SnapshotComparison, error) {
	snapshot, err := sm.Get(snapshotID)
	if err != nil {
		return nil, err
	}

	comparison := &SnapshotComparison{
		SnapshotID: snapshotID,
		Added:      make([]string, 0),
		Removed:    make([]string, 0),
		Modified:   make([]string, 0),
		Unchanged:  make([]string, 0),
	}

	// Build snapshot skills map
	snapshotSkills := make(map[string]SnapshotSkill)
	for _, ss := range snapshot.Skills {
		snapshotSkills[ss.ID] = ss
	}

	// Check current skills
	for id, skill := range currentSkills {
		ss, exists := snapshotSkills[id]
		if !exists {
			comparison.Added = append(comparison.Added, id)
			continue
		}

		currentHash := computeContentHash(skill.Instructions)
		if currentHash != ss.ContentHash {
			comparison.Modified = append(comparison.Modified, id)
		} else {
			comparison.Unchanged = append(comparison.Unchanged, id)
		}
	}

	// Check for removed skills
	for id := range snapshotSkills {
		if _, exists := currentSkills[id]; !exists {
			comparison.Removed = append(comparison.Removed, id)
		}
	}

	return comparison, nil
}

// SnapshotComparison holds the result of comparing current state with a snapshot.
type SnapshotComparison struct {
	SnapshotID string
	Added      []string
	Removed    []string
	Modified   []string
	Unchanged  []string
}

// save saves a snapshot to disk.
func (sm *SnapshotManager) save(snapshot *Snapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling snapshot: %w", err)
	}

	path := filepath.Join(sm.snapshotsDir, snapshot.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing snapshot file: %w", err)
	}

	return nil
}

// load loads a snapshot from disk.
func (sm *SnapshotManager) load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
	}

	return &snapshot, nil
}

// Helper functions

// generateSnapshotID generates a unique snapshot ID.
func generateSnapshotID() string {
	return fmt.Sprintf("snapshot-%d", time.Now().Unix())
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
