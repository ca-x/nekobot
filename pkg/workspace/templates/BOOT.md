---
summary: "Boot initialization instructions"
version: "1.0"
---

# BOOT.md - Startup Instructions

This file contains instructions that run when your agent starts up.

## Initialization

On startup, your agent will:

1. ✅ Load configuration from `~/.nekobot/config.json`
2. ✅ Initialize workspace at `{{.Workspace}}`
3. ✅ Load skills from `{{.Workspace}}/skills/`
4. ✅ Connect to configured channels (if gateway mode)
5. ✅ Start heartbeat (if enabled)

## Startup Tasks

Define any startup tasks below. These run once per agent boot.

### Check Today's Log

```prompt
Check if today's log exists at memory/{{.Date}}.md.
If not, it will be auto-created.
```

### Load Context

```prompt
Review recent memory entries to understand current context.
Check the last 3 days: memory/{{.Date}}.md and previous days.
```

### Status Check

```prompt
Verify all systems:
- Skills loaded correctly?
- Memory accessible?
- Tools available?

Report any issues.
```

## Custom Boot Tasks

Add your own initialization tasks:

### Example: Project Status

```prompt
If there's a Git repository:
- Check current branch
- Count uncommitted changes
- List recent commits (last 3)

Provide a brief status summary.
```

---

## Boot Logs

Boot logs are written to:
- Console (if running in agent mode)
- System logs (if running as service)

Check logs if boot seems slow or fails.

---

_Edit this file to customize your boot sequence._
