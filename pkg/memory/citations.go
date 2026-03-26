package memory

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatCitation formats a stable citation string for one memory result.
func FormatCitation(result *SearchResult) string {
	if result == nil {
		return ""
	}

	var parts []string
	if path := strings.TrimSpace(result.Metadata.FilePath); path != "" {
		parts = append(parts, path)
	} else if sessionKey := strings.TrimSpace(result.Metadata.SessionKey); sessionKey != "" {
		parts = append(parts, sessionKey)
	}

	if lineRange := citationLineRange(result.Metadata); lineRange != "" {
		parts = append(parts, lineRange)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(result.ID)
	}
	return strings.Join(parts, "#")
}

// BuildRelativePath makes a file path relative to workspace when useful.
func BuildRelativePath(filePath, workspaceDir string) string {
	if strings.TrimSpace(workspaceDir) == "" {
		return filePath
	}
	rel, err := filepath.Rel(workspaceDir, filePath)
	if err != nil {
		return filePath
	}
	if len(rel) < len(filePath) && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filePath
}

func citationLineRange(metadata Metadata) string {
	startLine := metadata.LineNumber
	endLine := metadata.EndLineNumber
	if startLine <= 0 {
		return ""
	}
	if endLine > 0 && endLine != startLine {
		return fmt.Sprintf("L%d-L%d", startLine, endLine)
	}
	return fmt.Sprintf("L%d", startLine)
}
