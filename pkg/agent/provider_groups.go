package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providerregistry"
	"nekobot/pkg/providers"
)

func providerConfigUsable(profile *config.ProviderProfile) bool {
	if profile == nil {
		return false
	}
	if strings.TrimSpace(profile.Name) == "" || strings.TrimSpace(profile.ProviderKind) == "" {
		return false
	}
	if meta, ok := providerregistry.Get(strings.TrimSpace(profile.ProviderKind)); ok {
		for _, field := range meta.AuthFields {
			if !field.Required {
				continue
			}
			switch field.Key {
			case "api_key":
				if strings.TrimSpace(profile.APIKey) == "" {
					return false
				}
			}
		}
	}
	return true
}

type providerGroupPlanner struct {
	mu       sync.Mutex
	managers map[string]*providers.RotationManager
}

func newProviderGroupPlanner() *providerGroupPlanner {
	return &providerGroupPlanner{
		managers: make(map[string]*providers.RotationManager),
	}
}

func (p *providerGroupPlanner) expand(
	cfg *config.Config,
	log *logger.Logger,
	primary string,
	fallback []string,
) ([]string, error) {
	if p == nil {
		p = newProviderGroupPlanner()
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	seen := make(map[string]struct{})
	order := make([]string, 0, 1+len(fallback))

	add := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		order = append(order, trimmed)
	}

	expandName := func(name string) error {
		group := resolveProviderGroup(cfg, name)
		if group == nil {
			add(name)
			return nil
		}

		members, err := p.planGroup(cfg, log, *group)
		if err != nil {
			return err
		}
		for _, member := range members {
			add(member)
		}
		return nil
	}

	if err := expandName(primary); err != nil {
		return nil, err
	}
	for _, name := range fallback {
		if err := expandName(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

func (p *providerGroupPlanner) planGroup(
	cfg *config.Config,
	log *logger.Logger,
	group config.ProviderGroupConfig,
) ([]string, error) {
	manager, err := p.getManager(log, group)
	if err != nil {
		return nil, err
	}

	candidates := normalizeProviderMembers(group.Members)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("provider group %s has no members", group.Name)
	}

	ordered := make([]string, 0, len(candidates))
	used := make(map[string]struct{}, len(candidates))

	first, err := manager.GetNextProfile()
	if err == nil && first != nil {
		ordered = append(ordered, first.Name)
		used[first.Name] = struct{}{}
	}

	for _, name := range candidates {
		if cfg.GetProviderConfig(name) == nil {
			return nil, fmt.Errorf("provider group %s member not found: %s", group.Name, name)
		}
		if _, ok := used[name]; ok {
			continue
		}
		ordered = append(ordered, name)
	}

	return ordered, nil
}

func (p *providerGroupPlanner) recordSuccess(providerName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, manager := range p.managers {
		profile, err := manager.GetProfileByName(providerName)
		if err != nil {
			continue
		}
		manager.RecordSuccess(profile)
	}
}

func (p *providerGroupPlanner) recordFailure(providerName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, manager := range p.managers {
		profile, getErr := manager.GetProfileByName(providerName)
		if getErr != nil {
			continue
		}
		manager.HandleError(profile, err, 0)
	}
}

func (p *providerGroupPlanner) getManager(
	log *logger.Logger,
	group config.ProviderGroupConfig,
) (*providers.RotationManager, error) {
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return nil, fmt.Errorf("provider group name is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if manager, ok := p.managers[name]; ok {
		return manager, nil
	}

	manager := providers.NewRotationManager(
		log,
		providers.RotationConfig{
			Enabled:        true,
			Strategy:       rotationStrategyFromConfig(group.Strategy),
			CooldownPeriod: 5 * time.Minute,
		},
	)
	for idx, member := range normalizeProviderMembers(group.Members) {
		manager.AddProfile(&providers.Profile{
			Name:     member,
			Priority: idx + 1,
		})
	}

	p.managers[name] = manager
	return manager, nil
}

func resolveProviderGroup(cfg *config.Config, name string) *config.ProviderGroupConfig {
	if cfg == nil {
		return nil
	}
	trimmed := strings.TrimSpace(name)
	for i := range cfg.Agents.Defaults.ProviderGroups {
		group := &cfg.Agents.Defaults.ProviderGroups[i]
		if strings.TrimSpace(group.Name) == trimmed {
			return group
		}
	}
	return nil
}

func normalizeProviderMembers(members []string) []string {
	seen := make(map[string]struct{}, len(members))
	out := make([]string, 0, len(members))
	for _, member := range members {
		trimmed := strings.TrimSpace(member)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func rotationStrategyFromConfig(raw string) providers.RotationStrategy {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "least_used":
		return providers.StrategyLeastUsed
	case "random":
		return providers.StrategyRandom
	default:
		return providers.StrategyRoundRobin
	}
}
