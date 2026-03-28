package servicecontrol

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/service"

	"nekobot/pkg/config"
)

// GatewayServiceStatus describes the installed gateway service state.
type GatewayServiceStatus struct {
	Name       string   `json:"name"`
	Platform   string   `json:"platform"`
	ConfigPath string   `json:"config_path"`
	Arguments  []string `json:"arguments"`
	Installed  bool     `json:"installed"`
	Status     string   `json:"status"`
}

var newService = service.New

type noopProgram struct{}

func (noopProgram) Start(service.Service) error { return nil }
func (noopProgram) Stop(service.Service) error  { return nil }

// ServiceConfig returns the gateway service manager configuration.
func ServiceConfig(configPath string) *service.Config {
	args := []string{"gateway", "run"}
	configFile := cmp.Or(strings.TrimSpace(configPath), strings.TrimSpace(os.Getenv(config.ConfigPathEnv)))
	if configFile != "" {
		if absPath, err := filepath.Abs(configFile); err == nil {
			configFile = absPath
		}
		args = append([]string{"-c", configFile}, args...)
	}

	return &service.Config{
		Name:        "nekobot-gateway",
		DisplayName: "Nekobot Gateway",
		Description: "Nekobot AI assistant gateway for multi-channel support",
		Arguments:   args,
	}
}

// RestartGatewayService restarts the installed gateway service.
func RestartGatewayService(configPath string) error {
	svc, err := buildService(configPath)
	if err != nil {
		return err
	}
	if err := svc.Restart(); err != nil {
		return fmt.Errorf("restarting service: %w", err)
	}
	return nil
}

// InspectGatewayService returns the current installation and run status.
func InspectGatewayService(configPath string) (*GatewayServiceStatus, error) {
	svcConfig := ServiceConfig(configPath)
	svc, err := newService(noopProgram{}, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}

	rawStatus, err := svc.Status()
	if err != nil {
		return &GatewayServiceStatus{
			Name:       svcConfig.Name,
			Platform:   service.Platform(),
			ConfigPath: resolvedConfigPath(svcConfig.Arguments),
			Arguments:  append([]string(nil), svcConfig.Arguments...),
			Installed:  false,
			Status:     "not_installed",
		}, nil
	}

	return &GatewayServiceStatus{
		Name:       svcConfig.Name,
		Platform:   service.Platform(),
		ConfigPath: resolvedConfigPath(svcConfig.Arguments),
		Arguments:  append([]string(nil), svcConfig.Arguments...),
		Installed:  true,
		Status:     normalizeStatus(rawStatus),
	}, nil
}

func buildService(configPath string) (service.Service, error) {
	svcConfig := ServiceConfig(configPath)
	svc, err := newService(noopProgram{}, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}
	return svc, nil
}

func normalizeStatus(status service.Status) string {
	switch status {
	case service.StatusRunning:
		return "running"
	case service.StatusStopped:
		return "stopped"
	case service.StatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

func resolvedConfigPath(args []string) string {
	if len(args) < 2 {
		return ""
	}
	for i := range len(args) - 1 {
		if args[i] == "-c" {
			return args[i+1]
		}
	}
	return ""
}
