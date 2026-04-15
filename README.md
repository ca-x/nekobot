# nekobot 🤖

An AI assistant platform built with Go, centered around a Web-first operating model:
start from CLI, then complete provider, channel, agent, memory, and tool configuration in the dashboard.

## Features

✅ **Web-led setup and runtime management** - first-run WebUI now creates the admin account and can save safe startup settings before you continue with providers / agents / channels / cron / tools

✅ **Unified provider system** - OpenAI, Claude, Gemini, OpenRouter-compatible endpoints, provider pools, routing defaults, and fallback chains

✅ **Multi-channel messaging** - Telegram, Discord, WhatsApp, Feishu, QQ, DingTalk, Slack, WeChat, and more

✅ **Persistent runtime config** - bootstrap file for startup settings plus runtime database for Web-managed configuration

✅ **Skills system** - multi-path loading, requirement gating, remote search / install, snapshots, and runtime inspection

✅ **Memory system** - built-in memory, QMD integration, workspace notes, session export, and configurable persistence

✅ **Workspace automation** - auto-bootstrap workspace files, daily logs, and runtime directories

✅ **Cron and automation** - scheduled tasks with per-task provider / model / fallback routing overrides

✅ **Tool sessions** - browser-accessible long-running tools with access control and process management

✅ **Pluggable runtime transport** - tool sessions can stay on tmux by default or opt into zellij for controlled rollout and web-oriented terminal workflows

✅ **Runtime prompts** - Web-managed prompt templates with global / channel / session bindings and render-time context resolution

✅ **Docker and service deployment** - containerized runtime plus native gateway service mode

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
by default. The first-run Web page now creates the initial admin account and can persist
safe startup settings such as `logger`, `gateway`, and `webui`. Providers / agents / channels /
tools are intended to be configured in WebUI.

### Configuration

Bootstrap config lives in `~/.nekobot/config.json` and mainly keeps startup settings
such as logger, gateway, storage, and WebUI. WebUI can now manage `logger`, `gateway`,
`webui`, and `storage` bootstrap state, but changing storage/database location still
requires an explicit migration/restart workflow. A minimal file looks like this:

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

Tool Sessions now support a runtime transport selection:

- **Default:** `tmux`
- **Optional:** `zellij`

Operators can:

- keep the shipped default (`tmux`)
- choose `zellij` per Tool Session in WebUI
- or set an instance-level default with `webui.tool_session_runtime_transport`

Example:

```json
{
  "webui": {
    "enabled": true,
    "port": 0,
    "tool_session_runtime_transport": "zellij"
  }
}
```

`zellij` is currently an opt-in path intended for staged validation. Existing installs
should continue to use `tmux` unless you explicitly switch.

Or use environment variables:

```bash
export NEKOBOT_GATEWAY_PORT="18790"
export NEKOBOT_LOGGER_LEVEL="debug"
```

See [docs/CONFIG.md](docs/CONFIG.md) for the current configuration model.
See [docs/RUNTIME_PROMPTS.md](docs/RUNTIME_PROMPTS.md) for prompt binding behavior and the regression checklist.

### Docker

```bash
docker compose up -d
```

This persists all runtime state in `./data`:

- `./data/config/config.json`: bootstrap config, auto-created on first boot
- `./data/db/nekobot.db`: runtime database for WebUI-managed settings
- `./data/workspace`: workspace and local runtime files

After startup, open `http://127.0.0.1:18791` and finish configuration in WebUI.

The default Docker image now preinstalls `QMD`. If you do not want QMD in the image,
build with:

```bash
docker build --build-arg INSTALL_QMD=false -t nekobot:no-qmd .
```

For non-preinstalled environments, WebUI can also install QMD into the persistent
workspace runtime directory under `./data/workspace/.nekobot/runtime/qmd`.

If QMD session export is enabled and `memory.qmd.sessions.export_dir` is left empty,
nekobot defaults it to `${WORKSPACE}/memory/sessions`, and the WebUI now shows the
resolved path, export count, retention policy, and a manual cleanup action.

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

### Interactive Mode

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

## Documentation

- [Configuration](docs/CONFIG.md)
- [QMD Integration](docs/QMD_INTEGRATION.md)
- [Provider Architecture](docs/PROVIDERS.md)
- [Logging System](docs/LOGGING.md)
- [Gateway Service Management](docs/GATEWAY_SERVICE.md)
- [Message Bus Architecture](docs/BUS_ARCHITECTURE.md)
- [Web Tools](docs/WEB_TOOLS.md)
- [Implementation Progress](docs/PROGRESS.md)

## License

MIT

## References

This project references and learns from:
- https://github.com/smallnest/goclaw
- https://github.com/sipeed/picoclaw
- /home/czyt/code/go/nextclaw
- /home/czyt/code/go/weclaw
