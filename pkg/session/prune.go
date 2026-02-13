package session

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// PruneStrategy defines how sessions should be pruned.
type PruneStrategy string

const (
	// PruneStrategyLRU removes least recently used sessions
	PruneStrategyLRU PruneStrategy = "lru"
	// PruneStrategyLFU removes least frequently used sessions
	PruneStrategyLFU PruneStrategy = "lfu"
	// PruneStrategyTTL removes sessions past their TTL
	PruneStrategyTTL PruneStrategy = "ttl"
	// PruneStrategySize removes largest sessions first
	PruneStrategySize PruneStrategy = "size"
)

// PruneConfig configures session pruning behavior.
type PruneConfig struct {
	Strategy         PruneStrategy
	MaxTotalSessions int
	MaxTotalMessages int
	MaxSessionAge    time.Duration // For TTL strategy
	PreserveCount    int           // Minimum messages to preserve per session
}

// DefaultPruneConfig returns default pruning configuration.
func DefaultPruneConfig() PruneConfig {
	return PruneConfig{
		Strategy:         PruneStrategyLRU,
		MaxTotalSessions: 1000,
		MaxTotalMessages: 10000,
		MaxSessionAge:    30 * 24 * time.Hour, // 30 days
		PreserveCount:    50,
	}
}

// Pruner manages session pruning operations.
type Pruner struct {
	manager *Manager
	config  PruneConfig
	stats   PruneStats
	mu      sync.RWMutex
}

// PruneStats contains pruning statistics.
type PruneStats struct {
	TotalPrunes    int64
	MessagesPruned int64
	SessionsPruned int64
	LastPruneAt    time.Time
}

// NewPruner creates a new session pruner.
func NewPruner(manager *Manager, config PruneConfig) *Pruner {
	return &Pruner{
		manager: manager,
		config:  config,
	}
}

// Prune prunes sessions based on the configured strategy.
func (p *Pruner) Prune() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.config.Strategy {
	case PruneStrategyLRU:
		return p.pruneLRU()
	case PruneStrategyLFU:
		return p.pruneLFU()
	case PruneStrategyTTL:
		return p.pruneTTL()
	case PruneStrategySize:
		return p.pruneSize()
	default:
		return fmt.Errorf("unknown prune strategy: %s", p.config.Strategy)
	}
}

// pruneLRU removes least recently used sessions.
func (p *Pruner) pruneLRU() error {
	sessions, err := p.listAllSessions()
	if err != nil {
		return err
	}

	if len(sessions) <= p.config.MaxTotalSessions {
		return nil // No pruning needed
	}

	// Sort by UpdatedAt (oldest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.Before(sessions[j].UpdatedAt)
	})

	// Remove oldest sessions
	toRemove := len(sessions) - p.config.MaxTotalSessions
	for i := 0; i < toRemove; i++ {
		if err := p.manager.DeleteJSONL(sessions[i].Key); err != nil {
			return fmt.Errorf("deleting session %s: %w", sessions[i].Key, err)
		}

		p.stats.SessionsPruned++
		p.stats.MessagesPruned += int64(len(sessions[i].Messages))
	}

	p.stats.TotalPrunes++
	p.stats.LastPruneAt = time.Now()

	return nil
}

// pruneLFU removes least frequently used sessions.
func (p *Pruner) pruneLFU() error {
	sessions, err := p.listAllSessions()
	if err != nil {
		return err
	}

	if len(sessions) <= p.config.MaxTotalSessions {
		return nil
	}

	// Sort by message count (least messages first, as proxy for frequency)
	sort.Slice(sessions, func(i, j int) bool {
		return len(sessions[i].Messages) < len(sessions[j].Messages)
	})

	// Remove sessions with fewest messages
	toRemove := len(sessions) - p.config.MaxTotalSessions
	for i := 0; i < toRemove; i++ {
		if err := p.manager.DeleteJSONL(sessions[i].Key); err != nil {
			return fmt.Errorf("deleting session %s: %w", sessions[i].Key, err)
		}

		p.stats.SessionsPruned++
		p.stats.MessagesPruned += int64(len(sessions[i].Messages))
	}

	p.stats.TotalPrunes++
	p.stats.LastPruneAt = time.Now()

	return nil
}

// pruneTTL removes sessions past their TTL.
func (p *Pruner) pruneTTL() error {
	sessions, err := p.listAllSessions()
	if err != nil {
		return err
	}

	now := time.Now()
	cutoff := now.Add(-p.config.MaxSessionAge)

	for _, session := range sessions {
		if session.UpdatedAt.Before(cutoff) {
			if err := p.manager.DeleteJSONL(session.Key); err != nil {
				return fmt.Errorf("deleting session %s: %w", session.Key, err)
			}

			p.stats.SessionsPruned++
			p.stats.MessagesPruned += int64(len(session.Messages))
		}
	}

	p.stats.TotalPrunes++
	p.stats.LastPruneAt = time.Now()

	return nil
}

// pruneSize removes largest sessions first.
func (p *Pruner) pruneSize() error {
	sessions, err := p.listAllSessions()
	if err != nil {
		return err
	}

	// Calculate total messages
	totalMessages := 0
	for _, session := range sessions {
		totalMessages += len(session.Messages)
	}

	if totalMessages <= p.config.MaxTotalMessages {
		return nil // No pruning needed
	}

	// Sort by message count (most messages first)
	sort.Slice(sessions, func(i, j int) bool {
		return len(sessions[i].Messages) > len(sessions[j].Messages)
	})

	// Remove largest sessions until under limit
	for _, session := range sessions {
		if totalMessages <= p.config.MaxTotalMessages {
			break
		}

		// Preserve minimum messages
		if len(session.Messages) <= p.config.PreserveCount {
			continue
		}

		if err := p.manager.DeleteJSONL(session.Key); err != nil {
			return fmt.Errorf("deleting session %s: %w", session.Key, err)
		}

		totalMessages -= len(session.Messages)
		p.stats.SessionsPruned++
		p.stats.MessagesPruned += int64(len(session.Messages))
	}

	p.stats.TotalPrunes++
	p.stats.LastPruneAt = time.Now()

	return nil
}

// PruneMessages prunes messages from a specific session, keeping only the most recent ones.
func (p *Pruner) PruneMessages(key string, keepCount int) error {
	session, err := p.manager.LoadJSONL(key)
	if err != nil {
		return err
	}

	if len(session.Messages) <= keepCount {
		return nil // No pruning needed
	}

	// Keep only the last N messages
	pruned := len(session.Messages) - keepCount
	session.Messages = session.Messages[pruned:]

	// Save back
	if err := p.manager.SaveJSONL(key, session.Messages, session.Metadata); err != nil {
		return err
	}

	p.stats.MessagesPruned += int64(pruned)
	return nil
}

// GetStats returns pruning statistics.
func (p *Pruner) GetStats() PruneStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// listAllSessions lists all sessions with their metadata.
func (p *Pruner) listAllSessions() ([]*SessionJSONL, error) {
	keys, err := p.manager.ListJSONL()
	if err != nil {
		return nil, err
	}

	sessions := make([]*SessionJSONL, 0, len(keys))
	for _, key := range keys {
		session, err := p.manager.LoadJSONL(key)
		if err != nil {
			continue // Skip invalid sessions
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}
