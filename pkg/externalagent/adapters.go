package externalagent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Adapter describes one supported external coding agent.
type Adapter interface {
	Kind() string
	Tool() string
	Command() string
	ConfigDir(homeDir string) string
	SupportsAutoInstall() bool
	InstallCommand(osName string) []string
}

type staticAdapter struct {
	kind               string
	tool               string
	command            string
	configDirResolver  func(string) string
	autoInstall        bool
	installCommandFunc func(string) []string
}

func (a staticAdapter) Kind() string              { return a.kind }
func (a staticAdapter) Tool() string              { return a.tool }
func (a staticAdapter) Command() string           { return a.command }
func (a staticAdapter) SupportsAutoInstall() bool { return a.autoInstall }
func (a staticAdapter) InstallCommand(osName string) []string {
	return append([]string(nil), a.installCommandFunc(osName)...)
}
func (a staticAdapter) ConfigDir(homeDir string) string {
	if a.configDirResolver == nil {
		return ""
	}
	return strings.TrimSpace(a.configDirResolver(homeDir))
}

func defaultAdapters() []Adapter {
	return []Adapter{
		staticAdapter{
			kind:              "codex",
			tool:              "codex",
			command:           "codex",
			configDirResolver: func(home string) string { return filepath.Join(home, ".codex") },
			autoInstall:       true,
			installCommandFunc: func(osName string) []string {
				return []string{"npm", "install", "-g", "@openai/codex"}
			},
		},
		staticAdapter{
			kind:              "claude",
			tool:              "claude",
			command:           "claude",
			configDirResolver: func(home string) string { return filepath.Join(home, ".claude") },
			autoInstall:       true,
			installCommandFunc: func(osName string) []string {
				return []string{"npm", "install", "-g", "@anthropic-ai/claude-code"}
			},
		},
		staticAdapter{
			kind:              "opencode",
			tool:              "opencode",
			command:           "opencode",
			configDirResolver: func(home string) string { return filepath.Join(home, ".config", "opencode") },
			autoInstall:       true,
			installCommandFunc: func(osName string) []string {
				if strings.EqualFold(strings.TrimSpace(osName), "darwin") {
					return []string{"brew", "install", "anomalyco/tap/opencode"}
				}
				return []string{"npm", "install", "-g", "opencode-ai"}
			},
		},
		staticAdapter{
			kind:              "aider",
			tool:              "aider",
			command:           "aider",
			configDirResolver: func(home string) string { return filepath.Join(home, ".aider") },
			autoInstall:       false,
			installCommandFunc: func(osName string) []string {
				return nil
			},
		},
	}
}

// Registry provides adapter lookup.
type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	items := make(map[string]Adapter)
	for _, adapter := range defaultAdapters() {
		items[adapter.Kind()] = adapter
	}
	return &Registry{adapters: items}
}

func (r *Registry) Get(kind string) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[strings.ToLower(strings.TrimSpace(kind))]
	return adapter, ok
}

func (r *Registry) List() []Adapter {
	if r == nil {
		return nil
	}
	kinds := make([]string, 0, len(r.adapters))
	for kind := range r.adapters {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	out := make([]Adapter, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, r.adapters[kind])
	}
	return out
}

// InstalledAgent describes a detected local external agent installation.
type InstalledAgent struct {
	Kind      string `json:"kind"`
	Tool      string `json:"tool"`
	ConfigDir string `json:"config_dir"`
}

// DetectInstalled returns supported agents whose config roots exist.
func DetectInstalled(homeDir string) ([]InstalledAgent, error) {
	trimmedHome := strings.TrimSpace(homeDir)
	if trimmedHome == "" {
		resolved, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		trimmedHome = resolved
	}
	registry := NewRegistry()
	var out []InstalledAgent
	for _, adapter := range registry.List() {
		configDir := strings.TrimSpace(adapter.ConfigDir(trimmedHome))
		if configDir == "" {
			continue
		}
		info, err := os.Stat(configDir)
		if err != nil || !info.IsDir() {
			continue
		}
		out = append(out, InstalledAgent{
			Kind:      adapter.Kind(),
			Tool:      adapter.Tool(),
			ConfigDir: configDir,
		})
	}
	return out, nil
}

// InstallHint returns the best-effort install command hint for one adapter kind.
func InstallHint(kind, osName string) ([]string, error) {
	adapter, ok := NewRegistry().Get(kind)
	if !ok {
		return nil, fmt.Errorf("unsupported external agent: %s", kind)
	}
	if !adapter.SupportsAutoInstall() {
		return nil, fmt.Errorf("external agent %s does not advertise auto install", kind)
	}
	cmd := adapter.InstallCommand(osName)
	if len(cmd) == 0 {
		return nil, fmt.Errorf("external agent %s has no install hint for %s", kind, osName)
	}
	return cmd, nil
}
