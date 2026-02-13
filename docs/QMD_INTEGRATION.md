# QMD Integration

Nekobot integrates with QMD (Query Markdown) for advanced semantic search capabilities across your workspace files.

## What is QMD?

QMD is an external CLI tool that provides:
- **Semantic Search**: Vector-based similarity search in markdown files
- **Collection Management**: Organize documents into searchable collections
- **Incremental Updates**: Efficiently update indexes when files change
- **Fast Queries**: Quick semantic search with relevance scoring

## Installation

QMD is an optional dependency. Install it from:
```bash
# Example (replace with actual QMD installation instructions)
go install github.com/username/qmd@latest
```

## Configuration

Enable QMD in your `config.json`:

```json
{
  "memory": {
    "qmd": {
      "enabled": true,
      "command": "qmd",
      "include_default": true,
      "paths": [
        {
          "name": "docs",
          "path": "~/Documents",
          "pattern": "**/*.md"
        },
        {
          "name": "notes",
          "path": "~/Notes",
          "pattern": "**/*.md"
        }
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

### Configuration Options

**Main Settings:**
- `enabled` - Enable/disable QMD integration
- `command` - QMD executable name or path (default: "qmd")
- `include_default` - Include default workspace memory collection

**Paths:**
Define custom collections to index:
- `name` - Collection identifier
- `path` - Directory to index (supports ~ expansion and env vars)
- `pattern` - Glob pattern for matching files (e.g., "**/*.md")

**Sessions:**
Export and index conversation sessions:
- `enabled` - Enable session export
- `export_dir` - Where to export session markdown files
- `retention_days` - How long to keep exports (0 = forever)

**Update:**
Automatic update configuration:
- `on_boot` - Update all collections when agent starts
- `interval` - Time between scheduled updates (e.g., "30m", "1h")
- `command_timeout` - Timeout for individual QMD commands
- `update_timeout` - Timeout for full update operations

## Usage

### Check QMD Status

```bash
nekobot qmd status
```

Output:
```
QMD Status:
  Available: true
  Version: qmd 1.0.0
  Collections: 3
  Last Update: 2026-02-13T15:30:00Z

Collections:
  - default
    Path: ~/.nekobot/workspace/memory
    Pattern: **/*.md
    Last Updated: 2026-02-13T15:30:00Z
  - docs
    Path: ~/Documents
    Pattern: **/*.md
    Last Updated: 2026-02-13T15:30:00Z
  - sessions
    Path: ~/.nekobot/workspace/memory/sessions
    Pattern: **/*.md
    Last Updated: 2026-02-13T15:30:00Z
```

### Update Collections

Manually trigger update of all collections:

```bash
nekobot qmd update
```

This will:
1. Initialize configured collections if they don't exist
2. Update indexes for all collections
3. Process any new or modified files

### Search

Search for content semantically:

```bash
nekobot qmd search default "how to use tools"
nekobot qmd search docs "api authentication"
nekobot qmd search sessions "previous conversation about database" --limit 5
```

Output:
```
Found 3 results for 'how to use tools' in collection 'default':

1. TOOLS.md (score: 0.92)
   Tools are powerful extensions that allow the agent to perform...

2. 2026-02-10.md (score: 0.78)
   Today I learned about the exec tool which allows running shell...

3. BOOTSTRAP.md (score: 0.65)
   The agent has access to various tools including file operations...
```

## Default Collections

When `include_default: true`, nekobot automatically creates:

**default** collection:
- Path: `${WORKSPACE}/memory/**/*.md`
- Includes: Daily logs, notes, MEMORY.md
- Updates: Automatically with other collections

**sessions** collection (if enabled):
- Path: `${WORKSPACE}/memory/sessions/**/*.md`
- Includes: Exported session conversations
- Updates: When sessions are exported

## Session Export

When sessions export is enabled, nekobot will:

1. **Export Sessions**: Convert JSONL session files to markdown
2. **Index with QMD**: Add to the sessions collection
3. **Cleanup Old Exports**: Remove exports older than retention period

This allows you to search through past conversations:
```bash
nekobot qmd search sessions "we discussed kubernetes deployment"
```

## Automatic Updates

QMD collections are kept up-to-date automatically:

### On Boot Update
When `update.on_boot: true`, all collections are updated when the agent starts.

### Scheduled Updates
Collections update periodically based on `update.interval`:
- "30m" - Every 30 minutes
- "1h" - Every hour
- "24h" - Daily

### Manual Updates
You can also trigger updates manually:
```bash
nekobot qmd update
```

## Integration with Agent

The agent can leverage QMD for semantic search in conversations:

```
User: Find my notes about API authentication
Agent: [Uses QMD to search across collections]
      I found 3 relevant notes:
      1. In docs/api.md - Discusses OAuth2 flow
      2. In memory/2026-02-10.md - Your implementation notes
      3. In sessions/session-123.md - Previous conversation about tokens
```

## Performance Tips

1. **Collection Size**: Keep collections focused
   - Separate large document sets into multiple collections
   - Use specific patterns (e.g., "docs/**/*.md" not "**/*.md")

2. **Update Frequency**: Balance freshness vs. resource usage
   - Frequent updates (30m) for active workspaces
   - Less frequent (1h+) for stable document sets

3. **Retention**: Clean up old exports
   - Set retention_days to prevent unbounded growth
   - 90 days is a good default for sessions

## Troubleshooting

### QMD Not Available

Error: `QMD not installed or not found in PATH`

**Solution**: Install QMD or disable in config:
```json
{
  "memory": {
    "qmd": {
      "enabled": false
    }
  }
}
```

### Collection Update Fails

Error: `Failed to update collection: timeout`

**Solutions**:
- Increase `update_timeout` in config
- Reduce collection size
- Check disk space and permissions

### Search Returns No Results

**Check**:
1. Collection exists: `nekobot qmd status`
2. Collection updated: Check "Last Updated" timestamp
3. Files indexed: Verify pattern matches your files
4. QMD working: Try updating manually

## Advanced Usage

### Custom Collections

Index any directory:
```json
{
  "memory": {
    "qmd": {
      "paths": [
        {
          "name": "projects",
          "path": "~/Projects",
          "pattern": "**/*.md"
        },
        {
          "name": "research",
          "path": "~/Research",
          "pattern": "**/*.{md,txt}"
        }
      ]
    }
  }
}
```

### Environment Variables

Use environment variables in paths:
```json
{
  "paths": [
    {
      "name": "custom",
      "path": "${DOCS_DIR}/notes",
      "pattern": "**/*.md"
    }
  ]
}
```

### Programmatic Usage

Use QMD in custom code:

```go
import (
    "nekobot/pkg/logger"
    "nekobot/pkg/memory/qmd"
)

// Create manager
log, _ := logger.New(&logger.Config{Level: logger.LevelInfo})
config := qmd.Config{
    Enabled: true,
    Command: "qmd",
    IncludeDefault: true,
}
manager := qmd.NewManager(log, config)

// Initialize collections
ctx := context.Background()
manager.Initialize(ctx, "/path/to/workspace")

// Search
results, err := manager.Search(ctx, "default", "query", 10)
for _, result := range results {
    fmt.Printf("%s (%.2f): %s\n", result.Path, result.Score, result.Snippet)
}

// Update
manager.UpdateAll(ctx)
```

## Benefits

1. **Semantic Understanding**: Find relevant content even with different wording
2. **Fast Search**: Quick results across large document sets
3. **Persistent Memory**: Search through past conversations and notes
4. **Context Awareness**: Agent can reference relevant past information
5. **Automatic Indexing**: Stay up-to-date without manual intervention

## Limitations

- Requires external QMD tool installation
- Adds processing time for indexing
- Storage overhead for vector indexes
- Best for markdown files (other formats may need conversion)
