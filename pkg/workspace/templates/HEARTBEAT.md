---
summary: "Heartbeat tasks for periodic execution"
version: "1.0"
---

# HEARTBEAT.md - Periodic Tasks

This file defines tasks that run periodically when heartbeat is enabled.

## What is Heartbeat?

Heartbeat allows your agent to perform autonomous periodic tasks:
- Check for updates
- Summarize activity
- Maintain memory
- Monitor systems
- Send reminders

## Configuration

Enable heartbeat in your config:

```json
{
  "heartbeat": {
    "enabled": true,
    "interval_minutes": 30
  }
}
```

## Task Prompts

Define your heartbeat tasks below. The agent will execute these every interval.

### Task 1: Activity Summary

```prompt
Review today's activity (check {{.Workspace}}/memory/{{.Date}}.md).
If there are significant events, add a brief summary.
```

### Task 2: Memory Maintenance

```prompt
Check if memory needs organization. If memory/ has more than 30 files,
consider summarizing older entries.
```

### Task 3: Workspace Health

```prompt
Quick workspace health check:
- Are there any error logs?
- Is disk usage reasonable?
- Any stuck processes?

Only report if something needs attention.
```

## Custom Tasks

Add your own tasks here. Each task should be:
- **Autonomous**: Doesn't require user input
- **Quick**: Completes in <30 seconds
- **Safe**: No destructive operations
- **Useful**: Provides value or prevents issues

### Example: Code Health

```prompt
If there's a Git repository in workspace, check:
- Uncommitted changes older than 7 days
- Stale branches
Report findings if any.
```

---

## Tips

- Start with simple, read-only tasks
- Gradually add more as you trust the system
- Heartbeat logs are in memory/heartbeat-state.json
- Disable heartbeat anytime in config

---

_Edit this file to customize your heartbeat tasks._
