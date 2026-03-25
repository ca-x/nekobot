package typing

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"nekobot/pkg/wechat/client"
)

const (
	minTTL             = 12 * time.Hour
	maxTTL             = 24 * time.Hour
	initialBackoffTime = 2 * time.Second
	maxBackoffTime     = 1 * time.Hour
)

type cacheEntry struct {
	ticket          string
	expiresAt       time.Time
	backoffDuration time.Duration
}

// ConfigCache caches per-user typing tickets obtained from GetConfig.
type ConfigCache struct {
	client  configClient
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type configClient interface {
	GetConfig(ctx context.Context, userID, contextToken string) (*clientResponse, error)
}

type clientResponse struct {
	Ret          int
	ErrMsg       string
	TypingTicket string
}

type clientAdapter struct {
	client *client.Client
}

func (a clientAdapter) GetConfig(ctx context.Context, userID, contextToken string) (*clientResponse, error) {
	resp, err := a.client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		return nil, err
	}
	return &clientResponse{Ret: resp.Ret, ErrMsg: resp.ErrMsg, TypingTicket: resp.TypingTicket}, nil
}

// NewConfigCache creates a new ConfigCache.
func NewConfigCache(c *client.Client) *ConfigCache {
	return &ConfigCache{
		client:  clientAdapter{client: c},
		entries: make(map[string]*cacheEntry),
	}
}

// GetTicket returns a cached typing ticket for the user, or fetches a new one.
func (cc *ConfigCache) GetTicket(ctx context.Context, userID, contextToken string) (string, error) {
	cc.mu.RLock()
	entry, ok := cc.entries[userID]
	cc.mu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) && entry.ticket != "" {
		return entry.ticket, nil
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	entry, ok = cc.entries[userID]
	if ok && time.Now().Before(entry.expiresAt) && entry.ticket != "" {
		return entry.ticket, nil
	}

	if ok && entry.ticket == "" && time.Now().Before(entry.expiresAt) {
		return "", fmt.Errorf("getconfig for user %s: in backoff", userID)
	}

	cc.evictExpired()

	resp, err := cc.client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		cc.applyBackoff(userID, entry)
		return "", fmt.Errorf("getconfig for user %s: %w", userID, err)
	}
	if resp.Ret != 0 {
		cc.applyBackoff(userID, entry)
		return "", &client.APIError{Ret: resp.Ret, ErrMsg: resp.ErrMsg}
	}

	ttl := minTTL + time.Duration(rand.Int64N(int64(maxTTL-minTTL)))
	cc.entries[userID] = &cacheEntry{
		ticket:    resp.TypingTicket,
		expiresAt: time.Now().Add(ttl),
	}

	return resp.TypingTicket, nil
}

// Invalidate removes the cached ticket for a user.
func (cc *ConfigCache) Invalidate(userID string) {
	cc.mu.Lock()
	delete(cc.entries, userID)
	cc.mu.Unlock()
}

func (cc *ConfigCache) applyBackoff(userID string, existing *cacheEntry) {
	backoff := initialBackoffTime
	if existing != nil && existing.backoffDuration > 0 {
		backoff = min(existing.backoffDuration*2, maxBackoffTime)
	}
	cc.entries[userID] = &cacheEntry{
		expiresAt:       time.Now().Add(backoff),
		backoffDuration: backoff,
	}
}

func (cc *ConfigCache) evictExpired() {
	now := time.Now()
	for k, v := range cc.entries {
		if now.After(v.expiresAt) {
			delete(cc.entries, k)
		}
	}
}
