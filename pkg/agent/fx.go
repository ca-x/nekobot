package agent

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providers"
	"nekobot/pkg/skills"
	_ "nekobot/pkg/providers/init" // Register all providers
)

// Module provides agent for fx dependency injection.
var Module = fx.Module("agent",
	fx.Provide(ProvideAgent),
)

// ProvideAgent provides an agent instance.
func ProvideAgent(
	cfg *config.Config,
	log *logger.Logger,
	skillsMgr *skills.Manager,
	lc fx.Lifecycle,
) (*Agent, error) {
	// Get provider config
	providerName := cfg.Agents.Defaults.Provider
	if providerName == "" {
		providerName = "claude" // default
	}

	providerCfg := cfg.GetProviderConfig(providerName)
	if providerCfg == nil {
		log.Warn("Provider not found, using default",
			zap.String("provider", providerName),
		)
		providerName = "claude"
		providerCfg = cfg.GetProviderConfig(providerName)
	}

	// Create provider client
	client, err := providers.NewClient(providerName, &providers.RelayInfo{
		ProviderName: providerName,
		APIKey:       providerCfg.APIKey,
		APIBase:      providerCfg.APIBase,
		Model:        cfg.Agents.Defaults.Model,
	})
	if err != nil {
		return nil, err
	}

	// Create agent
	agent, err := New(cfg, log, client)
	if err != nil {
		return nil, err
	}

	// Set skills manager on context builder
	agent.context.SetSkillsManager(skillsMgr)

	// Register skill tool
	agent.RegisterSkillTool(skillsMgr)

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Agent initialized",
				zap.String("provider", providerName),
				zap.String("model", cfg.Agents.Defaults.Model),
				zap.String("workspace", cfg.WorkspacePath()),
				zap.Int("skills_total", len(skillsMgr.ListEnabled())),
				zap.Int("skills_eligible", len(skillsMgr.ListEligibleEnabled())),
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Agent shutting down")
			return nil
		},
	})

	return agent, nil
}
