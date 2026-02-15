package agent

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	_ "nekobot/pkg/providers/init" // Register all providers
	"nekobot/pkg/providerstore"
	"nekobot/pkg/skills"
)

// Module provides agent for fx dependency injection.
var Module = fx.Module("agent",
	fx.Provide(ProvideAgent),
)

func errNoProviderConfigured() error {
	return fmt.Errorf("no provider configured")
}

type provideAgentDeps struct {
	fx.In

	Cfg           *config.Config
	Log           *logger.Logger
	SkillsMgr     *skills.Manager
	ProcessMgr    *process.Manager
	ApprovalMgr   *approval.Manager
	LC            fx.Lifecycle
	ProviderStore *providerstore.Manager `optional:"true"`
}

// ProvideAgent provides an agent instance.
func ProvideAgent(deps provideAgentDeps) (*Agent, error) {
	cfg := deps.Cfg
	log := deps.Log
	skillsMgr := deps.SkillsMgr
	processMgr := deps.ProcessMgr
	approvalMgr := deps.ApprovalMgr
	lc := deps.LC
	_ = deps.ProviderStore // Ensure provider store initializes first when module is present.

	// Get provider config
	providerName := strings.TrimSpace(cfg.Agents.Defaults.Provider)
	if providerName == "" && len(cfg.Providers) > 0 {
		providerName = strings.TrimSpace(cfg.Providers[0].Name)
	}

	providerCfg := cfg.GetProviderConfig(providerName)
	if providerCfg == nil {
		log.Warn("Provider not found, using first configured provider",
			zap.String("provider", providerName),
		)
		if len(cfg.Providers) == 0 {
			return nil, errNoProviderConfigured()
		}
		providerName = strings.TrimSpace(cfg.Providers[0].Name)
		providerCfg = cfg.GetProviderConfig(providerName)
	}
	if providerCfg == nil {
		return nil, errNoProviderConfigured()
	}
	providerKind := strings.TrimSpace(providerCfg.ProviderKind)
	if providerKind == "" {
		providerKind = providerName
	}

	// Create provider client
	client, err := providers.NewClient(providerKind, &providers.RelayInfo{
		ProviderName: providerName,
		APIKey:       providerCfg.APIKey,
		APIBase:      providerCfg.APIBase,
		Model:        cfg.Agents.Defaults.Model,
		Proxy:        providerCfg.Proxy,
		Timeout:      providerCfg.GetTimeout(),
	})
	if err != nil {
		return nil, err
	}

	// Create agent with process manager
	agent, err := New(cfg, log, client, processMgr, approvalMgr)
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
				zap.String("provider_kind", providerKind),
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
