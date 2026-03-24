package webui

import (
	"testing"

	"nekobot/pkg/config"
)

func TestPersistChatRoutingPersistsModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Model = "old-model"
	cfg.Agents.Defaults.Fallback = []string{"openai"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
		{Name: "openai", ProviderKind: "openai"},
	}

	s := &Server{config: cfg}
	if err := s.persistChatRouting("anthropic", "new-model", []string{"openai"}); err != nil {
		t.Fatalf("persistChatRouting failed: %v", err)
	}

	if cfg.Agents.Defaults.Model != "new-model" {
		t.Fatalf("expected model to persist, got %q", cfg.Agents.Defaults.Model)
	}
}
