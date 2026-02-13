# Nekobot Implementation Plan - Remaining Features from Goclaw

This document tracks remaining features to implement from goclaw that haven't been implemented in nekobot yet.

## Priority 1: API故障转移和轮换 (API Failover & Rotation)

### Overview
Implement intelligent API failover and rotation system inspired by goclaw's `providers/rotation.go`.

### Features
1. **Multiple API Key Profiles**
   - Support multiple API keys per provider
   - Each profile has: name, API key, priority, cooldown state, request count
   - Profile management: add, remove, get status

2. **Rotation Strategies**
   - `RoundRobin`: Cycle through available profiles
   - `LeastUsed`: Select profile with lowest request count
   - `Random`: Random selection from available profiles

3. **Intelligent Failover**
   - Automatic error classification (auth, rate limit, billing, network, server)
   - Cooldown mechanism for problematic profiles
   - Configurable cooldown duration
   - Only set cooldown for retriable errors (auth, rate limit, billing)

4. **Status Monitoring**
   - Track request count per profile
   - Cooldown status and expiry time
   - Profile availability check

### Implementation Files
- `pkg/providers/rotation.go` - Rotation provider implementation
- `pkg/providers/failover.go` - Error classification and failover logic
- `pkg/providers/types.go` - Update with FailoverReason enum
- `pkg/config/schema.go` - Add rotation config (strategy, cooldown, profiles)

### Configuration Schema
```json
{
  "providers": {
    "claude": {
      "rotation": {
        "enabled": true,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "primary": {
          "api_key": "sk-xxx",
          "priority": 1
        },
        "secondary": {
          "api_key": "sk-yyy",
          "priority": 2
        }
      }
    }
  }
}
```

---

## Priority 2: Skill高级管理 (Advanced Skills Management)

### Overview
Enhance skills system with advanced features from goclaw.

### Features Already Implemented ✅
- Basic skill discovery from .md files
- YAML frontmatter parsing
- Enable/disable functionality
- Requirements checking (eligibility)
- Hot-reload with file watching

### Features To Implement

#### 1. Multi-Path Skill Loading with Priority
Currently: Single `skills_dir`
Goal: Multiple directories with priority override

**Load Order** (later overrides earlier):
1. Embedded built-in skills
2. `${EXECUTABLE_DIR}/skills/`
3. `~/.nekobot/skills/` (global)
4. `${WORKSPACE}/.nekobot/skills/` (workspace hidden)
5. `${WORKSPACE}/skills/` (workspace)
6. `./skills/` (current directory - highest priority)

**Implementation**:
- `pkg/skills/loader.go` - Add multi-path loading
- `pkg/skills/priority.go` - Skill priority resolution
- Skill override: same ID from higher priority path replaces lower

#### 2. Skill Snapshots
Create immutable snapshots of skill state for:
- Debugging
- Auditing
- Rollback

**Features**:
- Snapshot current enabled skills + versions
- Store skill content hash
- Restore from snapshot
- List available snapshots

**Implementation**:
- `pkg/skills/snapshot.go` - Snapshot create/restore
- Snapshots stored in `${WORKSPACE}/.nekobot/snapshots/`

#### 3. Skill Versioning
Track skill changes over time:
- Git-like versioning (optional)
- Change detection via content hash
- Version history

**Implementation**:
- `pkg/skills/version.go` - Versioning logic
- Store version metadata in skill struct

#### 4. Advanced Gating (Eligibility)
Already implemented basic gating. Enhance with:
- **OS-level gating**: `requires.os: [darwin, linux]`
- **Architecture gating**: `requires.arch: [arm64, amd64]`
- **Network gating**: Check if URL accessible
- **Custom validators**: User-defined gating functions

**Implementation**:
- `pkg/skills/eligibility.go` - Add OS/arch/network checks
- `pkg/skills/validator.go` - Custom validator registration

---

## Priority 3: QMD管理 (QMD Integration)

### Overview
Integrate QMD (external markdown indexing tool) for advanced semantic search in workspace files.

### Background
QMD is an external CLI tool (like `git`) that:
- Indexes markdown files using vector embeddings
- Provides semantic search
- Manages document collections
- Handles incremental updates

### Features

#### 1. QMD Manager
Manage QMD process lifecycle and collections.

**Capabilities**:
- Check QMD availability (`qmd --version`)
- Initialize collections (default, sessions, custom paths)
- Incremental updates
- Fallback to built-in search if QMD unavailable

**Implementation**:
- `pkg/memory/qmd/manager.go` - Main QMD manager
- `pkg/memory/qmd/process.go` - Process execution helpers
- `pkg/memory/qmd/types.go` - QMD types and configs

#### 2. Collection Management
QMD organizes documents into collections.

**Default Collections**:
- `default`: `${WORKSPACE}/memory/**/*.md`
- `sessions`: Exported session files
- `custom`: User-defined paths

**Operations**:
- Create collection: `qmd create <name> <path> <pattern>`
- Update collection: `qmd update <name>`
- List collections: `qmd list`
- Search collection: `qmd search <name> <query>`
- Delete collection: `qmd delete <name>`

**Implementation**:
- `pkg/memory/qmd/collection.go` - Collection CRUD
- `pkg/memory/qmd/search.go` - Search interface

#### 3. Session Export for QMD
Export JSONL sessions to markdown for QMD indexing.

**Process**:
1. Read session `.jsonl` files
2. Convert to markdown format
3. Export to `${WORKSPACE}/memory/sessions/`
4. Index with QMD

**Implementation**:
- `pkg/memory/qmd/sessions.go` - Session export logic
- `pkg/session/export.go` - Export to markdown

#### 4. Automatic Updates
Keep QMD collections up-to-date.

**Strategies**:
- On boot: Update all collections on agent start
- Scheduled: Update every N minutes
- On change: Update when files modified (via fsnotify)

**Configuration**:
```json
{
  "memory": {
    "qmd": {
      "enabled": true,
      "command": "qmd",
      "include_default": true,
      "paths": [
        {"name": "docs", "path": "~/Documents", "pattern": "**/*.md"}
      ],
      "sessions": {
        "enabled": true,
        "export_dir": "${WORKSPACE}/memory/sessions",
        "retention_days": 90
      },
      "update": {
        "on_boot": true,
        "interval": "30m",
        "command_timeout": "30s",
        "update_timeout": "5m"
      }
    }
  }
}
```

**Implementation**:
- `pkg/memory/qmd/updater.go` - Update scheduler
- `pkg/memory/manager.go` - Integrate with memory manager

---

## Priority 4: Workspace模版化 (Workspace Templating)

### Overview
Auto-generate workspace structure with template files from goclaw.

### Template Files
Embedded templates in `pkg/workspace/templates/*.md`:

1. **AGENTS.md** - Agent configuration and metadata
2. **SOUL.md** - Agent personality and behavior guidelines
3. **IDENTITY.md** - Agent identity and purpose
4. **USER.md** - User information and preferences
5. **TOOLS.md** - Available tools documentation
6. **HEARTBEAT.md** - Heartbeat task definitions
7. **BOOT.md** - Boot initialization instructions
8. **BOOTSTRAP.md** - Bootstrap overview

### Features

#### 1. Workspace Initialization
Auto-create workspace structure on first run.

**Structure**:
```
~/.nekobot/workspace/
├── AGENTS.md          # From template
├── SOUL.md            # From template
├── IDENTITY.md        # From template
├── USER.md            # From template
├── TOOLS.md           # From template
├── HEARTBEAT.md       # From template
├── BOOT.md            # From template
├── BOOTSTRAP.md       # From template
├── memory/            # Memory directory
│   ├── YYYY-MM-DD.md  # Daily logs (auto-created)
│   └── heartbeat-state.json
├── skills/            # User skills
├── sessions/          # Session storage
└── .nekobot/          # Hidden metadata
    ├── skills/        # Global skills
    └── snapshots/     # Skill snapshots
```

**Implementation**:
- `pkg/workspace/manager.go` - Workspace manager
- `pkg/workspace/templates/` - Embedded template files (use `//go:embed`)
- `pkg/workspace/init.go` - Initialization logic

#### 2. Daily Log Auto-Creation
Automatically create `YYYY-MM-DD.md` in `memory/` directory.

**Features**:
- Create today's log on boot if missing
- Use template with placeholders
- Include date, day of week, agent info

**Template Example**:
```markdown
# {{.Date}} ({{.DayOfWeek}})

## Morning Notes
[Agent: {{.AgentID}}]

## Events

## Reflections

---
*Generated by nekobot v{{.Version}}*
```

**Implementation**:
- `pkg/workspace/daily.go` - Daily log creation
- Template stored in `pkg/workspace/templates/DAILY.md`

#### 3. Template Variables
Support variable substitution in templates.

**Variables**:
- `{{.Date}}` - Current date (2006-01-02)
- `{{.DayOfWeek}}` - Day name (Monday, Tuesday, ...)
- `{{.AgentID}}` - Agent identifier
- `{{.Version}}` - Nekobot version
- `{{.Workspace}}` - Workspace path
- `{{.User}}` - System username

**Implementation**:
- Use Go `text/template` package
- `pkg/workspace/template.go` - Template rendering

#### 4. Workspace Onboarding
Interactive CLI wizard for first-time setup.

**Process**:
```bash
$ nekobot onboard
Welcome to nekobot! Let's set up your workspace.

Workspace location: [~/.nekobot/workspace] ▊
Agent name: [nekobot] ▊
Personality (soul): [helpful assistant] ▊

✓ Created workspace at ~/.nekobot/workspace
✓ Generated configuration files
✓ Initialized memory directory
✓ Set up skills directory

Ready to go! Try: nekobot agent -m "hello"
```

**Implementation**:
- `cmd/nekobot/onboard.go` - Onboard command
- Interactive prompts using `github.com/manifoldco/promptui`

---

## Implementation Order

### Phase 1: Workspace Templating (Easiest)
1. Create embedded templates
2. Implement workspace manager
3. Add daily log auto-creation
4. Implement onboard command

**Estimated effort**: 2-3 days

### Phase 2: Advanced Skills Management (Medium)
1. Multi-path skill loading
2. Skill snapshots
3. Advanced gating (OS/arch)
4. Skill versioning

**Estimated effort**: 3-4 days

### Phase 3: API Failover & Rotation (Medium-Hard)
1. Error classification
2. Rotation provider with strategies
3. Profile management
4. Status monitoring

**Estimated effort**: 4-5 days

### Phase 4: QMD Integration (Hardest)
1. QMD process management
2. Collection CRUD
3. Session export
4. Automatic updates
5. Integration with memory system

**Estimated effort**: 5-7 days

---

## Configuration Changes

### New Config Fields

```json
{
  "workspace": {
    "path": "~/.nekobot/workspace",
    "auto_init": true,
    "daily_log": {
      "enabled": true,
      "template": "DAILY.md"
    }
  },
  "skills": {
    "paths": [
      "${EXECUTABLE_DIR}/skills",
      "~/.nekobot/skills",
      "${WORKSPACE}/.nekobot/skills",
      "${WORKSPACE}/skills",
      "./skills"
    ],
    "auto_reload": true,
    "snapshots": {
      "enabled": true,
      "dir": "${WORKSPACE}/.nekobot/snapshots"
    }
  },
  "providers": {
    "claude": {
      "rotation": {
        "enabled": false,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "primary": {
          "api_key": "${ANTHROPIC_API_KEY}",
          "priority": 1
        }
      }
    }
  },
  "memory": {
    "qmd": {
      "enabled": false,
      "command": "qmd",
      "include_default": true,
      "sessions": {
        "enabled": true,
        "export_dir": "${WORKSPACE}/memory/sessions",
        "retention_days": 90
      },
      "update": {
        "on_boot": true,
        "interval": "30m"
      }
    }
  }
}
```

---

## Testing Strategy

### Phase 1: Workspace
- Test workspace initialization
- Test template rendering
- Test daily log creation
- Test onboard wizard

### Phase 2: Skills
- Test multi-path loading priority
- Test snapshot create/restore
- Test OS/arch gating
- Test hot-reload

### Phase 3: Failover
- Test rotation strategies
- Test error classification
- Test cooldown mechanism
- Test profile failover

### Phase 4: QMD
- Mock QMD for testing
- Test collection lifecycle
- Test session export
- Test search integration

---

## Dependencies

### New External Dependencies
```go
require (
    github.com/manifoldco/promptui v0.9.0  // Interactive CLI prompts
)
```

### Optional External Tool
- **QMD**: External CLI tool (like `git`)
  - If not installed, fallback to built-in search
  - Installation: User responsibility
  - Detection: Check `qmd --version`

---

## Success Criteria

### Workspace Templating
- [ ] Workspace auto-initializes with all template files
- [ ] Daily logs auto-create on boot
- [ ] Onboard wizard completes successfully
- [ ] Template variables render correctly

### Advanced Skills
- [ ] Skills load from multiple paths with correct priority
- [ ] Skill snapshots can be created and restored
- [ ] OS/arch gating works correctly
- [ ] Skill changes detected and hot-reloaded

### API Failover
- [ ] Rotation strategies select profiles correctly
- [ ] Errors trigger appropriate cooldowns
- [ ] Failed profiles excluded during cooldown
- [ ] Profile status accurately reported

### QMD Integration
- [ ] QMD availability detected correctly
- [ ] Collections created and updated
- [ ] Sessions exported to markdown
- [ ] Semantic search returns relevant results
- [ ] Graceful fallback when QMD unavailable

---

## Documentation

### User-Facing Docs
- `docs/WORKSPACE.md` - Workspace setup and structure
- `docs/SKILLS_ADVANCED.md` - Advanced skills features
- `docs/FAILOVER.md` - API failover configuration
- `docs/QMD.md` - QMD integration guide

### Developer Docs
- Update `docs/ARCHITECTURE.md` with new subsystems
- Document rotation provider interface
- Document QMD integration points
- Document workspace template system

---

## Next Steps

1. **Review this plan** - Confirm scope and priorities
2. **Start Phase 1** - Implement workspace templating (easiest win)
3. **Iterate** - Build and test each phase
4. **Document** - Write docs alongside code
5. **Test thoroughly** - Each feature well-tested before moving on

---

## References

- Goclaw rotation: `/Users/czyt/code/go/goclaw/providers/rotation.go`
- Goclaw workspace: `/Users/czyt/code/go/goclaw/internal/workspace/`
- Goclaw QMD: `/Users/czyt/code/go/goclaw/memory/qmd/`
- Goclaw skills: `/Users/czyt/code/go/goclaw/skills/`
