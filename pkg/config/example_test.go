package config_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"nekobot/pkg/config"
)

// Example_basicUsage demonstrates basic configuration loading.
func Example_basicUsage() {
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Workspace: %s\n", cfg.Agents.Defaults.Workspace)
	fmt.Printf("Model: %s\n", cfg.Agents.Defaults.Model)
	fmt.Printf("Temperature: %.1f\n", cfg.Agents.Defaults.Temperature)
}

// Example_saveAndLoad demonstrates saving and loading configuration.
func Example_saveAndLoad() {
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "nanobot-test-config.json")
	defer os.Remove(configPath)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o-mini"
	cfg.Providers = append(cfg.Providers, config.ProviderProfile{
		Name:         "openai",
		ProviderKind: "openai",
		APIKey:       "sk-test-key",
		APIBase:      "https://api.openai.com/v1",
	})

	loader := config.NewLoader()
	if err := loader.Save(configPath, cfg); err != nil {
		log.Fatal(err)
	}

	loadedCfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	openaiCfg := loadedCfg.GetProviderConfig("openai")
	fmt.Printf("Model: %s\n", loadedCfg.Agents.Defaults.Model)
	fmt.Printf("API Key set: %v\n", openaiCfg != nil && openaiCfg.APIKey != "")
}

// Example_validation demonstrates configuration validation.
func Example_validation() {
	cfg := config.DefaultConfig()
	cfg.Providers = append(cfg.Providers, config.ProviderProfile{
		Name:         "demo-openai",
		ProviderKind: "openai",
		APIKey:       "test",
		APIBase:      "https://api.openai.com/v1",
	})

	// Introduce some invalid values
	cfg.Agents.Defaults.Temperature = 3.0 // Invalid: must be 0-2
	cfg.Gateway.Port = 99999              // Invalid: must be 1-65535
	cfg.Heartbeat.IntervalMinutes = 1     // Invalid: must be >= 5

	if err := config.ValidateConfig(cfg); err != nil {
		fmt.Println("Validation failed:")
		fmt.Println(err)
	}
}

// Example_providerConfig demonstrates getting provider-specific configuration.
func Example_providerConfig() {
	cfg := config.DefaultConfig()
	cfg.Providers = append(cfg.Providers,
		config.ProviderProfile{
			Name:         "openai",
			ProviderKind: "openai",
			APIKey:       "sk-openai-key",
			APIBase:      "https://api.openai.com/v1",
		},
		config.ProviderProfile{
			Name:         "anthropic",
			ProviderKind: "anthropic",
			APIKey:       "sk-claude-key",
			APIBase:      "https://api.anthropic.com/v1",
		},
	)

	openaiCfg := cfg.GetProviderConfig("openai")
	anthropicCfg := cfg.GetProviderConfig("anthropic")

	fmt.Printf("OpenAI Base: %s\n", openaiCfg.APIBase)
	fmt.Printf("Anthropic Base: %s\n", anthropicCfg.APIBase)
}
