---
summary: "Available tools documentation"
version: "1.0"
---

# TOOLS.md - Available Tools

Your agent has access to these tools for completing tasks.

## File Operations

- **read_file** - Read contents of a file
- **write_file** - Create or overwrite a file
- **edit_file** - Edit specific parts of a file by string replacement
- **append_file** - Append content to an existing file
- **list_dir** - List directory contents

## Execution

- **exec** - Execute shell commands
  - ⚠️ Use carefully - commands run with your permissions
  - Restricted to workspace by default (configurable)

## Web & Research

- **web_search** - Search the web using Brave API
  - Requires API key in config
- **web_fetch** - Fetch content from URLs
- **browser** - Control Chrome browser via CDP
  - Navigate, screenshot, interact with pages
  - Requires Chrome with remote debugging

## Communication

- **message** - Send messages directly to you
  - Use when agent needs immediate attention
  - Bypasses normal conversation flow

## Memory & Search

- **memory** - Search and manage long-term memory
  - Semantic search across workspace files
  - Add important information to memory

## Skills

- **skill** - Access specialized skills
  - List available skills
  - Invoke skill-specific workflows

## Async Execution

- **spawn** - Create background subagent tasks
  - Long-running operations
  - Parallel task execution

---

## Tool Configuration

Tools can be enabled/disabled in your config file:

```json
{
  "tools": {
    "web": {
      "search": {
        "enabled": true,
        "api_key": "YOUR_BRAVE_API_KEY"
      }
    }
  }
}
```

## Safety

- File operations are restricted to workspace by default
- Shell commands require user approval for dangerous operations
- Web tools respect rate limits

---

_This file is auto-generated. Tool availability depends on your configuration._
