package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Loader handles configuration loading with Viper.
type Loader struct {
	viper *viper.Viper
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()

	// Set default config name and paths
	v.SetConfigName("config")
	v.SetConfigType("json")

	// Add default config paths
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".nanobot"))
	}
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Environment variable settings
	v.SetEnvPrefix("NEKOBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &Loader{viper: v}
}

// Load loads the configuration from file and environment variables.
// If configPath is empty, it will search default paths.
// If the file doesn't exist, it returns the default configuration.
func (l *Loader) Load(configPath string) (*Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// If specific path is provided, use it
	if configPath != "" {
		l.viper.SetConfigFile(configPath)
	}

	// Try to read config file
	if err := l.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Unmarshal into config struct
	if err := l.viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a specific file.
func (l *Loader) LoadFromFile(path string) (*Config, error) {
	return l.Load(path)
}

// Save saves the configuration to a file.
func (l *Loader) Save(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Determine format from extension
	ext := filepath.Ext(path)
	format := "json"
	switch ext {
	case ".yaml", ".yml":
		format = "yaml"
	case ".toml":
		format = "toml"
	case ".json":
		format = "json"
	}

	// Create a new viper instance for writing
	v := viper.New()
	v.SetConfigType(format)

	// Set all values from config
	v.Set("agents", cfg.Agents)
	v.Set("channels", cfg.Channels)
	v.Set("providers", cfg.Providers)
	v.Set("transcription", cfg.Transcription)
	v.Set("gateway", cfg.Gateway)
	v.Set("tools", cfg.Tools)
	v.Set("heartbeat", cfg.Heartbeat)
	v.Set("redis", cfg.Redis)
	v.Set("state", cfg.State)
	v.Set("bus", cfg.Bus)
	v.Set("memory", cfg.Memory)
	v.Set("approval", cfg.Approval)
	v.Set("webui", cfg.WebUI)

	// Write to file
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// SaveToFile is a convenience function to save config without creating a Loader.
func SaveToFile(cfg *Config, path string) error {
	loader := NewLoader()
	return loader.Save(path, cfg)
}

// GetConfigHome returns the default config directory.
func GetConfigHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".nekobot"), nil
}

// GetConfigPath returns the path of the loaded config file.
func (l *Loader) GetConfigPath() string {
	return l.viper.ConfigFileUsed()
}

// Set sets a configuration value.
func (l *Loader) Set(key string, value interface{}) {
	l.viper.Set(key, value)
}

// Get gets a configuration value.
func (l *Loader) Get(key string) interface{} {
	return l.viper.Get(key)
}

// GetString gets a string configuration value.
func (l *Loader) GetString(key string) string {
	return l.viper.GetString(key)
}

// GetInt gets an integer configuration value.
func (l *Loader) GetInt(key string) int {
	return l.viper.GetInt(key)
}

// GetBool gets a boolean configuration value.
func (l *Loader) GetBool(key string) bool {
	return l.viper.GetBool(key)
}

// IsSet checks if a key is set in the configuration.
func (l *Loader) IsSet(key string) bool {
	return l.viper.IsSet(key)
}

// InitDefaultConfig creates a default config file if it doesn't exist.
// Returns the path to the config file and whether it was newly created.
func InitDefaultConfig() (configPath string, created bool, err error) {
	home, err := GetConfigHome()
	if err != nil {
		return "", false, err
	}

	configPath = filepath.Join(home, "config.json")

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, false, nil // File exists, not created
	}

	// Create config directory
	if err := os.MkdirAll(home, 0755); err != nil {
		return "", false, fmt.Errorf("creating config directory: %w", err)
	}

	// Write default config template to file
	if err := os.WriteFile(configPath, []byte(DefaultConfigTemplate), 0644); err != nil {
		return "", false, fmt.Errorf("writing default config: %w", err)
	}

	return configPath, true, nil
}

// EnsureWorkspace creates the workspace directory if it doesn't exist.
func (cfg *Config) EnsureWorkspace() error {
	workspace := cfg.WorkspacePath()
	if workspace == "" {
		return fmt.Errorf("workspace path not set")
	}

	// Create workspace directory
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"sessions", "state", "memory"}
	for _, subdir := range subdirs {
		path := filepath.Join(workspace, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating %s directory: %w", subdir, err)
		}
	}

	return nil
}
