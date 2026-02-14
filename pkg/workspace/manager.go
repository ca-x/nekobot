package workspace

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nekobot/pkg/logger"
	"nekobot/pkg/version"
)

//go:embed templates/*.md
var templatesFS embed.FS

// BootstrapFiles defines all bootstrap template files.
var BootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"IDENTITY.md",
	"USER.md",
	"TOOLS.md",
	"HEARTBEAT.md",
	"BOOT.md",
	"BOOTSTRAP.md",
}

// Manager manages workspace directory operations.
type Manager struct {
	workspaceDir string
	log          *logger.Logger
}

// NewManager creates a new workspace manager.
func NewManager(workspaceDir string, log *logger.Logger) *Manager {
	return &Manager{
		workspaceDir: workspaceDir,
		log:          log,
	}
}

// GetDefaultWorkspaceDir returns the default workspace directory.
func GetDefaultWorkspaceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".nekobot", "workspace"), nil
}

// Ensure ensures workspace directory exists with all necessary files.
func (m *Manager) Ensure() error {
	// Create workspace directory
	if err := os.MkdirAll(m.workspaceDir, 0755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"memory", "skills", "sessions", ".nekobot/skills", ".nekobot/snapshots"}
	for _, subdir := range subdirs {
		path := filepath.Join(m.workspaceDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", subdir, err)
		}
	}

	// Copy bootstrap files if they don't exist
	for _, filename := range BootstrapFiles {
		if err := m.ensureFile(filename); err != nil {
			return fmt.Errorf("ensuring %s: %w", filename, err)
		}
	}

	// Create today's log if it doesn't exist
	if err := m.ensureTodayLog(); err != nil {
		return fmt.Errorf("creating today's log: %w", err)
	}

	// Create heartbeat state file if it doesn't exist
	heartbeatState := filepath.Join(m.workspaceDir, "memory", "heartbeat-state.json")
	if _, err := os.Stat(heartbeatState); os.IsNotExist(err) {
		if err := os.WriteFile(heartbeatState, []byte("{}"), 0644); err != nil {
			return fmt.Errorf("creating heartbeat state: %w", err)
		}
	}

	return nil
}

// ensureFile ensures a single file exists, creating from template if needed.
func (m *Manager) ensureFile(filename string) error {
	targetPath := filepath.Join(m.workspaceDir, filename)

	// Check if file already exists
	if _, err := os.Stat(targetPath); err == nil {
		// File exists, don't overwrite
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// Read template from embedded FS
	templatePath := filepath.Join("templates", filename)
	templateData, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	// Render template with variables
	rendered, err := RenderTemplate(string(templateData), m.getTemplateVars())
	if err != nil {
		return fmt.Errorf("rendering template %s: %w", filename, err)
	}

	// Write to target
	if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", targetPath, err)
	}

	m.log.Info(fmt.Sprintf("Created workspace file: %s", filename))
	return nil
}

// ensureTodayLog creates today's daily log if it doesn't exist.
func (m *Manager) ensureTodayLog() error {
	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(m.workspaceDir, "memory", today+".md")

	// Check if exists
	if _, err := os.Stat(logPath); err == nil {
		return nil // Already exists
	} else if !os.IsNotExist(err) {
		return err
	}

	// Read DAILY template
	templateData, err := templatesFS.ReadFile("templates/DAILY.md")
	if err != nil {
		return fmt.Errorf("reading DAILY template: %w", err)
	}

	// Render with today's date
	vars := m.getTemplateVars()
	vars["Date"] = today
	vars["DayOfWeek"] = time.Now().Format("Monday")

	rendered, err := RenderTemplate(string(templateData), vars)
	if err != nil {
		return fmt.Errorf("rendering daily log: %w", err)
	}

	// Write log file
	if err := os.WriteFile(logPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("writing daily log: %w", err)
	}

	m.log.Info(fmt.Sprintf("Created daily log: %s", today))
	return nil
}

// getTemplateVars returns variables for template rendering.
func (m *Manager) getTemplateVars() map[string]string {
	now := time.Now()

	return map[string]string{
		"Date":      now.Format("2006-01-02"),
		"DayOfWeek": now.Format("Monday"),
		"AgentID":   "nekobot",
		"AgentName": "Nekobot",
		"Version":   version.GetVersion(),
		"Workspace": m.workspaceDir,
		"Timezone":  now.Format("MST"),
		"Model":     "claude-3-5-sonnet-20241022", // Default, will be from config
		"Provider":  "anthropic",                  // Default, will be from config
	}
}

// GetWorkspaceDir returns the workspace directory path.
func (m *Manager) GetWorkspaceDir() string {
	return m.workspaceDir
}

// ListFiles returns all workspace files.
func (m *Manager) ListFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(m.workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(m.workspaceDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}
