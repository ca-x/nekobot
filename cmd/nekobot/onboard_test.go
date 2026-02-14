package main

import "testing"

func TestOnboardDefaultConfig_UsesProvidedWorkspace(t *testing.T) {
	workspace := "/tmp/custom-workspace"

	cfg := onboardDefaultConfig(workspace)

	if cfg.Agents.Defaults.Workspace != workspace {
		t.Fatalf("expected workspace %q, got %q", workspace, cfg.Agents.Defaults.Workspace)
	}
}
