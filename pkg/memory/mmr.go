package memory

import (
	"math"
	"regexp"
	"strings"
)

var mmrTokenPattern = regexp.MustCompile(`[a-z0-9_]+`)

type mmrItem struct {
	id     string
	score  float64
	tokens map[string]struct{}
}

// ApplyMMR re-ranks search results using Maximal Marginal Relevance.
func ApplyMMR(results []*SearchResult, config MMRConfig) []*SearchResult {
	if !config.Enabled || len(results) < 2 {
		return results
	}

	lambda := config.Lambda
	if lambda < 0 {
		lambda = 0
	} else if lambda > 1 {
		lambda = 1
	}

	items := make([]mmrItem, 0, len(results))
	byID := make(map[string]*SearchResult, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		items = append(items, mmrItem{
			id:     result.ID,
			score:  result.Score,
			tokens: tokenizeMMR(result.Text),
		})
		byID[result.ID] = result
	}
	if len(items) < 2 {
		return results
	}

	selected := make([]mmrItem, 0, len(items))
	remaining := append([]mmrItem(nil), items...)
	for len(remaining) > 0 {
		bestIndex := -1
		bestScore := math.Inf(-1)
		for i, candidate := range remaining {
			score := lambda*candidate.score - (1-lambda)*maxMMRSimilarity(candidate, selected)
			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}
		if bestIndex < 0 {
			break
		}
		selected = append(selected, remaining[bestIndex])
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}

	reordered := make([]*SearchResult, 0, len(selected))
	for _, item := range selected {
		if result := byID[item.id]; result != nil {
			reordered = append(reordered, result)
		}
	}
	return reordered
}

func tokenizeMMR(text string) map[string]struct{} {
	matches := mmrTokenPattern.FindAllString(strings.ToLower(text), -1)
	tokens := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		tokens[match] = struct{}{}
	}
	return tokens
}

func maxMMRSimilarity(item mmrItem, selected []mmrItem) float64 {
	maxSimilarity := 0.0
	for _, picked := range selected {
		similarity := jaccardSimilarity(item.tokens, picked.tokens)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}
	return maxSimilarity
}

func jaccardSimilarity(left, right map[string]struct{}) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1
	}
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	intersection := 0
	for token := range left {
		if _, ok := right[token]; ok {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
