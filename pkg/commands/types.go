// Package commands provides a unified command system for channels.
package commands

import (
	"context"
)

// Command represents a slash command that can be executed.
type Command struct {
	// Name is the command name (without /)
	Name string
	// Description is a short description of what the command does
	Description string
	// Usage shows how to use the command
	Usage string
	// Handler is the function that executes the command
	Handler CommandHandler
	// RequiresAuth indicates if the command requires authentication
	RequiresAuth bool
	// AdminOnly indicates if only admins can use this command
	AdminOnly bool
}

// CommandHandler is a function that handles a command.
type CommandHandler func(ctx context.Context, req CommandRequest) (CommandResponse, error)

// CommandRequest contains information about a command invocation.
type CommandRequest struct {
	// Channel is the channel name (telegram, discord, slack, etc.)
	Channel string
	// ChatID identifies the conversation
	ChatID string
	// UserID identifies the user who invoked the command
	UserID string
	// Username is the display name of the user
	Username string
	// Command is the command name
	Command string
	// Args are the command arguments (text after the command)
	Args string
	// Metadata contains channel-specific metadata
	Metadata map[string]string
}

// CommandResponse contains the command execution result.
type CommandResponse struct {
	// Content is the response text
	Content string
	// Ephemeral indicates if the response should only be visible to the user
	Ephemeral bool
	// ReplyInline indicates if the response should be sent inline (true)
	// or as a regular message through the bus (false)
	ReplyInline bool
	// Interaction contains optional structured interaction payload.
	Interaction *CommandInteraction
}

// CommandInteraction describes optional interactive follow-up actions.
type CommandInteraction struct {
	// Type identifies interaction type (e.g., "skill_install_confirm").
	Type string
	// Repo is used by skill-install confirmation flows.
	Repo string
	// Reason is optional explanation text.
	Reason string
	// Message is a user-facing prompt.
	Message string
	// Command is command name to re-run after confirmation.
	Command string
}
