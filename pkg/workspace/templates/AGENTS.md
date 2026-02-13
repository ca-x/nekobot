---
summary: "Agent list and management"
version: "1.0"
---

# AGENTS.md - Your Agents

This file tracks all agents configured in your nekobot system.

## Current Agent

- **Name:** {{.AgentName}}
- **Model:** {{.Model}}
- **Provider:** {{.Provider}}
- **Workspace:** {{.Workspace}}
- **Created:** {{.Date}}

## Configuration

Your agent uses the configuration at:
- Primary: `~/.nekobot/config.json`
- Workspace: `{{.Workspace}}/config.json` (if exists)

## Skills

Skills directory: `{{.Workspace}}/skills/`

To add skills, place `.md` files in the skills directory or install from GitHub.

## Sessions

Active sessions are stored in: `{{.Workspace}}/sessions/`

## Memory

Long-term memory is stored in: `{{.Workspace}}/memory/`

---

_This file is auto-generated but can be edited. Changes persist._
