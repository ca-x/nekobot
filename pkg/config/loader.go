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

const ConfigPathEnv = "NEKOBOT_CONFIG_FILE"

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()

	// Set default config name and paths
	v.SetConfigName("config")
	v.SetConfigType("json")

	// Add default config paths
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".nekobot"))
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
// If the file doesn't exist, it auto-creates one.
func (l *Loader) Load(configPath string) (*Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// Allow global override from environment.
	if strings.TrimSpace(configPath) == "" {
		configPath = strings.TrimSpace(os.Getenv(ConfigPathEnv))
	}
	explicitPath := strings.TrimSpace(configPath) != ""
	resolvedPath, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, err
	}
	if explicitPath {
		// Keep -c config colocated with its workspace by default.
		cfg.Agents.Defaults.Workspace = defaultWorkspaceForConfigPath(resolvedPath)
		cfg.Storage.DBDir = filepath.Dir(resolvedPath)
	}

	// If specific path is provided, use it
	if explicitPath {
		l.viper.SetConfigFile(resolvedPath)
	}

	// Try to read config file
	if err := l.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			if strings.TrimSpace(cfg.Storage.DBDir) == "" {
				cfg.Storage.DBDir = filepath.Dir(resolvedPath)
			}
			if err := SaveToFile(cfg, resolvedPath); err != nil {
				return nil, fmt.Errorf("creating config file: %w", err)
			}
			if _, err := EnsureRuntimeDBFile(cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Unmarshal into config struct
	if err := l.viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Backward compatibility: migrate tools.web.search.api_key -> brave_api_key in-memory.
	if cfg.Tools.Web.Search.BraveAPIKey == "" {
		cfg.Tools.Web.Search.BraveAPIKey = cfg.Tools.Web.Search.LegacyAPIKey
	}
	if !explicitPath {
		used := strings.TrimSpace(l.viper.ConfigFileUsed())
		if used != "" {
			resolvedPath = used
		}
	} else {
		desiredWorkspace := defaultWorkspaceForConfigPath(resolvedPath)
		desiredDBDir := filepath.Dir(resolvedPath)
		changed := false
		if strings.TrimSpace(cfg.Agents.Defaults.Workspace) != desiredWorkspace {
			cfg.Agents.Defaults.Workspace = desiredWorkspace
			changed = true
		}
		if strings.TrimSpace(cfg.Storage.DBDir) != desiredDBDir {
			cfg.Storage.DBDir = desiredDBDir
			changed = true
		}
		if changed {
			if err := SaveToFile(cfg, resolvedPath); err != nil {
				return nil, fmt.Errorf("updating workspace in config file: %w", err)
			}
		}
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
	v.Set("storage", cfg.Storage)
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
	resolvedPath, err := resolveConfigPath(strings.TrimSpace(os.Getenv(ConfigPathEnv)))
	if err != nil {
		return "", false, err
	}
	configPath = resolvedPath
	explicitPath := strings.TrimSpace(os.Getenv(ConfigPathEnv)) != ""

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		if explicitPath {
			loader := NewLoader()
			cfg, loadErr := loader.LoadFromFile(configPath)
			if loadErr == nil {
				desiredWorkspace := defaultWorkspaceForConfigPath(configPath)
				desiredDBDir := filepath.Dir(configPath)
				changed := false
				if strings.TrimSpace(cfg.Agents.Defaults.Workspace) != desiredWorkspace {
					cfg.Agents.Defaults.Workspace = desiredWorkspace
					changed = true
				}
				if strings.TrimSpace(cfg.Storage.DBDir) != desiredDBDir {
					cfg.Storage.DBDir = desiredDBDir
					changed = true
				}
				if changed {
					if saveErr := SaveToFile(cfg, configPath); saveErr != nil {
						return "", false, fmt.Errorf("updating workspace in config file: %w", saveErr)
					}
				}
			}
		}
		return configPath, false, nil // File exists, not created
	}

	cfg := DefaultConfig()
	cfg.Storage.DBDir = filepath.Dir(configPath)
	if explicitPath {
		cfg.Agents.Defaults.Workspace = defaultWorkspaceForConfigPath(configPath)
	}
	if err := SaveToFile(cfg, configPath); err != nil {
		return "", false, fmt.Errorf("writing default config: %w", err)
	}
	if _, err := EnsureRuntimeDBFile(cfg); err != nil {
		return "", false, err
	}

	return configPath, true, nil
}

func resolveConfigPath(configPath string) (string, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		home, err := GetConfigHome()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, "config.json")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return abs, nil
}

func defaultWorkspaceForConfigPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "workspace")
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
