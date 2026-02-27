package config

import "testing"

func TestValidateConfigRejectsInvalidOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Orchestrator = "unknown"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for orchestrator")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	for _, validationErr := range validationErrors {
		if validationErr.Field == "agents.defaults.orchestrator" {
			return
		}
	}
	t.Fatalf("expected orchestrator validation error, got %v", err)
}

func TestValidateConfigRejectsInvalidMemorySettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Enabled = true
	cfg.Memory.Semantic.Enabled = true
	cfg.Memory.Semantic.DefaultTopK = 9
	cfg.Memory.Semantic.MaxTopK = 4
	cfg.Memory.Semantic.SearchPolicy = "invalid"
	cfg.Memory.Episodic.Enabled = true
	cfg.Memory.Episodic.SummaryWindowMessages = 0
	cfg.Memory.ShortTerm.Enabled = true
	cfg.Memory.ShortTerm.RawHistoryLimit = 0

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for memory settings")
	}

	validationErrors, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	requiredFields := map[string]bool{
		"memory.semantic.default_top_k":           false,
		"memory.semantic.search_policy":           false,
		"memory.episodic.summary_window_messages": false,
		"memory.short_term.raw_history_limit":     false,
	}

	for _, validationErr := range validationErrors {
		if _, ok := requiredFields[validationErr.Field]; ok {
			requiredFields[validationErr.Field] = true
		}
	}

	for field, found := range requiredFields {
		if !found {
			t.Fatalf("expected validation error for %s, got %v", field, err)
		}
	}
}
