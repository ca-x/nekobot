# nekobot 🤖

A lightweight, extensible AI assistant built with Go. nekobot provides a clean architecture for LLM-powered agents with tool orchestration, session management, and multi-provider support.

## Features

✅ **Unified Provider System** - Support for OpenAI, Claude, Gemini, and 10+ providers
✅ **Tool System** - Extensible tools for file operations, command execution, and more
✅ **Session Management** - Persistent conversation history
✅ **Memory System** - Long-term memory and daily notes
✅ **Configuration Management** - Flexible config with hot-reload
✅ **Structured Logging** - High-performance logging with rotation
✅ **Dependency Injection** - Clean architecture with Uber FX

## Quick Start

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd nekobot

# Build
go build -o nekobot ./cmd/nekobot

# Run
./nekobot agent -m "Hello! List files in the current directory."
```

### Configuration

Create a config file at `~/.nekobot/config.json`:

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.nekobot/workspace",
      "restrict_to_workspace": true,
      "provider": "claude",
      "model": "claude-sonnet-4-5-20250929",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "providers": {
    "anthropic": {
      "api_key": "your-api-key-here"
    },
    "openai": {
      "api_key": "your-openai-key"
    }
  }
}
```

Or use environment variables:

```bash
export NEKOBOT_PROVIDERS_ANTHROPIC_API_KEY="your-key"
export NEKOBOT_AGENTS_DEFAULTS_MODEL="claude-sonnet-4-5-20250929"
```

## Usage

### One-Shot Mode

```bash
# Simple query
nekobot agent -m "What is 2+2?"

# File operation
nekobot agent -m "Create a file called hello.txt with 'Hello World'"

# Command execution
nekobot agent -m "Run 'ls -la' and show me the results"
```

### Interactive Mode (Coming Soon)

```bash
nekobot agent
```

## Architecture

```
nekobot/
├── cmd/
│   └── nekobot/          # CLI entry point
├── pkg/
│   ├── agent/            # Agent core
│   │   ├── agent.go      # Main agent loop
│   │   ├── context.go    # Context builder
│   │   ├── memory.go     # Memory management
│   │   └── fx.go         # DI module
│   ├── providers/        # LLM providers
│   │   ├── types.go      # Unified types
│   │   ├── registry.go   # Provider registry
│   │   ├── client.go     # High-level client
│   │   ├── converter/    # Format converters
│   │   ├── streaming/    # Stream processors
│   │   └── adaptor/      # Provider implementations
│   ├── tools/            # Tool system
│   │   ├── registry.go   # Tool registry
│   │   ├── file.go       # File tools
│   │   ├── exec.go       # Shell execution
│   │   └── common.go     # Common tools
│   ├── config/           # Configuration
│   ├── logger/           # Logging
│   └── session/          # Session management
└── docs/                 # Documentation
```

## Available Tools

- **read_file**: Read file contents
- **write_file**: Write content to files
- **list_dir**: List directory contents
- **exec**: Execute shell commands
- **message**: Send messages to user

## Development

### Adding a New Tool

```go
package tools

type MyTool struct {}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Description of what my tool does"
}

func (t *MyTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{
                "type": "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"param1"},
    }
}

func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // Tool implementation
    return "result", nil
}
```

Then register it:

```go
agent.GetTools().MustRegister(&MyTool{})
```

### Adding a New Provider

Implement the `providers.Adaptor` interface and register it:

```go
func init() {
    providers.Register("myprovider", func() providers.Adaptor {
        return &MyAdaptor{}
    })
}
```

## Progress

- ✅ Phase 1: Provider Architecture (13 files, ~2.7k lines)
- ✅ Phase 2: Configuration Management (5 files, ~1k lines)
- ✅ Phase 3: Logging + DI (6 files, ~800 lines)
- ✅ Phase 4: Agent Core + Tools (8 files, ~1.2k lines)
- ✅ Phase 5: Session + CLI (3 files, ~400 lines)
- 🚧 Phase 6: Advanced Features (channels, heartbeat, cron)

**Total**: 35+ files, ~6,100 lines of code

## Documentation

- [Provider Architecture](docs/PROVIDERS.md)
- [Logging System](docs/LOGGING.md)
- [Implementation Progress](docs/PROGRESS.md)

## License

MIT

## Credits

Inspired by picoclaw and nanobot projects.

Built with ❤️ using Go, zap, fx, viper, and cobra.
