// Package bus provides message routing between channels and agents.
package bus

import (
	"context"
	"time"
)

// MessageType represents the type of message.
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeVideo    MessageType = "video"
	MessageTypeFile     MessageType = "file"
	MessageTypeLocation MessageType = "location"
	MessageTypeCommand  MessageType = "command"
)

// Message represents a message flowing through the bus.
type Message struct {
	ID        string                 `json:"id"`         // Unique message ID
	ChannelID string                 `json:"channel_id"` // Source/target channel
	SessionID string                 `json:"session_id"` // Session/conversation ID
	UserID    string                 `json:"user_id"`    // User identifier
	Username  string                 `json:"username"`   // User display name
	Type      MessageType            `json:"type"`       // Message type
	Content   string                 `json:"content"`    // Text content
	Data      map[string]interface{} `json:"data"`       // Additional data
	Timestamp time.Time              `json:"timestamp"`  // Message timestamp
	ReplyTo   string                 `json:"reply_to"`   // ID of message being replied to
}

// Handler is a function that processes messages.
type Handler func(ctx context.Context, msg *Message) error

// Bus is the interface for message routing.
type Bus interface {
	// Start starts the message bus.
	Start() error

	// Stop stops the message bus.
	Stop() error

	// RegisterHandler registers a handler for a specific channel.
	RegisterHandler(channelID string, handler Handler)

	// UnregisterHandlers removes all handlers for a channel.
	UnregisterHandlers(channelID string)

	// SendInbound sends an inbound message (from channel to agent).
	SendInbound(msg *Message) error

	// SendOutbound sends an outbound message (from agent to channel).
	SendOutbound(msg *Message) error

	// GetMetrics returns current bus metrics.
	GetMetrics() map[string]uint64
}
