package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"nekobot/pkg/config"
	"nekobot/pkg/fileutil"
)

const (
	defaultLearningsMaxRawEntries     = 500
	defaultLearningsCompressedMaxSize = 10000
	defaultLearningsHalfLifeDays      = 30
)

// LearningEntry represents one append-only learning record.
type LearningEntry struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Content    string                 `json:"content"`
	Category   string                 `json:"category,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Source     string                 `json:"source,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// LearningsManager manages append-only learnings storage and active summary refresh.
type LearningsManager struct {
	mu         sync.Mutex
	enabled    bool
	rawPath    string
	activePath string
	config     config.LearningsConfig
	compressor *LearningsCompressor
}

// NewLearningsManager creates a learnings manager from configuration.
func NewLearningsManager(cfg *config.Config) (*LearningsManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("learnings config is required")
	}

	workspace := cfg.WorkspacePath()
	memoryDir := filepath.Join(workspace, "memory")

	learningsCfg := cfg.Learnings
	if learningsCfg.MaxRawEntries <= 0 {
		learningsCfg.MaxRawEntries = defaultLearningsMaxRawEntries
	}
	if learningsCfg.CompressedMaxSize <= 0 {
		learningsCfg.CompressedMaxSize = defaultLearningsCompressedMaxSize
	}
	if learningsCfg.HalfLifeDays <= 0 {
		learningsCfg.HalfLifeDays = defaultLearningsHalfLifeDays
	}

	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure learnings directory %s: %w", memoryDir, err)
	}

	return &LearningsManager{
		enabled:    learningsCfg.Enabled,
		rawPath:    filepath.Join(memoryDir, "learnings.jsonl"),
		activePath: filepath.Join(memoryDir, "active_learnings.md"),
		config:     learningsCfg,
		compressor: NewLearningsCompressor(learningsCfg),
	}, nil
}

// Add appends a learning entry and refreshes the active summary.
func (m *LearningsManager) Add(entry LearningEntry) error {
	if m == nil {
		return fmt.Errorf("learnings manager is nil")
	}
	if !m.enabled {
		return nil
	}

	normalized, err := normalizeLearningEntry(entry)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.append(normalized); err != nil {
		return err
	}

	entries, err := m.listLocked()
	if err != nil {
		return err
	}

	if len(entries) > m.config.MaxRawEntries {
		entries = entries[len(entries)-m.config.MaxRawEntries:]
	}

	active := m.compressor.Compress(entries)
	if err := fileutil.WriteFileAtomic(m.activePath, []byte(active), 0o644); err != nil {
		return fmt.Errorf("write active learnings %s: %w", m.activePath, err)
	}

	return nil
}

// List returns all stored learning entries.
func (m *LearningsManager) List() ([]LearningEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("learnings manager is nil")
	}
	if !m.enabled {
		return []LearningEntry{}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listLocked()
}

// ReadActive returns the compressed active learnings markdown.
func (m *LearningsManager) ReadActive() string {
	if m == nil || !m.enabled {
		return ""
	}

	data, err := os.ReadFile(m.activePath)
	if err != nil {
		return ""
	}
	return string(data)
}

func (m *LearningsManager) append(entry LearningEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal learning entry: %w", err)
	}

	file, err := os.OpenFile(m.rawPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open learnings jsonl %s: %w", m.rawPath, err)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append learnings jsonl %s: %w", m.rawPath, err)
	}
	return nil
}

func (m *LearningsManager) listLocked() ([]LearningEntry, error) {
	file, err := os.Open(m.rawPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LearningEntry{}, nil
		}
		return nil, fmt.Errorf("open learnings jsonl %s: %w", m.rawPath, err)
	}
	defer file.Close()

	entries := make([]LearningEntry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry LearningEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("decode learning entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan learnings jsonl %s: %w", m.rawPath, err)
	}

	return entries, nil
}

func normalizeLearningEntry(entry LearningEntry) (LearningEntry, error) {
	entry.Content = strings.TrimSpace(entry.Content)
	entry.Category = strings.TrimSpace(entry.Category)
	entry.Source = strings.TrimSpace(entry.Source)

	if entry.Content == "" {
		return LearningEntry{}, fmt.Errorf("learning content is required")
	}
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}
	if entry.Confidence < 0 {
		entry.Confidence = 0
	}
	if entry.Confidence > 1 {
		entry.Confidence = 1
	}
	if entry.Metadata == nil {
		entry.Metadata = map[string]interface{}{}
	}

	return entry, nil
}
