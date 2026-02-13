// Package providers provides API key rotation and profile management.
package providers

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// RotationStrategy defines how profiles are selected.
type RotationStrategy string

const (
	// StrategyRoundRobin cycles through profiles in order.
	StrategyRoundRobin RotationStrategy = "round_robin"

	// StrategyLeastUsed selects the profile with the lowest request count.
	StrategyLeastUsed RotationStrategy = "least_used"

	// StrategyRandom randomly selects from available profiles.
	StrategyRandom RotationStrategy = "random"
)

// Profile represents an API key profile.
type Profile struct {
	Name         string
	APIKey       string
	Priority     int
	RequestCount int64
	CooldownUntil time.Time
	LastError    *ErrorClassification
}

// IsAvailable checks if the profile is not on cooldown.
func (p *Profile) IsAvailable() bool {
	return time.Now().After(p.CooldownUntil)
}

// SetCooldown puts the profile on cooldown for the specified duration.
func (p *Profile) SetCooldown(duration time.Duration, classification ErrorClassification) {
	p.CooldownUntil = time.Now().Add(duration)
	p.LastError = &classification
}

// IncrementRequests increments the request count.
func (p *Profile) IncrementRequests() {
	p.RequestCount++
}

// RotationConfig holds rotation configuration.
type RotationConfig struct {
	Enabled         bool
	Strategy        RotationStrategy
	CooldownPeriod  time.Duration
}

// RotationManager manages API key rotation and failover.
type RotationManager struct {
	log             *logger.Logger
	config          RotationConfig
	profiles        []*Profile
	currentIndex    int
	mu              sync.RWMutex
	rand            *rand.Rand
}

// NewRotationManager creates a new rotation manager.
func NewRotationManager(log *logger.Logger, config RotationConfig) *RotationManager {
	return &RotationManager{
		log:      log,
		config:   config,
		profiles: make([]*Profile, 0),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddProfile adds a profile to the rotation pool.
func (rm *RotationManager) AddProfile(profile *Profile) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.profiles = append(rm.profiles, profile)
	rm.log.Debug("Added profile to rotation",
		zap.String("name", profile.Name),
		zap.Int("priority", profile.Priority))
}

// GetNextProfile returns the next available profile based on the rotation strategy.
func (rm *RotationManager) GetNextProfile() (*Profile, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(rm.profiles) == 0 {
		return nil, fmt.Errorf("no profiles available")
	}

	// If rotation is disabled, always use the first profile
	if !rm.config.Enabled {
		if rm.profiles[0].IsAvailable() {
			return rm.profiles[0], nil
		}
		return nil, fmt.Errorf("primary profile on cooldown")
	}

	// Get available profiles
	available := rm.getAvailableProfiles()
	if len(available) == 0 {
		return nil, fmt.Errorf("all profiles on cooldown")
	}

	// Select based on strategy
	var selected *Profile
	switch rm.config.Strategy {
	case StrategyRoundRobin:
		selected = rm.selectRoundRobin(available)
	case StrategyLeastUsed:
		selected = rm.selectLeastUsed(available)
	case StrategyRandom:
		selected = rm.selectRandom(available)
	default:
		selected = available[0]
	}

	return selected, nil
}

// HandleError processes an error and potentially puts the profile on cooldown.
func (rm *RotationManager) HandleError(profile *Profile, err error, statusCode int) {
	classification := ClassifyError(err, statusCode)

	rm.log.Warn("Profile error",
		zap.String("profile", profile.Name),
		zap.String("reason", string(classification.Reason)),
		zap.String("message", classification.Message),
		zap.Bool("cooldown", classification.ShouldCooldown))

	if classification.ShouldCooldown {
		rm.mu.Lock()
		profile.SetCooldown(rm.config.CooldownPeriod, classification)
		rm.mu.Unlock()

		rm.log.Info("Profile put on cooldown",
			zap.String("profile", profile.Name),
			zap.Duration("duration", rm.config.CooldownPeriod),
			zap.Time("until", profile.CooldownUntil))
	}
}

// RecordSuccess records a successful request for a profile.
func (rm *RotationManager) RecordSuccess(profile *Profile) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	profile.IncrementRequests()
	profile.LastError = nil
}

// GetStatus returns the status of all profiles.
func (rm *RotationManager) GetStatus() []ProfileStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	statuses := make([]ProfileStatus, len(rm.profiles))
	for i, p := range rm.profiles {
		statuses[i] = ProfileStatus{
			Name:         p.Name,
			Available:    p.IsAvailable(),
			RequestCount: p.RequestCount,
			CooldownUntil: p.CooldownUntil,
			LastError:    p.LastError,
		}
	}

	return statuses
}

// ProfileStatus holds the status of a profile.
type ProfileStatus struct {
	Name         string
	Available    bool
	RequestCount int64
	CooldownUntil time.Time
	LastError    *ErrorClassification
}

// getAvailableProfiles returns all profiles not on cooldown.
func (rm *RotationManager) getAvailableProfiles() []*Profile {
	available := make([]*Profile, 0)
	for _, p := range rm.profiles {
		if p.IsAvailable() {
			available = append(available, p)
		}
	}
	return available
}

// selectRoundRobin selects the next profile in round-robin order.
func (rm *RotationManager) selectRoundRobin(available []*Profile) *Profile {
	if len(available) == 0 {
		return nil
	}

	// Find the next profile after currentIndex
	for i := 0; i < len(rm.profiles); i++ {
		idx := (rm.currentIndex + i) % len(rm.profiles)
		profile := rm.profiles[idx]

		// Check if this profile is in available list
		for _, avail := range available {
			if profile.Name == avail.Name {
				rm.currentIndex = (idx + 1) % len(rm.profiles)
				return profile
			}
		}
	}

	// Fallback to first available
	rm.currentIndex = 0
	return available[0]
}

// selectLeastUsed selects the profile with the lowest request count.
func (rm *RotationManager) selectLeastUsed(available []*Profile) *Profile {
	if len(available) == 0 {
		return nil
	}

	leastUsed := available[0]
	for _, p := range available[1:] {
		if p.RequestCount < leastUsed.RequestCount {
			leastUsed = p
		}
	}

	return leastUsed
}

// selectRandom randomly selects from available profiles.
func (rm *RotationManager) selectRandom(available []*Profile) *Profile {
	if len(available) == 0 {
		return nil
	}

	idx := rm.rand.Intn(len(available))
	return available[idx]
}

// Reset resets all profiles (clears cooldowns and request counts).
func (rm *RotationManager) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, p := range rm.profiles {
		p.RequestCount = 0
		p.CooldownUntil = time.Time{}
		p.LastError = nil
	}

	rm.log.Info("Reset all profiles")
}

// GetProfileByName returns a profile by name.
func (rm *RotationManager) GetProfileByName(name string) (*Profile, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, p := range rm.profiles {
		if p.Name == name {
			return p, nil
		}
	}

	return nil, fmt.Errorf("profile not found: %s", name)
}

// GetProfileCount returns the total number of profiles.
func (rm *RotationManager) GetProfileCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.profiles)
}

// GetAvailableCount returns the number of available (not on cooldown) profiles.
func (rm *RotationManager) GetAvailableCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.getAvailableProfiles())
}
