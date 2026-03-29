// Package tools provides the undo tool for reverting to previous conversation turns.
package tools

import (
	"context"
	"fmt"

	"nekobot/pkg/session"
)

// UndoTool provides turn-based undo functionality.
type UndoTool struct {
	snapshotMgr *session.SnapshotManager
	sessionID   string
}

// UndoToolOptions contains options for the undo tool.
type UndoToolOptions struct {
	SnapshotMgr *session.SnapshotManager
	SessionID   string
}

// NewUndoTool creates a new undo tool.
func NewUndoTool(opts UndoToolOptions) *UndoTool {
	return &UndoTool{
		snapshotMgr: opts.SnapshotMgr,
		sessionID:   opts.SessionID,
	}
}

// Name returns the tool name.
func (t *UndoTool) Name() string {
	return "undo"
}

// Description returns the tool description.
func (t *UndoTool) Description() string {
	return "Revert the conversation to the previous turn, undoing the last assistant response and any tool calls it made."
}

// Parameters returns the tool parameter schema.
func (t *UndoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
}

// Execute performs the undo operation.
func (t *UndoTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	_ = ctx
	_ = params

	if t.snapshotMgr == nil {
		return "", fmt.Errorf("snapshot manager is not initialized")
	}

	store := t.snapshotMgr.GetStore(t.sessionID)
	if store == nil {
		return "", fmt.Errorf("snapshot store not found for session %s", t.sessionID)
	}

	// Load snapshots from disk if not already loaded
	if err := store.LoadSnapshots(); err != nil {
		return "", fmt.Errorf("loading snapshots: %w", err)
	}

	if !store.CanUndo() {
		return "Nothing to undo - already at the beginning of the conversation", nil
	}

	messages, err := store.Undo()
	if err != nil {
		return "", fmt.Errorf("undo operation failed: %w", err)
	}

	if messages == nil {
		return "Nothing to undo - already at the beginning of the conversation", nil
	}

	turn := store.GetCurrentTurn()
	return fmt.Sprintf("Successfully undone to turn %d. The conversation has been reverted to its previous state.", turn), nil
}

// CanUndo returns true if undo is possible.
func (t *UndoTool) CanUndo() bool {
	if t.snapshotMgr == nil {
		return false
	}
	store := t.snapshotMgr.GetStore(t.sessionID)
	if store == nil {
		return false
	}
	if err := store.LoadSnapshots(); err != nil {
		return false
	}
	return store.CanUndo()
}

// GetTurnCount returns the number of turns stored.
func (t *UndoTool) GetTurnCount() int {
	if t.snapshotMgr == nil {
		return 0
	}
	store := t.snapshotMgr.GetStore(t.sessionID)
	if store == nil {
		return 0
	}
	if err := store.LoadSnapshots(); err != nil {
		return 0
	}
	return store.GetTurnCount()
}

// GetCurrentTurn returns the current turn number.
func (t *UndoTool) GetCurrentTurn() int {
	if t.snapshotMgr == nil {
		return -1
	}
	store := t.snapshotMgr.GetStore(t.sessionID)
	if store == nil {
		return -1
	}
	if err := store.LoadSnapshots(); err != nil {
		return -1
	}
	return store.GetCurrentTurn()
}
