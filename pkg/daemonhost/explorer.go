package daemonhost

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

const (
	DefaultWorkspaceTreeListLimit = 200
	MaxWorkspaceTreeListLimit     = 1000
	DefaultWorkspaceFileMaxBytes  = 64 * 1024
	MaxWorkspaceFileBytes         = 256 * 1024
)

var errWorkspaceNotFound = errors.New("workspace not found")

func findWorkspaceByID(inventory *daemonv1.RuntimeInventory, workspaceID string) (*daemonv1.Workspace, error) {
	if inventory == nil {
		return nil, errWorkspaceNotFound
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	for _, workspace := range inventory.Workspaces {
		if workspace == nil {
			continue
		}
		if strings.TrimSpace(workspace.WorkspaceId) == workspaceID {
			return workspace, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errWorkspaceNotFound, workspaceID)
}

func normalizeWorkspaceRelativePath(relPath string) (string, error) {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" || trimmed == "." || trimmed == "/" {
		return "", nil
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	for strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path escapes workspace root")
	}
	return cleaned, nil
}

func safeWorkspacePath(workspace *daemonv1.Workspace, relPath string) (string, string, string, error) {
	if workspace == nil {
		return "", "", "", fmt.Errorf("workspace is required")
	}
	root := workspaceDir(workspace)
	if strings.TrimSpace(root) == "" {
		return "", "", "", fmt.Errorf("workspace path is required")
	}
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve workspace root: %w", err)
	}
	cleanRel, err := normalizeWorkspaceRelativePath(relPath)
	if err != nil {
		return "", "", "", err
	}
	resolvedPath := resolvedRoot
	if cleanRel != "" {
		resolvedPath = filepath.Join(resolvedRoot, filepath.FromSlash(cleanRel))
	}
	resolvedPath, err = filepath.Abs(resolvedPath)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve workspace path: %w", err)
	}
	if relToRoot, relErr := filepath.Rel(resolvedRoot, resolvedPath); relErr != nil {
		return "", "", "", fmt.Errorf("resolve workspace path: %w", relErr)
	} else {
		relToRoot = filepath.Clean(relToRoot)
		if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			return "", "", "", fmt.Errorf("path escapes workspace root")
		}
	}
	return resolvedRoot, resolvedPath, cleanRel, nil
}

func normalizeWorkspaceTreeListLimit(limit uint32) int {
	if limit == 0 {
		return DefaultWorkspaceTreeListLimit
	}
	if limit > MaxWorkspaceTreeListLimit {
		return MaxWorkspaceTreeListLimit
	}
	return int(limit)
}

func normalizeWorkspaceReadLimit(limit uint32) int {
	if limit == 0 {
		return DefaultWorkspaceFileMaxBytes
	}
	if limit > MaxWorkspaceFileBytes {
		return MaxWorkspaceFileBytes
	}
	return int(limit)
}

func ListWorkspaceTree(inventory *daemonv1.RuntimeInventory, req *daemonv1.ListWorkspaceTreeRequest) (*daemonv1.ListWorkspaceTreeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	workspace, err := findWorkspaceByID(inventory, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	_, resolvedPath, cleanRel, err := safeWorkspacePath(workspace, req.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("stat workspace path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("read workspace directory: %w", err)
	}
	items := make([]*daemonv1.WorkspaceTreeEntry, 0, len(entries))
	for _, entry := range entries {
		childPath := entry.Name()
		if cleanRel != "" {
			childPath = filepath.ToSlash(filepath.Join(cleanRel, entry.Name()))
		}
		item := &daemonv1.WorkspaceTreeEntry{Path: childPath, Name: entry.Name(), IsDir: entry.IsDir()}
		if info, statErr := entry.Info(); statErr == nil {
			item.SizeBytes = info.Size()
			item.ModifiedTimeUnix = info.ModTime().Unix()
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	if limit := normalizeWorkspaceTreeListLimit(req.Limit); len(items) > limit {
		items = items[:limit]
	}
	return &daemonv1.ListWorkspaceTreeResponse{WorkspaceId: workspace.WorkspaceId, Path: cleanRel, Entries: items}, nil
}

func ReadWorkspaceFile(inventory *daemonv1.RuntimeInventory, req *daemonv1.ReadWorkspaceFileRequest) (*daemonv1.ReadWorkspaceFileResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	workspace, err := findWorkspaceByID(inventory, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	_, resolvedPath, cleanRel, err := safeWorkspacePath(workspace, req.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("stat workspace path: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("open workspace file: %w", err)
	}
	defer file.Close()
	limit := normalizeWorkspaceReadLimit(req.MaxBytes)
	content, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read workspace file: %w", err)
	}
	truncated := len(content) > limit
	if truncated {
		content = content[:limit]
	}
	return &daemonv1.ReadWorkspaceFileResponse{WorkspaceId: workspace.WorkspaceId, Path: cleanRel, Content: string(content), Truncated: truncated, SizeBytes: info.Size()}, nil
}
