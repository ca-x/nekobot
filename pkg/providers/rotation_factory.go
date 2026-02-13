// Package providers provides factory functions for creating rotation managers from config.
package providers

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// CreateRotationManagerFromConfig creates a RotationManager from provider configuration.
func CreateRotationManagerFromConfig(
	log *logger.Logger,
	providerCfg config.ProviderConfig,
) (*RotationManager, error) {
	// Parse cooldown duration
	cooldownDuration := 5 * time.Minute // Default
	if providerCfg.Rotation.Cooldown != "" {
		duration, err := time.ParseDuration(providerCfg.Rotation.Cooldown)
		if err != nil {
			return nil, fmt.Errorf("parsing cooldown duration: %w", err)
		}
		cooldownDuration = duration
	}

	// Determine rotation strategy
	var strategy RotationStrategy
	switch providerCfg.Rotation.Strategy {
	case "round_robin", "":
		strategy = StrategyRoundRobin
	case "least_used":
		strategy = StrategyLeastUsed
	case "random":
		strategy = StrategyRandom
	default:
		return nil, fmt.Errorf("unknown rotation strategy: %s", providerCfg.Rotation.Strategy)
	}

	// Create rotation config
	rotationConfig := RotationConfig{
		Enabled:        providerCfg.Rotation.Enabled,
		Strategy:       strategy,
		CooldownPeriod: cooldownDuration,
	}

	// Create rotation manager
	manager := NewRotationManager(log, rotationConfig)

	// Add profiles
	if len(providerCfg.Profiles) > 0 {
		// Add configured profiles
		for name, profileCfg := range providerCfg.Profiles {
			profile := &Profile{
				Name:     name,
				APIKey:   profileCfg.APIKey,
				Priority: profileCfg.Priority,
			}
			manager.AddProfile(profile)
		}
	} else if providerCfg.APIKey != "" {
		// No profiles configured, use main API key as default profile
		profile := &Profile{
			Name:     "default",
			APIKey:   providerCfg.APIKey,
			Priority: 1,
		}
		manager.AddProfile(profile)
	} else {
		return nil, fmt.Errorf("no API key or profiles configured")
	}

	log.Info("Created rotation manager",
		zap.String("strategy", string(rotationConfig.Strategy)),
		zap.Int("profiles", manager.GetProfileCount()),
		zap.Duration("cooldown", rotationConfig.CooldownPeriod))

	return manager, nil
}

