package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool allows the agent to read file contents.
type ReadFileTool struct {
	workspace string
	restrict  bool // If true, only allow access within workspace
}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool(workspace string, restrict bool) *ReadFileTool {
	return &ReadFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Provide the file path (absolute or relative to workspace)."
}

func (t *ReadFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read (absolute or relative to workspace)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// resolvePath resolves a path relative to workspace if it's not absolute.
func (t *ReadFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workspace, path)
}

// checkPathInWorkspace ensures the path is within the workspace.
func (t *ReadFileTool) checkPathInWorkspace(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absWorkspace, err := filepath.Abs(t.workspace)
	if err != nil {
		return fmt.Errorf("invalid workspace: %w", err)
	}

	if !strings.HasPrefix(absPath, absWorkspace) {
		return fmt.Errorf("access denied: path outside workspace")
	}

	return nil
}

// WriteFileTool allows the agent to write file contents.
type WriteFileTool struct {
	workspace string
	restrict  bool
}

// NewWriteFileTool creates a new write_file tool.
func NewWriteFileTool(workspace string, restrict bool) *WriteFileTool {
	return &WriteFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Creates parent directories as needed."
}

func (t *WriteFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write (absolute or relative to workspace)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content must be a string")
	}

	// Resolve path
	path := t.resolvePath(pathArg)

	// Security check
	if t.restrict {
		if err := t.checkPathInWorkspace(path); err != nil {
			return "", err
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

func (t *WriteFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workspace, path)
}

func (t *WriteFileTool) checkPathInWorkspace(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absWorkspace, err := filepath.Abs(t.workspace)
	if err != nil {
		return fmt.Errorf("invalid workspace: %w", err)
	}

	if !strings.HasPrefix(absPath, absWorkspace) {
		return fmt.Errorf("access denied: path outside workspace")
	}

	return nil
}
