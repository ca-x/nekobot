package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditFileTool allows the agent to edit specific parts of a file using string replacement.
type EditFileTool struct {
	workspace string
	restrict  bool
}

// NewEditFileTool creates a new edit_file tool.
func NewEditFileTool(workspace string, restrict bool) *EditFileTool {
	return &EditFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit a file by replacing an exact string match (old_string) with new content (new_string). " +
		"Provide enough context in old_string to uniquely identify the text to replace. " +
		"To delete text, set new_string to an empty string."
}

func (t *EditFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to edit (absolute or relative to workspace)",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "The exact string to find and replace. Must match uniquely in the file.",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "The replacement string. Use empty string to delete the matched text.",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}

	oldString, ok := args["old_string"].(string)
	if !ok {
		return "", fmt.Errorf("old_string must be a string")
	}

	newString, _ := args["new_string"].(string) // empty string is valid

	if oldString == "" {
		return "", fmt.Errorf("old_string cannot be empty")
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

	fileContent := string(content)

	// Check that old_string exists and is unique
	count := strings.Count(fileContent, oldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in file")
	}
	if count > 1 {
		return "", fmt.Errorf("old_string matches %d locations; provide more context to make it unique", count)
	}

	// Replace
	newContent := strings.Replace(fileContent, oldString, newString, 1)

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", filepath.Base(path)), nil
}

func (t *EditFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workspace, path)
}

func (t *EditFileTool) checkPathInWorkspace(path string) error {
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

// AppendFileTool allows the agent to append content to an existing file.
type AppendFileTool struct {
	workspace string
	restrict  bool
}

// NewAppendFileTool creates a new append_file tool.
func NewAppendFileTool(workspace string, restrict bool) *AppendFileTool {
	return &AppendFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

func (t *AppendFileTool) Name() string {
	return "append_file"
}

func (t *AppendFileTool) Description() string {
	return "Append content to the end of an existing file. Creates the file if it doesn't exist."
}

func (t *AppendFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to append to (absolute or relative to workspace)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to append to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *AppendFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Open file in append mode (create if not exists)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("failed to append to file: %w", err)
	}

	return fmt.Sprintf("Successfully appended %d bytes to %s", n, filepath.Base(path)), nil
}

func (t *AppendFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workspace, path)
}

func (t *AppendFileTool) checkPathInWorkspace(path string) error {
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
