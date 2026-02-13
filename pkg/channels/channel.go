// Package channels provides the channel interface and management for multi-platform support.
package channels

import (
	"context"

	"nekobot/pkg/bus"
)

// Channel represents a communication channel (Telegram, Discord, etc).
type Channel interface {
	// ID returns the unique channel identifier.
	ID() string

	// Name returns the human-readable channel name.
	Name() string

	// Start starts the channel and begins listening for messages.
	Start(ctx context.Context) error

	// Stop stops the channel gracefully.
	Stop(ctx context.Context) error

	// IsEnabled returns whether the channel is enabled in configuration.
	IsEnabled() bool

	// SendMessage sends a message through this channel.
	SendMessage(ctx context.Context, msg *bus.Message) error
}

// ChannelConfig is the interface for channel-specific configuration.
type ChannelConfig interface {
	// IsEnabled returns whether the channel is enabled.
	IsEnabled() bool

	// GetAllowList returns the list of allowed users/groups.
	GetAllowList() []string

	// Validate validates the configuration.
	Validate() error
}
