// Package audit provides FX module for tool execution auditing.
package audit

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides audit logging capabilities.
var Module = fx.Module("audit",
	fx.Provide(NewLoggerFromConfig),
)

// Params holds dependencies for creating an audit logger.
type Params struct {
	fx.In

	Config    *config.Config
	Log       *logger.Logger
	Lifecycle fx.Lifecycle
}

// NewLoggerFromConfig creates an audit logger from configuration.
func NewLoggerFromConfig(p Params) *Logger {
	cfg := Config{
		Enabled:       p.Config.Audit.Enabled,
		MaxArgLength:  p.Config.Audit.MaxArgLength,
		MaxResults:    p.Config.Audit.MaxResults,
		RetentionDays: p.Config.Audit.RetentionDays,
	}

	// Apply defaults
	if cfg.MaxArgLength <= 0 {
		cfg.MaxArgLength = 1000
	}

	logger := NewLogger(cfg, p.Config.Agents.Defaults.Workspace, p.Log)

	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return logger.Close()
		},
	})

	return logger
}

// Hook returns a function that can be used as a tool execution hook.
// The hook logs tool executions to the audit log.
func (l *Logger) Hook() func(ctx context.Context, toolName string, args map[string]interface{}, result string, duration time.Duration, err error) {
	return func(ctx context.Context, toolName string, args map[string]interface{}, result string, duration time.Duration, err error) {
		entry := &Entry{
			ToolName:   toolName,
			Arguments:  args,
			DurationMs: duration.Milliseconds(),
			Success:    err == nil,
			Workspace:  l.workspace,
		}

		// Extract session ID from context if available
		if sessionID, ok := ctx.Value("session_id").(string); ok {
			entry.SessionID = sessionID
		}

		if err != nil {
			entry.Error = err.Error()
		}

		// Add result preview (truncated)
		if result != "" {
			maxPreview := 500
			if len(result) > maxPreview {
				entry.ResultPreview = result[:maxPreview] + "... [truncated]"
			} else {
				entry.ResultPreview = result
			}
		}

		l.Log(entry)

		if l.log != nil {
			l.log.Debug("Tool executed",
				zap.String("tool", toolName),
				zap.Int64("duration_ms", entry.DurationMs),
				zap.Bool("success", err == nil),
			)
		}
	}
}