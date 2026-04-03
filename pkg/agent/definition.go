package agent

import (
	"strings"

	"nekobot/pkg/approval"
	"nekobot/pkg/config"
)

type DefinitionRoute struct {
	Provider string
	Model    string
	Fallback []string
}

type DefinitionToolPolicy struct {
	Allowlist []string
	Denylist  []string
}

type DefinitionPromptBoundary struct {
	Static  []string
	Dynamic []string
}

// AgentDefinition is the compatibility bridge between the current runtime config
// and the future definition-driven agent execution model.
type AgentDefinition struct {
	ID                string
	Orchestrator      string
	Route             DefinitionRoute
	PermissionMode    approval.Mode
	ToolPolicy        DefinitionToolPolicy
	MaxToolIterations int
	PromptSections    DefinitionPromptBoundary
}

// AgentDefinitionFromRuntimeConfig builds the current main-agent definition from
// existing runtime config, preserving today's behavior while making it inspectable.
func AgentDefinitionFromRuntimeConfig(cfg *config.Config) AgentDefinition {
	if cfg == nil {
		return AgentDefinition{
			ID:             "main",
			PromptSections: defaultPromptBoundary(),
		}
	}

	mode := approval.Mode(strings.TrimSpace(cfg.Approval.Mode))
	switch mode {
	case approval.ModeAuto, approval.ModePrompt, approval.ModeManual:
	default:
		mode = approval.ModeAuto
	}

	return AgentDefinition{
		ID:           "main",
		Orchestrator: strings.TrimSpace(cfg.Agents.Defaults.Orchestrator),
		Route: DefinitionRoute{
			Provider: strings.TrimSpace(cfg.Agents.Defaults.Provider),
			Model:    strings.TrimSpace(cfg.Agents.Defaults.Model),
			Fallback: append([]string(nil), cfg.Agents.Defaults.Fallback...),
		},
		PermissionMode: mode,
		ToolPolicy: DefinitionToolPolicy{
			Allowlist: append([]string(nil), cfg.Approval.Allowlist...),
			Denylist:  append([]string(nil), cfg.Approval.Denylist...),
		},
		MaxToolIterations: cfg.Agents.Defaults.MaxToolIterations,
		PromptSections:    defaultPromptBoundary(),
	}
}

func defaultPromptBoundary() DefinitionPromptBoundary {
	return DefinitionPromptBoundary{
		Static:  []string{"identity", "bootstrap"},
		Dynamic: []string{"skills", "memory", "managed_prompts"},
	}
}
