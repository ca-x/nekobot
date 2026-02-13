// Package main is the entry point for nekobot CLI.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providers"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/tools"
	"nekobot/pkg/workspace"
)

const logo = "🤖"

var (
	configPath  string
	message     string
	sessionID   string
	debugMode   bool
	agentModel  string
	agentProv   string
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
	Long: `Start an interactive chat session with the nekobot agent.

Examples:
  # Interactive mode
  nekobot agent

  # One-shot mode
  nekobot agent -m "What is the weather like?"

  # Use specific session
  nekobot agent -s my-session

  # Use specific model/provider
  nekobot agent -m "Hello" --model claude-opus-4-6 --provider anthropic`,
	Run: runAgent,
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
	agentCmd.Flags().StringVarP(&sessionID, "session", "s", "cli:default", "session ID for conversation history")
	agentCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "enable debug mode")
	agentCmd.Flags().StringVar(&agentModel, "model", "", "override model")
	agentCmd.Flags().StringVar(&agentProv, "provider", "", "override provider")

	// Add commands
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(versionCmd)
}

func runAgent(cmd *cobra.Command, args []string) {
	if debugMode {
		fmt.Println("🔍 Debug mode enabled")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// If message is provided, run in one-shot mode
	if message != "" {
		runOneShot(ctx, cancel)
	} else {
		// Otherwise, start interactive mode
		runInteractive(ctx, cancel)
	}
}

func runOneShot(ctx context.Context, cancel context.CancelFunc) {
	app := fx.New(
		logger.Module,
		config.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,

		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, ag *agent.Agent, sm *session.Manager) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						defer cancel()

						// Get or create session
						sess, err := sm.GetOrCreate(sessionID)
						if err != nil {
							log.Error("Failed to get session", zap.Error(err))
							os.Exit(1)
						}

						// Process message
						response, err := ag.Chat(ctx, sess, message)
						if err != nil {
							log.Error("Chat failed", zap.Error(err))
							os.Exit(1)
						}

						fmt.Printf("\n%s %s\n", logo, response)
					}()
					return nil
				},
			})
		}),
		fx.NopLogger, // Suppress fx logs
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting agent: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()

	stopCtx := context.Background()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Printf("Error stopping agent: %v\n", err)
	}
}

func runInteractive(ctx context.Context, cancel context.CancelFunc) {
	fmt.Printf("%s Interactive mode (Ctrl+C to exit)\n\n", logo)

	app := fx.New(
		logger.Module,
		config.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,

		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, ag *agent.Agent, sm *session.Manager) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						defer cancel()

						// Get or create session
						sess, err := sm.GetOrCreate(sessionID)
						if err != nil {
							log.Error("Failed to get session", zap.Error(err))
							os.Exit(1)
						}

						// Run interactive loop
						if err := interactiveLoop(ctx, ag, sess); err != nil {
							log.Error("Interactive loop failed", zap.Error(err))
						}
					}()
					return nil
				},
			})
		}),
		fx.NopLogger, // Suppress fx logs
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting agent: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()

	stopCtx := context.Background()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Printf("Error stopping agent: %v\n", err)
	}
}

func interactiveLoop(ctx context.Context, ag *agent.Agent, sess *session.Session) error {
	prompt := fmt.Sprintf("%s You: ", logo)

	// Try to use readline for better UX
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".nekobot_history"),
		HistoryLimit:    100,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	if err != nil {
		fmt.Printf("Warning: readline not available, using simple mode\n")
		return simpleInteractiveLoop(ctx, ag, sess)
	}
	defer rl.Close()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nGoodbye!")
			return nil
		default:
		}

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return nil
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Process message
		response, err := ag.Chat(ctx, sess, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", logo, response)
	}
}

func simpleInteractiveLoop(ctx context.Context, ag *agent.Agent, sess *session.Session) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nGoodbye!")
			return nil
		default:
		}

		fmt.Print(fmt.Sprintf("%s You: ", logo))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return nil
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Process message
		response, err := ag.Chat(ctx, sess, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", logo, response)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
