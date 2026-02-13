# NekoBot Logger - Dual Output Guide

## Overview

The NekoBot logger **automatically outputs to both console and file** when configured with an output path. This is a built-in feature that requires no additional configuration.

## How It Works

```
                   ┌─────────────────┐
                   │   Your Log Call │
                   └────────┬────────┘
                            │
                ┌───────────┴───────────┐
                │                       │
         ┌──────▼──────┐       ┌───────▼────────┐
         │   Console   │       │   File (JSON)  │
         │  (Colored)  │       │  + Rotation    │
         └─────────────┘       └────────────────┘
```

## Basic Usage

```go
package main

import (
    "nekobot/pkg/logger"
    "go.uber.org/zap"
)

func main() {
    // Configure logger with file path
    cfg := logger.DefaultConfig()
    cfg.OutputPath = "/var/log/nekobot/app.log"  // Enable file output
    cfg.Development = true                        // Enable colored console

    log, err := logger.New(cfg)
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // This single log call outputs to BOTH:
    // 1. Console (colored, human-readable)
    // 2. File (JSON, with rotation)
    log.Info("Server started",
        zap.String("host", "0.0.0.0"),
        zap.Int("port", 8080),
    )
}
```

## Output Examples

### Console Output (Development Mode)
```
INFO [2026-02-13T20:00:00.000Z] Server started  host=0.0.0.0 port=8080
```

### File Output (JSON)
```json
{"level":"info","time":"2026-02-13T20:00:00.000Z","msg":"Server started","host":"0.0.0.0","port":8080}
```

## Configuration Options

### Console-Only (No File)
```go
cfg := logger.DefaultConfig()
cfg.OutputPath = ""  // Empty = console only
```

### File with Rotation
```go
cfg := logger.DefaultConfig()
cfg.OutputPath = "/var/log/nekobot/app.log"
cfg.MaxSize = 100      // 100MB per file
cfg.MaxBackups = 5     // Keep 5 old files
cfg.MaxAge = 30        // Keep for 30 days
cfg.Compress = true    // Compress rotated files
```

### Different Formats

The logger intelligently uses different formats for each output:

| Output  | Development Mode | Production Mode |
|---------|------------------|-----------------|
| Console | Colored, human-readable | JSON |
| File    | JSON (always)    | JSON (always) |

## Advanced: Custom Outputs

```go
// You can also create multi-target outputs manually
import "gopkg.in/natefinch/lumberjack.v2"

fileWriter := &lumberjack.Logger{
    Filename:   "/var/log/nekobot/app.log",
    MaxSize:    100,
    MaxBackups: 5,
}

// The logger automatically tees to both console and file
```

## Why Dual Output?

1. **Development**: See logs in real-time on console
2. **Production**: Persistent logs in files for auditing
3. **Debugging**: Console for quick checks, files for detailed analysis
4. **Monitoring**: Files can be ingested by log aggregators
5. **Compliance**: File logs provide audit trail

## Best Practices

1. **Always sync on exit**: `defer log.Sync()`
2. **Use structured fields**: Makes logs searchable
3. **Set appropriate levels**: Reduce noise in production
4. **Rotate regularly**: Prevent disk fill
5. **Monitor log sizes**: Set MaxSize appropriately

## Testing

```bash
# Run your app
./nekobot

# Console will show colored output
# File will accumulate JSON logs

# View file logs
tail -f /var/log/nekobot/app.log

# Search file logs
grep '"level":"error"' /var/log/nekobot/app.log
```

## Integration with fx

```go
import (
    "go.uber.org/fx"
    "nekobot/pkg/logger"
)

func main() {
    fx.New(
        logger.Module,  // Provides logger with dual output
        fx.Invoke(run),
    ).Run()
}

func run(log *logger.Logger) {
    // Automatically logs to both console and file
    log.Info("Application started")
}
```

## Troubleshooting

### Logs not appearing in file?

1. Check file permissions: `ls -la /var/log/nekobot/`
2. Ensure directory exists: `mkdir -p /var/log/nanobot`
3. Call `Sync()` before exit: `defer log.Sync()`

### Console too noisy?

Set level to `LevelWarn` or `LevelError`:
```go
cfg.Level = logger.LevelWarn
```

### File too large?

Reduce `MaxSize` or `MaxAge`:
```go
cfg.MaxSize = 50  // 50MB instead of 100MB
```
