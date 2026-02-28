package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	acp "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/state"
	"nekobot/pkg/tools"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/workspace"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run ACP agent over stdio",
	Long: `Run nekobot as an ACP (Agent Client Protocol) agent over stdio.

This command is intended to be launched by ACP-compatible clients.
It keeps running until the peer disconnects or the process receives a termination signal.`,
	Run: runACP,
}

func init() {
	rootCmd.AddCommand(acpCmd)
}

func runACP(cmd *cobra.Command, args []string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		state.Module,
		process.Module,
		approval.Module,
		toolsessions.Module,
		providerstore.Module,
		agent.Module,
		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, ag *agent.Agent) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						adapter := agent.NewACPAdapter(ag)
						conn := acp.NewAgentSideConnection(adapter, os.Stdout, os.Stdin)
						adapter.SetAgentConnection(conn)
						log.Info("ACP server started")

						select {
						case <-ctx.Done():
							log.Info("ACP context cancelled")
						case <-conn.Done():
							log.Info("ACP peer disconnected")
						}
						cancel()
					}()
					return nil
				},
			})
		}),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting ACP agent: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()

	stopCtx := context.Background()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping ACP agent: %v\n", err)
	}
}
