package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RegisterBuiltinCommands registers built-in commands.
func RegisterBuiltinCommands(registry *Registry) error {
	builtins := []*Command{
		{
			Name:        "help",
			Description: "Show available commands",
			Usage:       "/help [command]",
			Handler:     helpHandler(registry),
		},
		{
			Name:        "start",
			Description: "Start interaction with the bot",
			Usage:       "/start",
			Handler:     startHandler,
		},
		{
			Name:        "status",
			Description: "Show bot status",
			Usage:       "/status",
			Handler:     statusHandler,
		},
	}

	for _, cmd := range builtins {
		if err := registry.Register(cmd); err != nil {
			return fmt.Errorf("failed to register %s: %w", cmd.Name, err)
		}
	}

	return nil
}

// helpHandler creates a handler for the /help command.
func helpHandler(registry *Registry) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		// If a specific command is requested, show detailed help
		if req.Args != "" {
			parts := strings.Fields(req.Args)
			if len(parts) > 0 {
				cmdName := strings.TrimPrefix(parts[0], "/")
				if at := strings.Index(cmdName, "@"); at > 0 {
					cmdName = cmdName[:at]
				}
				cmd, exists := registry.Get(cmdName)
				if exists {
					content := fmt.Sprintf("**/%s**\n\n%s\n\n**Usage:** %s",
						cmd.Name, cmd.Description, cmd.Usage)

					return CommandResponse{
						Content:     content,
						ReplyInline: true,
					}, nil
				}
			}
		}

		// Show all commands
		cmds := registry.List()
		if len(cmds) == 0 {
			return CommandResponse{
				Content:     "No commands available.",
				ReplyInline: true,
			}, nil
		}

		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name < cmds[j].Name
		})

		// Limited-interaction channels prefer plain slash list.
		if strings.EqualFold(strings.TrimSpace(req.Channel), "serverchan") {
			var sb strings.Builder
			sb.WriteString("可用命令：\n\n")
			for _, cmd := range cmds {
				desc := strings.TrimSpace(cmd.Description)
				if desc == "" {
					desc = "Command"
				}
				sb.WriteString(fmt.Sprintf("/%s - %s\n", cmd.Name, desc))
			}
			sb.WriteString("\n提示：普通文本会进入 AI 对话。")
			return CommandResponse{Content: sb.String(), ReplyInline: true}, nil
		}

		var sb strings.Builder
		sb.WriteString("🤖 **Available Commands**\n\n")

		for _, cmd := range cmds {
			sb.WriteString(fmt.Sprintf("**/%s** - %s\n", cmd.Name, cmd.Description))
		}

		sb.WriteString("\nUse `/help [command]` for detailed information.")

		return CommandResponse{
			Content:     sb.String(),
			ReplyInline: true,
		}, nil
	}
}

// startHandler handles the /start command.
func startHandler(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	content := `👋 **Welcome to Nanobot!**

I'm an AI assistant that can help you with various tasks.

Type /help to see available commands, or just send me a message to start chatting!`

	return CommandResponse{
		Content:     content,
		ReplyInline: true,
	}, nil
}

// statusHandler handles the /status command.
func statusHandler(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	content := fmt.Sprintf(`✅ **Nanobot Status**

Channel: %s
Status: 🟢 Online

Ready to assist you!`, req.Channel)

	return CommandResponse{
		Content:     content,
		ReplyInline: true,
	}, nil
}
