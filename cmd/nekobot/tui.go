package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/tools"
	"nekobot/pkg/workspace"
)

var tuiSessionID string

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start a minimal terminal UI chat",
	Long:  "Start a minimal Bubble Tea based chat interface for the nekobot agent.",
	Run:   runTUI,
}

func init() {
	tuiCmd.Flags().StringVarP(&tuiSessionID, "session", "s", "cli:tui", "session ID for conversation history")
	rootCmd.AddCommand(tuiCmd)
}

type aiResponseMsg struct {
	text string
	err  error
}

type tuiModel struct {
	ctx      context.Context
	agent    *agent.Agent
	session  *session.Session
	input    textinput.Model
	messages []string
	waiting  bool
	provider string
	model    string
}

func newTUIModel(ctx context.Context, ag *agent.Agent, sess *session.Session, provider, model string) tuiModel {
	input := textinput.New()
	input.Placeholder = "Type a message and press Enter"
	input.Focus()
	input.Prompt = "> "

	return tuiModel{
		ctx:      ctx,
		agent:    ag,
		session:  sess,
		input:    input,
		messages: []string{"Welcome to nekobot TUI. Press Ctrl+C to exit."},
		provider: provider,
		model:    model,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.waiting {
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			m.messages = append(m.messages, "You: "+prompt)
			m.input.SetValue("")
			m.waiting = true
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
				defer cancel()
				resp, err := m.agent.Chat(ctx, m.session, prompt)
				return aiResponseMsg{text: resp, err: err}
			}
		}
	case aiResponseMsg:
		m.waiting = false
		if msg.err != nil {
			m.messages = append(m.messages, "Bot: ❌ "+msg.err.Error())
		} else {
			m.messages = append(m.messages, "Bot: "+msg.text)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Nekobot TUI | Provider: %s | Model: %s\n", m.provider, m.model))
	b.WriteString(strings.Repeat("-", 80))
	b.WriteString("\n")

	start := 0
	if len(m.messages) > 20 {
		start = len(m.messages) - 20
	}
	for _, line := range m.messages[start:] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if m.waiting {
		b.WriteString("\nThinking...\n")
	}
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	return b.String()
}

func runTUI(cmd *cobra.Command, args []string) {
	cfgPath, created, err := config.InitDefaultConfig()
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to initialize config: %v\n", err)
	} else if created {
		fmt.Printf("✅ Created default config at: %s\n", cfgPath)
		fmt.Println("📝 Please edit the config file to add your API keys")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

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
		process.Module,
		agent.Module,

		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, ag *agent.Agent, sm *session.Manager, cfg *config.Config) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go func() {
						defer cancel()

						sess, err := sm.Get(tuiSessionID)
						if err != nil {
							log.Error("Failed to get session", zap.Error(err))
							return
						}

						model := newTUIModel(ctx, ag, sess, cfg.Agents.Defaults.Provider, cfg.Agents.Defaults.Model)
						if _, err := tea.NewProgram(model).Run(); err != nil {
							log.Error("TUI terminated with error", zap.Error(err))
						}
					}()
					return nil
				},
			})
		}),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()

	stopCtx := context.Background()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Printf("Error stopping TUI: %v\n", err)
	}
}
