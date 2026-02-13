package commands

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/skills"
)

// ChannelManager interface to avoid circular dependency with channels package.
type ChannelManager interface {
	GetEnabledChannels() []Channel
}

// Channel interface for basic channel information.
type Channel interface {
	Name() string
	ID() string
}

// Dependencies holds dependencies needed for advanced commands.
type Dependencies struct {
	Config         *config.Config
	Agent          *agent.Agent
	SkillsManager  *skills.Manager
	ChannelManager ChannelManager
}

// RegisterAdvancedCommands registers advanced commands that require dependencies.
func RegisterAdvancedCommands(registry *Registry, deps Dependencies) error {
	advancedCmds := []*Command{
		{
			Name:        "model",
			Description: "List or switch AI models",
			Usage:       "/model [provider] or /model list",
			Handler:     modelHandler(deps.Config),
		},
		{
			Name:        "gateway",
			Description: "Gateway management (restart, status)",
			Usage:       "/gateway <action>",
			Handler:     gatewayHandler(deps.ChannelManager),
			AdminOnly:   true,
		},
		{
			Name:        "agent",
			Description: "Switch agent or show agent info",
			Usage:       "/agent [name]",
			Handler:     agentHandler(deps.Config),
		},
	}

	for _, cmd := range advancedCmds {
		if err := registry.Register(cmd); err != nil {
			return fmt.Errorf("failed to register %s: %w", cmd.Name, err)
		}
	}

	// Register skill commands dynamically
	if deps.SkillsManager != nil {
		if err := registerSkillCommands(registry, deps.SkillsManager); err != nil {
			return fmt.Errorf("failed to register skill commands: %w", err)
		}
	}

	return nil
}

// modelHandler handles the /model command.
func modelHandler(cfg *config.Config) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		args := strings.TrimSpace(req.Args)

		// List models
		if args == "" || args == "list" {
			var sb strings.Builder
			sb.WriteString("🤖 **Available Providers**\n\n")

			// List all configured providers
			providers := []struct {
				Name   string
				Config config.ProviderConfig
			}{
				{"anthropic", cfg.Providers.Anthropic},
				{"openai", cfg.Providers.OpenAI},
				{"openrouter", cfg.Providers.OpenRouter},
				{"groq", cfg.Providers.Groq},
				{"zhipu", cfg.Providers.Zhipu},
				{"vllm", cfg.Providers.VLLM},
				{"gemini", cfg.Providers.Gemini},
				{"nvidia", cfg.Providers.Nvidia},
				{"moonshot", cfg.Providers.Moonshot},
				{"deepseek", cfg.Providers.DeepSeek},
			}

			hasProviders := false
			for _, p := range providers {
				if p.Config.APIKey != "" {
					hasProviders = true
					sb.WriteString(fmt.Sprintf("**%s**\n", p.Name))
					if p.Config.APIBase != "" {
						sb.WriteString(fmt.Sprintf("  Base: %s\n", p.Config.APIBase))
					}
					if p.Config.Rotation.Enabled {
						sb.WriteString("  Rotation: ✅ Enabled\n")
					}
					sb.WriteString("\n")
				}
			}

			if !hasProviders {
				sb.WriteString("No providers configured.\n")
			}

			sb.WriteString("\nUse `/model <provider>` to get provider info.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// Show provider info
		providerName := strings.ToLower(args)
		var providerConfig config.ProviderConfig
		found := false

		switch providerName {
		case "anthropic":
			providerConfig = cfg.Providers.Anthropic
			found = providerConfig.APIKey != ""
		case "openai":
			providerConfig = cfg.Providers.OpenAI
			found = providerConfig.APIKey != ""
		case "openrouter":
			providerConfig = cfg.Providers.OpenRouter
			found = providerConfig.APIKey != ""
		case "groq":
			providerConfig = cfg.Providers.Groq
			found = providerConfig.APIKey != ""
		case "zhipu":
			providerConfig = cfg.Providers.Zhipu
			found = providerConfig.APIKey != ""
		case "vllm":
			providerConfig = cfg.Providers.VLLM
			found = providerConfig.APIKey != ""
		case "gemini":
			providerConfig = cfg.Providers.Gemini
			found = providerConfig.APIKey != ""
		case "nvidia":
			providerConfig = cfg.Providers.Nvidia
			found = providerConfig.APIKey != ""
		case "moonshot":
			providerConfig = cfg.Providers.Moonshot
			found = providerConfig.APIKey != ""
		case "deepseek":
			providerConfig = cfg.Providers.DeepSeek
			found = providerConfig.APIKey != ""
		}

		if !found {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Provider '%s' not found or not configured. Use `/model list` to see available providers.", providerName),
				ReplyInline: true,
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🤖 **Provider: %s**\n\n", providerName))
		if providerConfig.APIBase != "" {
			sb.WriteString(fmt.Sprintf("Base URL: %s\n", providerConfig.APIBase))
		}
		if providerConfig.Rotation.Enabled {
			sb.WriteString(fmt.Sprintf("Rotation: ✅ Enabled (%s)\n", providerConfig.Rotation.Strategy))
			sb.WriteString(fmt.Sprintf("Profiles: %d\n", len(providerConfig.Profiles)))
		}

		return CommandResponse{
			Content:     sb.String(),
			ReplyInline: true,
		}, nil
	}
}

// gatewayHandler handles the /gateway command.
func gatewayHandler(channelMgr ChannelManager) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		args := strings.TrimSpace(req.Args)

		if args == "" || args == "status" {
			// Show gateway status
			var sb strings.Builder
			sb.WriteString("🌐 **Gateway Status**\n\n")

			channels := channelMgr.GetEnabledChannels()
			sb.WriteString(fmt.Sprintf("Active Channels: %d\n\n", len(channels)))

			for _, ch := range channels {
				sb.WriteString(fmt.Sprintf("• **%s** - %s\n", ch.Name(), ch.ID()))
			}

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		switch args {
		case "restart":
			// Note: Actual restart would require service control
			return CommandResponse{
				Content:     "⚠️ Gateway restart is not yet implemented.\n\nThis requires integration with system service control.",
				ReplyInline: true,
			}, nil

		case "reload":
			return CommandResponse{
				Content:     "⚠️ Configuration reload is not yet implemented.\n\nThis requires hot-reload support.",
				ReplyInline: true,
			}, nil

		default:
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Unknown gateway action: %s\n\nAvailable: status, restart, reload", args),
				ReplyInline: true,
			}, nil
		}
	}
}

// agentHandler handles the /agent command.
func agentHandler(cfg *config.Config) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		args := strings.TrimSpace(req.Args)

		// Show current agent info
		if args == "" || args == "info" {
			var sb strings.Builder
			sb.WriteString("🤖 **Agent Information**\n\n")

			// Show default provider
			defaultProvider := cfg.Agents.Defaults.Provider
			if defaultProvider == "" {
				defaultProvider = "Not configured"
			}

			sb.WriteString(fmt.Sprintf("Default Provider: **%s**\n", defaultProvider))
			sb.WriteString(fmt.Sprintf("Model: %s\n", cfg.Agents.Defaults.Model))
			sb.WriteString(fmt.Sprintf("Max Tokens: %d\n", cfg.Agents.Defaults.MaxTokens))
			sb.WriteString(fmt.Sprintf("Temperature: %.2f\n", cfg.Agents.Defaults.Temperature))

			sb.WriteString("\nUse `/agent list` to see all available providers.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// List available agents/providers
		if args == "list" {
			var sb strings.Builder
			sb.WriteString("🤖 **Available Providers**\n\n")

			providers := []string{
				"anthropic", "openai", "openrouter", "groq",
				"zhipu", "vllm", "gemini", "nvidia",
				"moonshot", "deepseek",
			}

			currentProvider := cfg.Agents.Defaults.Provider
			for _, p := range providers {
				prefix := "  "
				if p == currentProvider {
					prefix = "→ " // Current
				}
				sb.WriteString(fmt.Sprintf("%s**%s**\n", prefix, p))
			}

			sb.WriteString("\nUse `/model <provider>` to see provider details.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// Show provider info
		return CommandResponse{
			Content:     "ℹ️ Use `/model list` to see available providers and their configuration.",
			ReplyInline: true,
		}, nil
	}
}

// registerSkillCommands registers commands for all loaded skills.
func registerSkillCommands(registry *Registry, skillsMgr *skills.Manager) error {
	allSkills := skillsMgr.List()

	for _, skill := range allSkills {
		// Create a command for this skill
		skillName := skill.Name
		cmd := &Command{
			Name:        skillName,
			Description: fmt.Sprintf("Execute %s skill", skillName),
			Usage:       fmt.Sprintf("/%s [args]", skillName),
			Handler:     skillHandler(skillsMgr, skillName),
		}

		// Try to register (ignore if already exists)
		if err := registry.Register(cmd); err != nil {
			// Skill name might conflict with builtin command, skip
			continue
		}
	}

	return nil
}

// skillHandler creates a handler for executing a skill.
func skillHandler(skillsMgr *skills.Manager, skillName string) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		// Get the skill
		skill, err := skillsMgr.Get(skillName)
		if err != nil || skill == nil {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Skill '%s' not found.", skillName),
				ReplyInline: true,
			}, nil
		}

		// Return skill info for now
		// TODO: Actual skill execution would require agent integration
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🔧 **Skill: %s**\n\n", skill.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", skill.Description))

		if skill.Instructions != "" {
			preview := skill.Instructions
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("**Instructions Preview:**\n%s\n\n", preview))
		}

		sb.WriteString("ℹ️ Direct skill execution from commands is not yet implemented.\n")
		sb.WriteString("Skills are automatically available to the agent during conversations.")

		return CommandResponse{
			Content:     sb.String(),
			ReplyInline: true,
		}, nil
	}
}
