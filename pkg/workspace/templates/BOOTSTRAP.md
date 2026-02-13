---
summary: "Workspace overview and bootstrap guide"
version: "1.0"
---

# BOOTSTRAP.md - Getting Started

Welcome to your nekobot workspace!

This directory contains all the files your agent uses to persist memory, configure behavior, and maintain context across sessions.

## Workspace Structure

```
{{.Workspace}}/
├── AGENTS.md         # Agent configuration and metadata
├── SOUL.md           # Agent personality and guidelines
├── IDENTITY.md       # Agent identity and purpose
├── USER.md           # Your preferences and information
├── TOOLS.md          # Available tools documentation
├── HEARTBEAT.md      # Periodic task definitions
├── BOOT.md           # Startup instructions
├── BOOTSTRAP.md      # This file
├── memory/           # Long-term memory storage
│   ├── YYYY-MM-DD.md # Daily logs (auto-created)
│   └── heartbeat-state.json
├── skills/           # Custom skills directory
├── sessions/         # Conversation session storage
└── .nekobot/         # Hidden metadata
    ├── skills/       # Global skills
    └── snapshots/    # Skill snapshots
```

## Key Files

### Core Identity
- **SOUL.md** - Defines your agent's personality and core principles
- **IDENTITY.md** - Basic agent information and purpose
- **USER.md** - Information about you and your preferences

### Configuration
- **AGENTS.md** - Agent metadata and configuration
- **TOOLS.md** - Available tools and their usage
- **BOOT.md** - Startup tasks and initialization
- **HEARTBEAT.md** - Periodic autonomous tasks

### Memory
- **memory/** - Stores daily logs and long-term memory
- **sessions/** - Conversation history in JSONL format

## First Steps

1. **Personalize Core Files**
   - Edit `SOUL.md` to define agent personality
   - Fill in `IDENTITY.md` with agent name and vibe
   - Update `USER.md` with your preferences

2. **Configure Tools**
   - Review `TOOLS.md` for available capabilities
   - Add API keys to `~/.nekobot/config.json` as needed

3. **Add Skills** (Optional)
   - Place skill `.md` files in `skills/` directory
   - Skills auto-load on agent startup

4. **Enable Heartbeat** (Optional)
   - Edit `HEARTBEAT.md` to define periodic tasks
   - Enable in config: `{"heartbeat": {"enabled": true}}`

## Usage

### Agent Mode (Interactive)
```bash
nekobot agent
```

### Agent Mode (One-shot)
```bash
nekobot agent -m "your message here"
```

### Gateway Mode (Multi-channel)
```bash
nekobot gateway
```

## Configuration

Primary config: `~/.nekobot/config.json`

Override with workspace config: `{{.Workspace}}/config.json`

See `config.example.json` for available options.

## Memory System

Your agent maintains memory through:
1. **Daily Logs**: Auto-created in `memory/YYYY-MM-DD.md`
2. **Session History**: Stored in `sessions/*.jsonl`
3. **Long-term Memory**: Semantic search across all memory files

Memory is automatically indexed for quick retrieval.

## Skills

Add skills by:
1. Creating `.md` files in `skills/` with YAML frontmatter
2. Installing from GitHub (coming soon)
3. Using pre-built skills from community

Skills extend agent capabilities for specific domains (git, docker, etc.).

## Support

- **Documentation**: See `docs/` in project repository
- **Issues**: Report at GitHub issues
- **Configuration**: Check `config.example.json` for all options

---

**Initialized:** {{.Date}}
**Version:** {{.Version}}

_This workspace is yours. Customize it to fit your workflow._
