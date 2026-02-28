# Nekobot Architecture Overview

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Nekobot CLI                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Commands: agent, gateway, version                        │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────────┘
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Agent Module │  │Gateway Module│  │ Config Module│
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                  │
       └─────────────────┴──────────────────┘
                         │
        ┌────────────────┼───────────────────────┐
        ▼                ▼                       ▼
┌──────────────┐  ┌──────────────┐      ┌──────────────┐
│  Providers   │  │    Tools     │      │  Skills      │
│  (LLM APIs)  │  │   Registry   │      │   Manager    │
└──────────────┘  └──────┬───────┘      └──────┬───────┘
                         │                     │
        ┌────────────────┴────────┐            │
        ▼                         ▼            ▼
┌──────────────┐          ┌──────────────┐  ┌──────────────┐
│   Memory     │          │   Browser    │  │  Validator   │
│   System     │          │     CDP      │  │ Eligibility  │
└──────────────┘          └──────────────┘  └──────────────┘
```

## Core Components

### 1. Provider Layer (pkg/providers/)
**Purpose**: Unified interface to multiple LLM providers

**Components**:
- `adaptor.go` - Core provider interface
- `registry.go` - Provider registry
- `client.go` - High-level client
- `converter/` - Format converters (OpenAI ↔ Claude ↔ Gemini)
- `streaming/` - Stream processors (SSE, JSON-lines)
- `adaptors/` - Provider implementations (OpenAI, Claude, Gemini, Generic)

**Supported Providers**: OpenAI, Claude (Anthropic), Gemini (Google), OpenRouter, vLLM

### 2. Agent Core (pkg/agent/)
**Purpose**: Main AI agent with tool orchestration

**Components**:
- `agent.go` - Agent loop and chat handling
- `context.go` - System prompt building (identity, tools, skills, memory)
- `memory.go` - File-based memory (MEMORY.md, daily notes)
- `fx.go` - Dependency injection

**Features**:
- Tool calling with iteration control
- Context building from multiple sources
- Session history management
- Skills integration

### 3. Tools System (pkg/tools/)
**Purpose**: Extensible tool system for agent capabilities

**Built-in Tools**:
- **file.go** - Read, write, list, edit, append files
- **exec.go** - Shell command execution
- **web_search.go** - Brave Search + DuckDuckGo fallback integration
- **web_fetch.go** - URL content fetching with HTML parsing
- **browser.go** - Chrome CDP automation (navigate, screenshot, click, type, execute JS)
- **message.go** - Direct user communication via bus

**Tool Registry**:
- `registry.go` - Tool registration and discovery
- `common.go` - Common tool utilities

### 4. Skills System (pkg/skills/)
**Purpose**: Pluggable capabilities loaded from markdown files

**Components**:
- `manager.go` - Skill discovery and loading
- `watcher.go` - File watching with hot-reload (fsnotify)
- `validator.go` - Skill validation and diagnostics
- `eligibility.go` - System requirement checking (OS, binaries, env vars)
- `installer.go` - Dependency installation (brew, go, npm, pip/uv, download)
- `types.go` - Enhanced types (SkillEntry, InstallSpec, Diagnostic)

**Features**:
- YAML frontmatter parsing
- Hot-reload with file watching
- Requirement validation
- Auto-installation of dependencies
- Enable/disable individual skills

### 5. Memory System (pkg/memory/)
**Purpose**: Vector-based semantic memory for agent

**Components**:
- `types.go` - Memory types, embeddings, search options
- `store.go` - FileStore implementation with vector search

**Features**:
- Vector embeddings for semantic similarity
- Hybrid search (vector + keyword)
- Multiple sources (longterm, session, daily)
- Multiple types (fact, preference, context, conversation)
- Cosine similarity search
- Access tracking and importance scoring

**Storage**: JSON-based file store with atomic writes

### 6. State Management (pkg/state/)
**Purpose**: KV store for application state

**Components**:
- `kv.go` - KV interface
- `state.go` - FileStore implementation
- `redis.go` - RedisStore implementation
- `factory.go` - Backend selection

**Use Cases**: Feature flags, heartbeat state, cron status, runtime config

### 7. Message Bus (pkg/bus/)
**Purpose**: Message routing between channels and agent

**Components**:
- `interface.go` - Bus interface
- `local_bus.go` - Go channels implementation
- `redis_bus.go` - Redis pub/sub implementation
- `factory.go` - Backend selection

**Features**:
- Inbound/outbound message routing
- Handler registration per channel
- Metrics tracking
- Scalable with Redis backend

### 8. Channels (pkg/channels/)
**Purpose**: Multi-platform communication

**Components**:
- `channel.go` - Channel interface
- `manager.go` - Channel lifecycle management
- `telegram/` - Telegram bot implementation

**Planned**: Discord, WhatsApp, Feishu, QQ, DingTalk, Slack

### 9. Heartbeat System (pkg/heartbeat/)
**Purpose**: Periodic autonomous tasks

**Features**:
- Reads tasks from HEARTBEAT.md
- Configurable interval
- Enable/disable with state persistence
- Independent execution cycle

### 10. Cron System (pkg/cron/)
**Purpose**: Scheduled task execution

**Features**:
- Cron expression support (robfig/cron)
- One-time (`at`) and fixed-interval (`every`) schedules
- Runtime database persistence (`cron_jobs` table)
- Individual job enable/disable
- Job history tracking
- Run-now trigger support via CLI/WebUI

### 11. Session Management (pkg/session/)
**Purpose**: Conversation history persistence

**Features**:
- File-based session storage
- Message history
- Tool call tracking
- Metadata support

**Planned**: JSONL format, pruning strategies (LRU, LFU, TTL, Size)

### 12. Configuration (pkg/config/)
**Purpose**: Flexible configuration management

**Components**:
- `config.go` - Config structures
- `loader.go` - Viper-based loader
- `watcher.go` - Hot-reload support
- `validator.go` - Config validation

**Sources**: JSON/YAML files, environment variables (NEKOBOT_ prefix)

### 13. Logging (pkg/logger/)
**Purpose**: Structured logging

**Implementation**: zap + lumberjack (rotation)

**Features**:
- Terminal and file output
- Log rotation
- Structured fields
- Multiple log levels

## Data Flow

### Agent Chat Flow
```
User Message
    ↓
Agent.Chat()
    ↓
Context Building (system prompt + history + skills + memory)
    ↓
Provider API Call (OpenAI/Claude/Gemini)
    ↓
Tool Calls? → Yes → Execute Tools → Loop
    ↓ No
Final Response
```

### Gateway Message Flow
```
Channel (Telegram/Discord/etc.)
    ↓
Bus.SendInbound()
    ↓
Agent Processes
    ↓
Bus.SendOutbound()
    ↓
Channel Sends to User
```

### Skills Loading Flow
```
Discover *.md files in skills/
    ↓
Parse YAML frontmatter
    ↓
Validate skill definition
    ↓
Check eligibility (OS, binaries, env)
    ↓
Install dependencies if needed
    ↓
Register skill
    ↓
Watch for file changes (hot-reload)
```

## Storage Structure

```
~/.nekobot/
├── config.json              # Main configuration
├── workspace/               # Agent workspace
│   ├── AGENTS.md           # Agent capabilities
│   ├── SOUL.md             # Personality
│   ├── USER.md             # User info
│   ├── IDENTITY.md         # Custom identity
│   ├── HEARTBEAT.md        # Heartbeat tasks
│   ├── memory/             # Long-term memory
│   │   ├── MEMORY.md       # Persistent facts
│   │   └── YYYYMM/         # Daily notes
│   │       └── YYYYMMDD.md
│   ├── sessions/           # Conversation sessions
│   │   └── *.json
│   ├── state/              # Application state
│   │   └── *.json
│   └── screenshots/        # Browser screenshots
├── skills/                  # Skill definitions
│   └── *.md
├── logs/                    # Application logs
│   └── nekobot.log
└── memory/                  # Vector memory store
    └── embeddings.json
```

## Key Design Patterns

### 1. Interface-Based Architecture
All major components define interfaces for flexibility:
- Providers (Adaptor interface)
- State (KV interface)
- Bus (Bus interface)
- Memory (Store interface)
- Tools (Tool interface)
- Skills (SkillLoader interface)

### 2. Factory Pattern
Component creation through factories:
- Provider factory (by name)
- State factory (file vs Redis)
- Bus factory (local vs Redis)

### 3. Registry Pattern
Centralized registration:
- Provider registry
- Tool registry
- Channel registry

### 4. Dependency Injection
Using Uber FX for clean DI:
- Module-based organization
- Lifecycle hooks
- Clean shutdown

### 5. Event-Driven
Async communication:
- Message bus for channels
- Watcher events for skills
- Tool execution events

## Extensibility Points

### Adding a New Provider
1. Implement `Adaptor` interface
2. Create format converter
3. Register in provider registry
4. Add config section

### Adding a New Tool
1. Implement `Tool` interface (Name, Description, Parameters, Execute)
2. Register with tool registry
3. Agent automatically includes in prompts

### Adding a New Channel
1. Implement `Channel` interface
2. Register with channel manager
3. Configure in config file
4. Auto-starts when enabled

### Adding a New Skill
1. Create .md file with YAML frontmatter
2. Drop in skills/ directory
3. Auto-discovered and hot-reloaded
4. Specify requirements for auto-installation

## Performance Characteristics

- **Memory Usage**: <10MB idle (target)
- **Startup Time**: <100ms (target)
- **Request Latency**: <500ms (excluding LLM API time)
- **Concurrent Sessions**: 10+ without issues
- **Tool Execution**: Parallel where possible

## References

- Architecture inspired by: goclaw, picoclaw
- Provider pattern from: newapi
- Skills system from: goclaw
- Memory system from: goclaw (QMD)
- Browser automation from: goclaw
