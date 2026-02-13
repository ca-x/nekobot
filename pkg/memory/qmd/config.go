// Package qmd provides configuration conversion helpers.
package qmd

import (
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
