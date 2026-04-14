package externalagent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"nekobot/pkg/config"
	"nekobot/pkg/toolsessions"
)

const (
	metadataAgentKind = "external_agent_kind"
	metadataWorkspace = "external_agent_workspace"
	metadataTool      = "external_agent_tool"
	metadataCommand   = "external_agent_command"
)

type launcherSpec struct {
	tool    string
	command string
}

// SessionSpec describes the external-agent session identity and launch contract.
type SessionSpec struct {
	Owner     string
	AgentKind string
	Workspace string
	Tool      string
	Title     string
	Command   string
}

// Manager resolves or creates long-lived per-user external-agent sessions.
type Manager struct {
	cfg      *config.Config
	sessions *toolsessions.Manager
}

// NewManager creates an external-agent session manager.
func NewManager(cfg *config.Config, sessions *toolsessions.Manager) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if sessions == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return &Manager{cfg: cfg, sessions: sessions}, nil
}

// ResolveSession finds an existing reusable session or creates a new one.
func (m *Manager) ResolveSession(ctx context.Context, spec SessionSpec) (*toolsessions.Session, bool, error) {
	if m == nil || m.sessions == nil {
		return nil, false, fmt.Errorf("tool session manager is required")
	}

	normalized, err := m.normalizeLaunchSpec(spec)
	if err != nil {
		return nil, false, err
	}

	items, err := m.sessions.ListSessions(ctx, toolsessions.ListSessionsInput{
		Owner:  normalized.Owner,
		Source: toolsessions.SourceAgent,
		Limit:  500,
	})
	if err != nil {
		return nil, false, err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.State == toolsessions.StateArchived || item.State == toolsessions.StateTerminated {
			continue
		}
		if metadataString(item.Metadata, metadataAgentKind) != normalized.AgentKind ||
			metadataString(item.Metadata, metadataWorkspace) != normalized.Workspace {
			continue
		}
		if strings.TrimSpace(item.Tool) != normalized.Tool {
			continue
		}
		if strings.TrimSpace(item.Command) != normalized.Command {
			continue
		}
		if metadataString(item.Metadata, metadataTool) != "" && metadataString(item.Metadata, metadataTool) != normalized.Tool {
			continue
		}
		if metadataString(item.Metadata, metadataCommand) != "" && metadataString(item.Metadata, metadataCommand) != normalized.Command {
			continue
		}
		if filepath.Clean(strings.TrimSpace(item.Workdir)) != normalized.Workspace {
			continue
		}
		return item, false, nil
	}

	created, err := m.sessions.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   normalized.Owner,
		Source:  toolsessions.SourceAgent,
		Tool:    normalized.Tool,
		Title:   normalized.Title,
		Command: normalized.Command,
		Workdir: normalized.Workspace,
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: normalized.AgentKind,
			metadataWorkspace: normalized.Workspace,
			metadataTool:      normalized.Tool,
			metadataCommand:   normalized.Command,
		},
	})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// NormalizeLaunchSpec resolves workspace defaults and applies launcher policy
// without creating or reusing a session.
func NormalizeLaunchSpec(cfg *config.Config, spec SessionSpec) (SessionSpec, error) {
	manager := &Manager{cfg: cfg}
	return manager.normalizeLaunchSpec(spec)
}

func (m *Manager) normalizeLaunchSpec(spec SessionSpec) (SessionSpec, error) {
	resolvedWorkspace, err := m.resolveWorkspace(spec.Workspace)
	if err != nil {
		return SessionSpec{}, err
	}
	spec.Workspace = resolvedWorkspace

	normalized, err := normalizeSpec(spec)
	if err != nil {
		return SessionSpec{}, err
	}
	if err := m.validateWorkspacePolicy(normalized.Workspace); err != nil {
		return SessionSpec{}, err
	}
	return normalized, nil
}

func normalizeSpec(spec SessionSpec) (SessionSpec, error) {
	spec.Owner = strings.TrimSpace(spec.Owner)
	spec.AgentKind = strings.TrimSpace(strings.ToLower(spec.AgentKind))
	spec.Workspace = strings.TrimSpace(spec.Workspace)
	spec.Tool = strings.TrimSpace(spec.Tool)
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Command = strings.TrimSpace(spec.Command)

	if spec.Owner == "" {
		return SessionSpec{}, fmt.Errorf("owner is required")
	}
	if spec.AgentKind == "" {
		return SessionSpec{}, fmt.Errorf("agent_kind is required")
	}
	if spec.Workspace == "" {
		return SessionSpec{}, fmt.Errorf("workspace is required")
	}
	spec.Workspace = filepath.Clean(spec.Workspace)
	if !filepath.IsAbs(spec.Workspace) {
		return SessionSpec{}, fmt.Errorf("absolute workspace path is required")
	}
	adapter, ok := NewRegistry().Get(spec.AgentKind)
	if !ok {
		return SessionSpec{}, fmt.Errorf("unsupported agent_kind: %s", spec.AgentKind)
	}
	launcher := launcherSpec{tool: adapter.Tool(), command: adapter.Command()}
	if spec.Tool == "" {
		spec.Tool = launcher.tool
	} else if spec.Tool != launcher.tool {
		return SessionSpec{}, fmt.Errorf("tool must match agent_kind launcher: %s", launcher.tool)
	}
	if spec.Command == "" {
		spec.Command = launcher.command
	} else if spec.Command != launcher.command {
		return SessionSpec{}, fmt.Errorf("command must launch the selected tool")
	}
	if spec.Title == "" {
		spec.Title = strings.Title(spec.AgentKind) + " Session"
	}
	return spec, nil
}

func metadataString(values map[string]interface{}, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func (m *Manager) resolveWorkspace(raw string) (string, error) {
	workspace := strings.TrimSpace(raw)
	root := ""
	if m != nil && m.cfg != nil {
		root = filepath.Clean(strings.TrimSpace(m.cfg.WorkspacePath()))
	}

	if workspace == "" {
		if root == "" {
			return "", fmt.Errorf("workspace is required")
		}
		workspace = root
	} else if !filepath.IsAbs(workspace) {
		if root == "" || !filepath.IsAbs(root) {
			return "", fmt.Errorf("configured workspace root is invalid")
		}
		workspace = filepath.Join(root, workspace)
	}

	workspace = filepath.Clean(workspace)
	if !filepath.IsAbs(workspace) {
		return "", fmt.Errorf("absolute workspace path is required")
	}
	return workspace, nil
}

func (m *Manager) validateWorkspacePolicy(workspace string) error {
	if m == nil || m.cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !m.cfg.Agents.Defaults.RestrictToWorkspace {
		return nil
	}

	root := filepath.Clean(strings.TrimSpace(m.cfg.WorkspacePath()))
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("configured workspace root is invalid")
	}
	if workspace == root {
		return nil
	}
	rel, err := filepath.Rel(root, workspace)
	if err != nil {
		return fmt.Errorf("workspace must stay within configured workspace")
	}
	rel = filepath.Clean(strings.TrimSpace(rel))
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("workspace must stay within configured workspace")
	}
	return nil
}
