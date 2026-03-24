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

✅ **Message Bus** - Multi-channel message routing

✅ **State Management** - File or Redis backend for KV storage

✅ **Heartbeat System** - Periodic autonomous tasks

✅ **Cron Jobs** - Scheduled task execution

✅ **System Service** - Run gateway as a native system service

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

### Web-First Startup

```bash
# Start gateway + web dashboard
./nekobot gateway
```

On first start, nekobot will auto-create bootstrap config at `~/.nekobot/config.json`,
initialize `~/.nekobot/nekobot.db`, and expose the WebUI on `http://127.0.0.1:18791`
by default. Providers / agents / channels / tools are intended to be configured in WebUI.

### Configuration

Bootstrap config lives in `~/.nekobot/config.json` and mainly keeps startup settings
such as logger, gateway, storage, and WebUI. A minimal file looks like this:

```json
{
  "logger": {
    "level": "info"
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  },
  "webui": {
    "enabled": true,
    "port": 0,
    "username": "admin",
    "password": ""
  }
}
```

Most runtime settings, including providers / agents / tools / channels, should be
edited in WebUI and are persisted in the runtime database `nekobot.db`.

Or use environment variables:

```bash
export NEKOBOT_GATEWAY_PORT="18790"
export NEKOBOT_LOGGER_LEVEL="debug"
```

See [docs/CONFIG.md](docs/CONFIG.md) for the current configuration model.

### Docker

```bash
docker compose up -d
```

This persists all runtime state in `./data`:

- `./data/config/config.json`: bootstrap config, auto-created on first boot
- `./data/db/nekobot.db`: runtime database for WebUI-managed settings
- `./data/workspace`: workspace and local runtime files

After startup, open `http://127.0.0.1:18791` and finish configuration in WebUI.

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

### Gateway Mode

```bash
# Run gateway in foreground
nekobot gateway

# Install as system service
sudo nekobot gateway install

# Manage service
sudo nekobot gateway start
sudo nekobot gateway stop
sudo nekobot gateway restart
nekobot gateway status

# Uninstall service
sudo nekobot gateway uninstall
```

See [Gateway Service Documentation](docs/GATEWAY_SERVICE.md) for more details.

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
│   ├── bus/              # Message bus
│   ├── state/            # State management (file/redis)
│   ├── heartbeat/        # Heartbeat system
│   ├── cron/             # Cron jobs
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
- **web_search**: Search the web using Brave Search (with DuckDuckGo fallback)
- **web_fetch**: Fetch and extract content from URLs
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
- ✅ Phase 6: Advanced Features (15 files, ~3k lines)
  - Message Bus (local + Redis implementations)
  - State Management (file + Redis backends)
  - Heartbeat system for autonomous tasks
  - Cron job scheduling
  - System service management
  - Channel system framework
  - Web tools (search + fetch)
- 🚧 Phase 7: Channel Implementations
  - ✅ Telegram (basic implementation)
  - ⏳ Discord, WhatsApp, Feishu, etc.

**Total**: 50+ files, ~10,000 lines of code

## Documentation

- [Provider Architecture](docs/PROVIDERS.md)
- [Logging System](docs/LOGGING.md)
- [Gateway Service Management](docs/GATEWAY_SERVICE.md)
- [Message Bus Architecture](docs/BUS_ARCHITECTURE.md)
- [Web Tools](docs/WEB_TOOLS.md)
- [Implementation Progress](docs/PROGRESS.md)

## License

MIT

## Credits

Inspired by picoclaw and nanobot projects.

Built with ❤️ using Go, zap, fx, viper, and cobra.

## References

This project references and learns from:
- https://github.com/smallnest/goclaw
- https://github.com/sipeed/picoclaw
