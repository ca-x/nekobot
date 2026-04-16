package goaldriven

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/goaldriven/criteria"
	"nekobot/pkg/goaldriven/scope"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

type provideServiceDeps struct {
	fx.In

	Store    Store
	Parser   *criteria.Parser
	Schema   *criteria.Schema
	Resolver *scope.Resolver
	Log      *logger.Logger
	Agent    *agent.Agent `optional:"true"`
	KV       state.KV     `optional:"true"`
}

func provideService(deps provideServiceDeps) *Service {
	svc := NewService(deps.Store, deps.Parser, deps.Schema, deps.Resolver)
	svc.SetLogger(deps.Log)
	svc.SetAgent(deps.Agent)
	svc.SetKVStore(deps.KV)
	return svc
}

func registerLifecycle(lc fx.Lifecycle, svc *Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return svc.ResumeActiveRuns(ctx)
		},
	})
}

// Module provides GoalDriven with persistent store + resume hooks.
var Module = fx.Module("goaldriven",
	fx.Provide(
		NewPersistentStore,
		criteria.NewParser,
		criteria.NewSchema,
		scope.NewResolver,
		provideService,
	),
	fx.Invoke(registerLifecycle),
)
