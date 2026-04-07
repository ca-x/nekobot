package externalagent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"nekobot/pkg/toolsessions"
)

const (
	metadataAgentKind = "external_agent_kind"
	metadataWorkspace = "external_agent_workspace"
)

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
	sessions *toolsessions.Manager
}

// NewManager creates an external-agent session manager.
func NewManager(sessions *toolsessions.Manager) (*Manager, error) {
	if sessions == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return &Manager{sessions: sessions}, nil
}

// ResolveSession finds an existing reusable session or creates a new one.
func (m *Manager) ResolveSession(ctx context.Context, spec SessionSpec) (*toolsessions.Session, bool, error) {
	if m == nil || m.sessions == nil {
		return nil, false, fmt.Errorf("tool session manager is required")
	}

	normalized, err := normalizeSpec(spec)
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
		if metadataString(item.Metadata, metadataAgentKind) == normalized.AgentKind &&
			metadataString(item.Metadata, metadataWorkspace) == normalized.Workspace {
			return item, false, nil
		}
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
		},
	})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
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
	if spec.Tool == "" {
		spec.Tool = spec.AgentKind
	}
	if spec.Command == "" {
		spec.Command = spec.Tool
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
