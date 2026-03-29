package memory

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"nekobot/pkg/config"
)

const minimumLearningWeight = 0.05

// LearningsCompressor compresses raw learnings into prompt-friendly markdown.
type LearningsCompressor struct {
	config config.LearningsConfig
}

// NewLearningsCompressor creates a compressor for the given config.
func NewLearningsCompressor(cfg config.LearningsConfig) *LearningsCompressor {
	if cfg.CompressedMaxSize <= 0 {
		cfg.CompressedMaxSize = defaultLearningsCompressedMaxSize
	}
	if cfg.HalfLifeDays <= 0 {
		cfg.HalfLifeDays = defaultLearningsHalfLifeDays
	}
	return &LearningsCompressor{config: cfg}
}

// Compress renders active learnings markdown.
func (c *LearningsCompressor) Compress(entries []LearningEntry) string {
	return c.compressAt(entries, time.Now().UTC())
}

func (c *LearningsCompressor) compressAt(entries []LearningEntry, now time.Time) string {
	if c == nil || len(entries) == 0 {
		return ""
	}

	type scoredEntry struct {
		entry  LearningEntry
		weight float64
	}

	scored := make([]scoredEntry, 0, len(entries))
	for _, entry := range entries {
		confidence := entry.Confidence
		if confidence <= 0 {
			confidence = 0.5
		}

		ageInDays := float64(now.Sub(entry.Timestamp).Milliseconds()) / dayMilliseconds
		if entry.Timestamp.IsZero() || ageInDays < 0 {
			ageInDays = 0
		}

		weight := applyDecayScore(confidence, ageInDays, c.config.HalfLifeDays)
		scored = append(scored, scoredEntry{
			entry:  entry,
			weight: weight,
		})
	}

	slices.SortFunc(scored, func(a, b scoredEntry) int {
		if delta := cmp.Compare(b.weight, a.weight); delta != 0 {
			return delta
		}
		return cmp.Compare(b.entry.Timestamp.Unix(), a.entry.Timestamp.Unix())
	})

	lines := make([]string, 0, len(scored)+1)
	lines = append(lines, "## Active Learnings")
	for _, item := range scored {
		if item.weight < minimumLearningWeight && len(lines) > 1 {
			continue
		}

		line := formatLearningBullet(item.entry)
		candidate := strings.Join(append(lines, line), "\n")
		if len(candidate) > c.config.CompressedMaxSize {
			break
		}
		lines = append(lines, line)
	}

	if len(lines) == 1 {
		first := formatLearningBullet(scored[0].entry)
		if len("## Active Learnings\n"+first) <= c.config.CompressedMaxSize {
			lines = append(lines, first)
		}
	}

	return strings.Join(lines, "\n")
}

func formatLearningBullet(entry LearningEntry) string {
	parts := make([]string, 0, 4)
	if entry.Category != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.Category))
	}
	if entry.Source != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Source))
	}
	prefix := strings.Join(parts, " ")
	if prefix != "" {
		prefix += " "
	}
	return "- " + prefix + strings.TrimSpace(entry.Content)
}
