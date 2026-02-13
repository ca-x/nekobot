# Slash Commands

Nanobot supports slash commands across multiple channels (Telegram, Slack, Discord). Slash commands provide quick access to bot functions without sending regular messages.

## How It Works

The slash command system consists of:

1. **Command Registry** (`pkg/commands/registry.go`) - Central registry for all commands
2. **Command Handler** - Function that executes the command
3. **Channel Integration** - Each channel checks for and handles slash commands

## Built-in Commands

### /start
**Description:** Welcome message and introduction
**Usage:** `/start`

Shows a welcome message explaining the bot's capabilities.

### /help
**Description:** Show available commands
**Usage:** `/help [command]`

- Without arguments: Lists all available commands
- With command name: Shows detailed help for that command

**Examples:**
```
/help              # List all commands
/help status       # Show help for /status command
```

### /status
**Description:** Show bot status
**Usage:** `/status`

Displays the bot's current status and which channel you're using.

### /model
**Description:** List or switch AI models
**Usage:** `/model [provider]` or `/model list`

Manage AI provider/model selection:
- `/model` or `/model list` - Show all configured providers
- `/model <name>` - Switch to a specific provider (not yet implemented)

**Examples:**
```
/model              # List all available models
/model list         # Same as above
/model claude       # Switch to Claude provider
```

### /agent
**Description:** Switch agent or show agent info
**Usage:** `/agent [name]`

Manage agent selection:
- `/agent` or `/agent info` - Show current agent information
- `/agent list` - List all available agents
- `/agent <name>` - Switch to a specific agent (not yet implemented)

**Examples:**
```
/agent              # Show current agent
/agent list         # List all agents
/agent codex        # Switch to Codex agent
```

### /gateway
**Description:** Gateway management
**Usage:** `/gateway <action>`
**Admin Only:** Yes

Manage the gateway service:
- `/gateway` or `/gateway status` - Show gateway status and active channels
- `/gateway restart` - Restart gateway (not yet implemented)
- `/gateway reload` - Reload configuration (not yet implemented)

**Examples:**
```
/gateway            # Show status
/gateway status     # Show detailed status
/gateway restart    # Restart gateway
```

### /skill
**Description:** Execute or show skill information
**Usage:** `/<skillname> [args]`

Skills are dynamically registered as commands. Each loaded skill becomes a command:
- `/actionbook` - Execute actionbook skill
- `/github` - Execute github skill
- `/obsidian` - Execute obsidian skill
- etc.

Currently shows skill information. Direct execution is not yet implemented.

**Examples:**
```
/github             # Show github skill info
/obsidian projects  # Execute obsidian skill with args
```

## Channel Support

### Telegram
Commands start with `/` and are detected automatically:
- `/help` - Show available commands
- `/start` - Welcome message
- `/status` - Bot status

### Slack
Uses Slack's native slash command system:
- Commands must be registered in Slack App settings
- Supports ephemeral responses (visible only to you)
- Can use interactive components

### Discord
Discord slash commands can be registered as application commands:
- Commands appear in the Discord UI
- Supports autocomplete and options
- Native Discord integration

## Creating Custom Commands

To add a new command, register it with the command registry:

```go
import "nekobot/pkg/commands"

// Create the command
cmd := &commands.Command{
    Name:        "ping",
    Description: "Check if bot is responsive",
    Usage:       "/ping",
    Handler:     pingHandler,
}

// Register with registry
registry.Register(cmd)
```

### Command Handler

A command handler has this signature:

```go
func pingHandler(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
    return commands.CommandResponse{
        Content:     "🏓 Pong!",
        ReplyInline: true,  // Reply directly, don't send through agent
        Ephemeral:   false, // Visible to all (Slack only)
    }, nil
}
```

### CommandRequest Fields

- `Channel` - Channel name (telegram, discord, slack)
- `ChatID` - Conversation identifier
- `UserID` - User who invoked the command
- `Username` - Display name
- `Command` - Command name (without /)
- `Args` - Text after the command
- `Metadata` - Channel-specific metadata

### CommandResponse Fields

- `Content` - Response text (supports Markdown)
- `ReplyInline` - If true, reply directly; if false, send through agent
- `Ephemeral` - Slack only: if true, only visible to user

## Command Registration

Commands can be registered during startup via the FX lifecycle:

```go
func registerCustomCommands(registry *commands.Registry) error {
    return registry.Register(&commands.Command{
        Name:        "mycmd",
        Description: "My custom command",
        Handler:     myHandler,
    })
}
```

Add to FX modules:

```go
fx.Invoke(registerCustomCommands)
```

## Examples

### Simple Command

```go
func echoHandler(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
    if req.Args == "" {
        return commands.CommandResponse{
            Content:     "Usage: /echo <text>",
            ReplyInline: true,
        }, nil
    }

    return commands.CommandResponse{
        Content:     "Echo: " + req.Args,
        ReplyInline: true,
    }, nil
}
```

### Command with API Call

```go
func weatherHandler(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
    city := req.Args
    if city == "" {
        return commands.CommandResponse{
            Content:     "Usage: /weather <city>",
            ReplyInline: true,
        }, nil
    }

    // Call weather API
    weather, err := getWeather(ctx, city)
    if err != nil {
        return commands.CommandResponse{
            Content:     "❌ Failed to get weather: " + err.Error(),
            ReplyInline: true,
        }, nil
    }

    return commands.CommandResponse{
        Content:     fmt.Sprintf("🌤️ Weather in %s: %s", city, weather),
        ReplyInline: true,
    }, nil
}
```

### Ephemeral Response (Slack)

```go
func secretHandler(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
    return commands.CommandResponse{
        Content:     "🤫 This message is only visible to you!",
        ReplyInline: true,
        Ephemeral:   true, // Only works in Slack
    }, nil
}
```

## Best Practices

1. **Keep commands simple** - Complex operations should use the agent
2. **Validate input** - Check args before processing
3. **Use inline replies** - Most commands should reply directly (ReplyInline: true)
4. **Handle errors gracefully** - Return user-friendly error messages
5. **Document usage** - Include clear usage strings
6. **Use emojis sparingly** - Make output readable
7. **Support help** - Provide good descriptions for /help

## Limitations

- Command names must be lowercase
- No spaces in command names
- Commands starting with `/` are reserved
- Slack commands must be registered in Slack App settings
- Discord commands require bot permissions

## Architecture

```
User Message
     ↓
Channel Handler (Telegram/Slack/Discord)
     ↓
Command Registry (IsCommand?)
     ↓ (if command)
Command Handler
     ↓
Response → Channel
```

Regular messages bypass the command system and go directly to the agent.
