// Package main is the entry point for nekobot CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
	"nekobot/pkg/workspace"
)

var (
	configPath string
	message    string
	session    string
)

var rootCmd = &cobra.Command{
	Use:   "nekobot",
	Short: "nekobot - A lightweight AI assistant",
	Long: `nekobot is a lightweight, extensible AI assistant that can help you with
various tasks including file operations, command execution, and more.`,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start interactive agent chat",
	Long:  `Start an interactive chat session with the nekobot agent.`,
	Run:   runAgent,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("nekobot v0.10.0-alpha")
		fmt.Println("Phase 10: QMD Integration (Complete)")
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")

	// Agent command flags
	agentCmd.Flags().StringVarP(&message, "message", "m", "", "send a single message (non-interactive)")
	agentCmd.Flags().StringVarP(&session, "session", "s", "default", "session ID for conversation history")

	// Add commands
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(versionCmd)
}

func runAgent(cmd *cobra.Command, args []string) {
	// If message is provided, run in one-shot mode
	if message != "" {
		runOneShot()
		return
	}

	// Otherwise, start interactive mode
	runInteractive()
}

func runOneShot() {
	app := fx.New(
		logger.Module,
		config.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, ag *agent.Agent) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					response, err := ag.Chat(ctx, message)
					if err != nil {
						log.Error("Chat failed", zap.Error(err))
						return err
					}

					fmt.Println(response)
					return nil
				},
			})
		}),
		fx.NopLogger, // Suppress fx logs in one-shot mode
	)

	app.Run()
}

func runInteractive() {
	fmt.Println("Interactive mode not yet implemented")
	fmt.Println("Use: nekobot agent -m \"your message here\"")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
