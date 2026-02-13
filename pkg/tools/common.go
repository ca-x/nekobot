package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ListDirTool allows the agent to list directory contents.
type ListDirTool struct {
	workspace string
	restrict  bool
}

// NewListDirTool creates a new list_dir tool.
func NewListDirTool(workspace string, restrict bool) *ListDirTool {
	return &ListDirTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *ListDirTool) Name() string {
	return "list_dir"
}

func (t *ListDirTool) Description() string {
	return "List contents of a directory. Returns file names, types, and sizes."
}

func (t *ListDirTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the directory (absolute or relative to workspace). Use '.' for workspace root.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}

	// Resolve path
	path := t.resolvePath(pathArg)

	// Security check
	if t.restrict {
		if err := t.checkPathInWorkspace(path); err != nil {
			return "", err
		}
	}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	// Format output
	var output string
	output += fmt.Sprintf("Contents of %s:\n\n", path)

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		typeStr := "file"
		sizeStr := fmt.Sprintf("%d bytes", info.Size())
		if entry.IsDir() {
			typeStr = "dir"
			sizeStr = "-"
		}

		output += fmt.Sprintf("%-40s [%s] %s\n", entry.Name(), typeStr, sizeStr)
	}

	if len(entries) == 0 {
		output += "(empty directory)\n"
	}

	return output, nil
}

func (t *ListDirTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workspace, path)
}

func (t *ListDirTool) checkPathInWorkspace(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absWorkspace, err := filepath.Abs(t.workspace)
	if err != nil {
		return fmt.Errorf("invalid workspace: %w", err)
	}

	// Check if path is within workspace
	relPath, err := filepath.Rel(absWorkspace, absPath)
	if err != nil || len(relPath) > 0 && relPath[0] == '.' && relPath[1] == '.' {
		return fmt.Errorf("access denied: path outside workspace")
	}

	return nil
}

// MessageTool allows the agent to send messages directly to the user.
type MessageTool struct {
	sendFunc func(message string) error
}

// NewMessageTool creates a new message tool.
func NewMessageTool(sendFunc func(string) error) *MessageTool {
	return &MessageTool{
		sendFunc: sendFunc,
	}
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send a message directly to the user. Use this for important notifications or when you need to communicate something separate from your main response."
}

func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The message content to send to the user",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MessageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content must be a string")
	}

	if t.sendFunc != nil {
		if err := t.sendFunc(content); err != nil {
			return "", fmt.Errorf("failed to send message: %w", err)
		}
	}

	return "Message sent to user", nil
}
