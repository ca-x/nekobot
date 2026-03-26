package memory

import (
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const dayMilliseconds = 24 * 60 * 60 * 1000

var datedMemoryPathPattern = regexp.MustCompile(`^(?:.*/)?memory/(\d{4})-(\d{2})-(\d{2})\.md$`)

// ApplyTemporalDecay applies time-aware decay to search results.
func ApplyTemporalDecay(results []*SearchResult, config TemporalDecayConfig) []*SearchResult {
	return applyTemporalDecayToResults(results, config, time.Now())
}

func applyTemporalDecayToResults(results []*SearchResult, config TemporalDecayConfig, now time.Time) []*SearchResult {
	if !config.Enabled || len(results) == 0 {
		return results
	}

	halfLifeDays := config.HalfLifeDays
	if halfLifeDays <= 0 {
		halfLifeDays = 30
	}

	for _, result := range results {
		if result == nil {
			continue
		}
		timestamp := extractMemoryTimestamp(result)
		if timestamp == nil {
			result.AgeInDays = 0
			continue
		}

		ageInDays := float64(now.Sub(*timestamp).Milliseconds()) / dayMilliseconds
		if ageInDays < 0 {
			ageInDays = 0
		}
		result.AgeInDays = ageInDays
		result.Score = applyDecayScore(result.Score, ageInDays, halfLifeDays)
	}

	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j] != nil && results[i] != nil && results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

func applyDecayScore(score, ageInDays, halfLifeDays float64) float64 {
	return score * decayMultiplier(ageInDays, halfLifeDays)
}

func decayMultiplier(ageInDays, halfLifeDays float64) float64 {
	lambda := decayLambda(halfLifeDays)
	if lambda <= 0 {
		return 1
	}
	return math.Exp(-lambda * ageInDays)
}

func decayLambda(halfLifeDays float64) float64 {
	if halfLifeDays <= 0 || math.IsInf(halfLifeDays, 0) || math.IsNaN(halfLifeDays) {
		return 0
	}
	return math.Ln2 / halfLifeDays
}

func extractMemoryTimestamp(result *SearchResult) *time.Time {
	if result == nil {
		return nil
	}
	if result.Metadata.Timestamp != nil {
		return result.Metadata.Timestamp
	}
	if path := result.Metadata.FilePath; path != "" {
		if result.Source == SourceLongTerm && isEvergreenMemoryPath(path) {
			return nil
		}
		if parsed := parseMemoryDateFromPath(path); parsed != nil {
			return parsed
		}
	}
	return &result.CreatedAt
}

func parseMemoryDateFromPath(filePath string) *time.Time {
	matches := datedMemoryPathPattern.FindStringSubmatch(filepath.ToSlash(filePath))
	if len(matches) < 4 {
		return nil
	}

	year, yearErr := strconv.Atoi(matches[1])
	month, monthErr := strconv.Atoi(matches[2])
	day, dayErr := strconv.Atoi(matches[3])
	if yearErr != nil || monthErr != nil || dayErr != nil {
		return nil
	}

	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &value
}

func isEvergreenMemoryPath(filePath string) bool {
	normalized := filepath.ToSlash(filePath)
	base := filepath.Base(normalized)
	if base == "MEMORY.md" || base == "memory.md" {
		return true
	}
	return filepath.Dir(normalized) == "memory" && !datedMemoryPathPattern.MatchString(normalized)
}
