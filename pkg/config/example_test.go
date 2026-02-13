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
	// Create a loader
	loader := config.NewLoader()

	// Load config (will use defaults if file doesn't exist)
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
	// Create config directory
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "nanobot-test-config.json")
	defer os.Remove(configPath)

	// Create a config
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4"
	cfg.Providers.OpenAI.APIKey = "sk-test-key"

	// Save it
	loader := config.NewLoader()
	if err := loader.Save(configPath, cfg); err != nil {
		log.Fatal(err)
	}

	// Load it back
	loadedCfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model: %s\n", loadedCfg.Agents.Defaults.Model)
	fmt.Printf("API Key set: %v\n", loadedCfg.Providers.OpenAI.APIKey != "")
}

// Example_validation demonstrates configuration validation.
func Example_validation() {
	cfg := config.DefaultConfig()

	// Introduce some invalid values
	cfg.Agents.Defaults.Temperature = 3.0 // Invalid: must be 0-2
	cfg.Gateway.Port = 99999               // Invalid: must be 1-65535
	cfg.Heartbeat.Interval = 1             // Invalid: must be >= 5

	// Validate
	if err := config.ValidateConfig(cfg); err != nil {
		fmt.Println("Validation failed:")
		fmt.Println(err)
	}
}

// Example_hotReload demonstrates configuration hot-reload.
func Example_hotReload() {
	// Create a config file
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "nanobot-watch-test.json")
	defer os.Remove(configPath)

	// Initial config
	cfg := config.DefaultConfig()
	loader := config.NewLoader()
	if err := loader.Save(configPath, cfg); err != nil {
		log.Fatal(err)
	}

	// Load and start watching
	loadedCfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create watcher
	watcher := config.NewWatcher(loader, loadedCfg)

	// Add handler
	watcher.AddHandler(func(newCfg *config.Config) error {
		fmt.Printf("Config changed! New model: %s\n", newCfg.Agents.Defaults.Model)
		return nil
	})

	// Start watching
	if err := watcher.Start(); err != nil {
		log.Fatal(err)
	}

	// In a real application, the watcher runs in the background
	// and automatically reloads when the file changes
	defer watcher.Stop()

	fmt.Println("Watching for config changes...")
}

// Example_providerConfig demonstrates getting provider-specific configuration.
func Example_providerConfig() {
	cfg := config.DefaultConfig()

	// Set some provider configs
	cfg.Providers.OpenAI.APIKey = "sk-openai-key"
	cfg.Providers.OpenAI.APIBase = "https://api.openai.com/v1"

	cfg.Providers.Anthropic.APIKey = "sk-claude-key"
	cfg.Providers.Anthropic.APIBase = "https://api.anthropic.com/v1"

	// Get provider config by name
	openaiCfg := cfg.GetProviderConfig("openai")
	claudeCfg := cfg.GetProviderConfig("claude")

	fmt.Printf("OpenAI Base: %s\n", openaiCfg.APIBase)
	fmt.Printf("Claude Base: %s\n", claudeCfg.APIBase)
}
