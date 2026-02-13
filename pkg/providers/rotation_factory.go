// Package providers provides factory functions for creating rotation managers from config.
package providers

import (
	"fmt"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// CreateRotationManagerFromConfig creates a RotationManager from provider configuration.
// NOTE: This function is deprecated as rotation has been removed in favor of simple fallback arrays.
func CreateRotationManagerFromConfig(
	log *logger.Logger,
	providerCfg config.ProviderProfile,
) (*RotationManager, error) {
	// Default cooldown
	cooldownDuration := 5 * time.Minute

	// Use round-robin strategy
	strategy := StrategyRoundRobin

	// Create rotation config
	rotationConfig := RotationConfig{
		Enabled:        false, // Rotation is disabled by default
		Strategy:       strategy,
		CooldownPeriod: cooldownDuration,
	}

	// Create rotation manager
	manager := NewRotationManager(log, rotationConfig)

	// Add single profile
	if providerCfg.APIKey != "" {
		profile := &Profile{
			Name:     providerCfg.Name,
			APIKey:   providerCfg.APIKey,
			Priority: 1,
		}
		manager.AddProfile(profile)
	} else {
		return nil, fmt.Errorf("no API key configured")
	}

	return manager, nil
}
