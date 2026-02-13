package memory

import (
	"path/filepath"

	"go.uber.org/fx"
	"nekobot/pkg/config"
)

// Module provides memory system for fx dependency injection.
var Module = fx.Module("memory",
	fx.Provide(
		NewManagerFromConfig,
	),
)

// NewManagerFromConfig creates a memory manager from configuration.
func NewManagerFromConfig(cfg *config.Config) (*Manager, error) {
	// Determine storage path
	workspace := cfg.WorkspacePath()
	storePath := filepath.Join(workspace, "memory", "embeddings.json")

	// Create embedding provider
	var provider EmbeddingProvider

	// Check if OpenAI API key is available
	if openaiCfg := cfg.GetProviderConfig("openai"); openaiCfg != nil && openaiCfg.APIKey != "" {
		provider = NewOpenAIEmbeddingProvider(openaiCfg.APIKey, "text-embedding-3-small")
	} else {
		// Fallback to simple provider
		provider = NewSimpleEmbeddingProvider(384)
	}

	return NewManager(storePath, provider)
}
