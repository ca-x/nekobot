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

// ManagedServiceStatus describes the installed service state.
type ManagedServiceStatus struct {
	Name       string   `json:"name"`
	Platform   string   `json:"platform"`
	ConfigPath string   `json:"config_path"`
	Arguments  []string `json:"arguments"`
	Installed  bool     `json:"installed"`
	Status     string   `json:"status"`
}

type ServiceSpec struct {
	Name        string
	DisplayName string
	Description string
	RunArgs     []string
}

var (
	GatewayServiceSpec = ServiceSpec{
		Name:        "nekobot-gateway",
		DisplayName: "Nekobot Gateway",
		Description: "Nekobot AI assistant gateway for multi-channel support",
		RunArgs:     []string{"gateway", "run"},
	}
	NekoClientdServiceSpec = ServiceSpec{
		Name:        "nekoclientd",
		DisplayName: "NekoClientd",
		Description: "Nekobot remote agent daemon client",
		RunArgs:     []string{"run"},
	}
)

var newService = service.New

type noopProgram struct{}

func (noopProgram) Start(service.Service) error { return nil }
func (noopProgram) Stop(service.Service) error  { return nil }

// ServiceConfig returns the managed service manager configuration.
func ServiceConfig(configPath string, spec ServiceSpec) *service.Config {
	args := append([]string(nil), spec.RunArgs...)
	configFile := cmp.Or(strings.TrimSpace(configPath), strings.TrimSpace(os.Getenv(config.ConfigPathEnv)))
	if configFile != "" {
		if absPath, err := filepath.Abs(configFile); err == nil {
			configFile = absPath
		}
		args = append([]string{"-c", configFile}, args...)
	}
	return &service.Config{
		Name:        spec.Name,
		DisplayName: spec.DisplayName,
		Description: spec.Description,
		Arguments:   args,
	}
}

func GatewayConfig(configPath string) *service.Config {
	return ServiceConfig(configPath, GatewayServiceSpec)
}
func NekoClientdConfig(configPath string) *service.Config {
	return ServiceConfig(configPath, NekoClientdServiceSpec)
}

func RestartGatewayService(configPath string) error {
	return restartService(configPath, GatewayServiceSpec)
}
func RestartNekoClientdService(configPath string) error {
	return restartService(configPath, NekoClientdServiceSpec)
}

func restartService(configPath string, spec ServiceSpec) error {
	svc, err := buildService(configPath, spec)
	if err != nil {
		return err
	}
	if err := svc.Restart(); err != nil {
		return fmt.Errorf("restarting service: %w", err)
	}
	return nil
}

func InspectGatewayService(configPath string) (*ManagedServiceStatus, error) {
	return InspectService(configPath, GatewayServiceSpec)
}
func InspectNekoClientdService(configPath string) (*ManagedServiceStatus, error) {
	return InspectService(configPath, NekoClientdServiceSpec)
}

// InspectService returns the current installation and run status.
func InspectService(configPath string, spec ServiceSpec) (*ManagedServiceStatus, error) {
	svcConfig := ServiceConfig(configPath, spec)
	svc, err := newService(noopProgram{}, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}
	rawStatus, err := svc.Status()
	if err != nil {
		return &ManagedServiceStatus{Name: svcConfig.Name, Platform: service.Platform(), ConfigPath: resolvedConfigPath(svcConfig.Arguments), Arguments: append([]string(nil), svcConfig.Arguments...), Installed: false, Status: "not_installed"}, nil
	}
	return &ManagedServiceStatus{Name: svcConfig.Name, Platform: service.Platform(), ConfigPath: resolvedConfigPath(svcConfig.Arguments), Arguments: append([]string(nil), svcConfig.Arguments...), Installed: true, Status: normalizeStatus(rawStatus)}, nil
}

func buildService(configPath string, spec ServiceSpec) (service.Service, error) {
	svcConfig := ServiceConfig(configPath, spec)
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
