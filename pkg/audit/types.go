// Package audit provides tool execution auditing for debugging and analysis.
package audit

import (
	"time"
)

// Entry represents a single audit log entry.
type Entry struct {
	Timestamp     time.Time              `json:"ts"`
	ToolName      string                 `json:"tool"`
	Arguments     map[string]interface{} `json:"args,omitempty"`
	DurationMs    int64                  `json:"duration_ms"`
	Success       bool                   `json:"success"`
	ResultPreview string                 `json:"result_preview,omitempty"`
	Error         string                 `json:"error,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	Workspace     string                 `json:"workspace,omitempty"`
}

// Config holds audit logging configuration.
type Config struct {
	Enabled      bool `mapstructure:"enabled" json:"enabled"`
	MaxArgLength int  `mapstructure:"max_arg_length" json:"max_arg_length"`   // Truncate args longer than this
	MaxResults   int  `mapstructure:"max_results" json:"max_results"`         // Max entries to keep (0 = unlimited)
	RetentionDays int `mapstructure:"retention_days" json:"retention_days"`   // Delete entries older than this
}

// DefaultConfig returns the default audit configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		MaxArgLength:  1000,
		MaxResults:    10000,
		RetentionDays: 30,
	}
}