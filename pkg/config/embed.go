// Package config provides embedded resources for the nanobot configuration.
package config

import _ "embed"

// DefaultConfigTemplate contains the default configuration template.
//go:embed config.example.json
var DefaultConfigTemplate string
