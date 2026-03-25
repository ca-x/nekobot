package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Status describes the current workspace bootstrap and file state.
type Status struct {
	Path                 string   `json:"path"`
	Exists               bool     `json:"exists"`
	Bootstrapped         bool     `json:"bootstrapped"`
	BootstrapFiles       []string `json:"bootstrap_files"`
	MissingBootstrap     []string `json:"missing_bootstrap"`
	TodayLogPath         string   `json:"today_log_path"`
	TodayLogExists       bool     `json:"today_log_exists"`
	HeartbeatStatePath   string   `json:"heartbeat_state_path"`
	HeartbeatStateExists bool     `json:"heartbeat_state_exists"`
	FileCount            int      `json:"file_count"`
	DirectoryCount       int      `json:"directory_count"`
	UpdatedAt            string   `json:"updated_at"`
}

// Inspect returns the current workspace status without mutating files.
func (m *Manager) Inspect() (*Status, error) {
	status := &Status{
		Path:               m.workspaceDir,
		BootstrapFiles:     append([]string(nil), BootstrapFiles...),
		TodayLogPath:       filepath.Join(m.workspaceDir, "memory", time.Now().Format("2006-01-02")+".md"),
		HeartbeatStatePath: filepath.Join(m.workspaceDir, "memory", "heartbeat-state.json"),
		UpdatedAt:          time.Now().Format(time.RFC3339),
	}

	info, err := os.Stat(m.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			status.MissingBootstrap = append([]string(nil), BootstrapFiles...)
			sort.Strings(status.MissingBootstrap)
			return status, nil
		}
		return nil, fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace path is not a directory: %s", m.workspaceDir)
	}

	status.Exists = true
	for _, filename := range BootstrapFiles {
		path := filepath.Join(m.workspaceDir, filename)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if os.IsNotExist(err) {
			status.MissingBootstrap = append(status.MissingBootstrap, filename)
			continue
		} else {
			return nil, fmt.Errorf("stat bootstrap file %s: %w", filename, err)
		}
	}
	sort.Strings(status.MissingBootstrap)
	status.Bootstrapped = len(status.MissingBootstrap) == 0

	if _, err := os.Stat(status.TodayLogPath); err == nil {
		status.TodayLogExists = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat today log: %w", err)
	}

	if _, err := os.Stat(status.HeartbeatStatePath); err == nil {
		status.HeartbeatStateExists = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat heartbeat state: %w", err)
	}

	if err := filepath.Walk(m.workspaceDir, func(path string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fileInfo.IsDir() {
			status.DirectoryCount++
			return nil
		}
		status.FileCount++
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk workspace: %w", err)
	}

	return status, nil
}
