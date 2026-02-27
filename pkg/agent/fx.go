package agent

import (
	"context"
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
	"nekobot/pkg/toolsessions"
)

// Module provides agent for fx dependency injection.
var Module = fx.Module("agent",
	fx.Provide(ProvideAgent),
)

type provideAgentDeps struct {
	fx.In

	Cfg           *config.Config
	Log           *logger.Logger
	SkillsMgr     *skills.Manager
	ProcessMgr    *process.Manager
	ApprovalMgr   *approval.Manager
	ToolSessMgr   *toolsessions.Manager `optional:"true"`
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
	toolSessMgr := deps.ToolSessMgr
	lc := deps.LC
	_ = deps.ProviderStore // Ensure provider store initializes first when module is present.

	// Get provider config
	providerName := strings.TrimSpace(cfg.Agents.Defaults.Provider)
	if providerName == "" && len(cfg.Providers) > 0 {
		providerName = strings.TrimSpace(cfg.Providers[0].Name)
	}

	var client *providers.Client
	var providerKind string

	providerCfg := cfg.GetProviderConfig(providerName)
	if providerCfg == nil && len(cfg.Providers) > 0 {
		log.Warn("Provider not found, using first configured provider",
			zap.String("provider", providerName),
		)
		providerName = strings.TrimSpace(cfg.Providers[0].Name)
		providerCfg = cfg.GetProviderConfig(providerName)
	}

	if providerCfg == nil {
		// No provider configured yet — start with a nil client.
		// Chat will fail at runtime with a clear error until a provider is added.
		log.Warn("No provider configured; agent will not be able to chat until a provider is added")
	} else {
		providerKind = strings.TrimSpace(providerCfg.ProviderKind)
		if providerKind == "" {
			providerKind = providerName
		}

		var err error
		client, err = providers.NewClient(providerKind, &providers.RelayInfo{
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
	}

	// Create agent with process manager
	agent, err := New(cfg, log, client, processMgr, approvalMgr, toolSessMgr)
	if err != nil {
		return nil, err
	}

	// Set skills manager on context builder
	agent.context.SetSkillsManager(skillsMgr)

	// Register skill tool
	agent.RegisterSkillTool(skillsMgr)

	orchestrator := strings.TrimSpace(strings.ToLower(cfg.Agents.Defaults.Orchestrator))
	if orchestrator == "" {
		orchestrator = orchestratorBlades
	}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Agent initialized",
				zap.String("provider", providerName),
				zap.String("provider_kind", providerKind),
				zap.String("orchestrator", orchestrator),
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
