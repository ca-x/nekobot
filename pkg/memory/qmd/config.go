// Package qmd provides configuration conversion helpers.
package qmd

import (
	"os"
	"path/filepath"
	"strings"

	"nekobot/pkg/config"
)

// ConfigFromConfig converts pkg/config QMD config to qmd.Config.
func ConfigFromConfig(cfg config.QMDConfig) Config {
	// Convert paths
	paths := make([]CollectionPath, len(cfg.Paths))
	for i, p := range cfg.Paths {
		paths[i] = CollectionPath{
			Name:    p.Name,
			Path:    p.Path,
			Pattern: p.Pattern,
		}
	}

	return Config{
		Enabled:        cfg.Enabled,
		Command:        cfg.Command,
		IncludeDefault: cfg.IncludeDefault,
		Paths:          paths,
		Sessions: SessionsConfig{
			Enabled:       cfg.Sessions.Enabled,
			SessionsDir:   "",
			ExportDir:     cfg.Sessions.ExportDir,
			RetentionDays: cfg.Sessions.RetentionDays,
		},
		Update: UpdateConfig{
			OnBoot:         cfg.Update.OnBoot,
			Interval:       cfg.Update.Interval,
			CommandTimeout: cfg.Update.CommandTimeout,
			UpdateTimeout:  cfg.Update.UpdateTimeout,
		},
	}
}

// ConfigFromConfigWithWorkspace resolves workspace-aware QMD paths.
func ConfigFromConfigWithWorkspace(cfg config.QMDConfig, workspaceDir string) Config {
	resolved := ConfigFromConfig(cfg)

	resolved.Paths = make([]CollectionPath, 0, len(cfg.Paths))
	for _, pathCfg := range cfg.Paths {
		resolved.Paths = append(resolved.Paths, CollectionPath{
			Name:    pathCfg.Name,
			Path:    resolveWorkspacePath(pathCfg.Path, workspaceDir),
			Pattern: pathCfg.Pattern,
		})
	}

	resolved.Sessions.SessionsDir = filepath.Join(workspaceDir, "sessions")
	if strings.TrimSpace(resolved.Sessions.ExportDir) == "" {
		resolved.Sessions.ExportDir = filepath.Join(workspaceDir, "memory", "sessions")
	} else {
		resolved.Sessions.ExportDir = resolveWorkspacePath(resolved.Sessions.ExportDir, workspaceDir)
	}

	return resolved
}

func resolveWorkspacePath(path string, workspaceDir string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	if workspace := strings.TrimSpace(workspaceDir); workspace != "" {
		replacer := strings.NewReplacer(
			"${WORKSPACE}", workspace,
			"$WORKSPACE", workspace,
		)
		path = replacer.Replace(path)
	}

	path = os.ExpandEnv(path)
	return expandHome(path)
}
