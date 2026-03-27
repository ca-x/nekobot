package servicecontrol

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kardianos/service"

	"nekobot/pkg/config"
)

type stubService struct {
	status       service.Status
	statusErr    error
	restartErr   error
	restartCalls int
}

func (s *stubService) Run() error                    { return nil }
func (s *stubService) Start() error                  { return nil }
func (s *stubService) Stop() error                   { return nil }
func (s *stubService) Restart() error                { s.restartCalls++; return s.restartErr }
func (s *stubService) Install() error                { return nil }
func (s *stubService) Uninstall() error              { return nil }
func (s *stubService) Logger(chan<- error) (service.Logger, error) { return nil, nil }
func (s *stubService) SystemLogger(chan<- error) (service.Logger, error) { return nil, nil }
func (s *stubService) String() string                { return "stub" }
func (s *stubService) Platform() string              { return service.Platform() }
func (s *stubService) Status() (service.Status, error) { return s.status, s.statusErr }

func TestServiceConfig_DefaultArguments(t *testing.T) {
	t.Setenv(config.ConfigPathEnv, "")

	got := ServiceConfig("").Arguments
	want := []string{"gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestServiceConfig_IncludesConfigFlag(t *testing.T) {
	t.Setenv(config.ConfigPathEnv, "")
	configFile := filepath.Join(t.TempDir(), "service-config.json")

	got := ServiceConfig(configFile).Arguments
	want := []string{"-c", configFile, "gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestServiceConfig_UsesConfigPathEnv(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "env-config.json")
	t.Setenv(config.ConfigPathEnv, configFile)

	got := ServiceConfig("").Arguments
	want := []string{"-c", configFile, "gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestInspectGatewayServiceMapsInstalledStatus(t *testing.T) {
	original := newService
	t.Cleanup(func() { newService = original })

	stub := &stubService{status: service.StatusRunning}
	newService = func(service.Interface, *service.Config) (service.Service, error) {
		return stub, nil
	}

	configFile := filepath.Join(t.TempDir(), "nekobot.json")
	status, err := InspectGatewayService(configFile)
	if err != nil {
		t.Fatalf("InspectGatewayService failed: %v", err)
	}
	if !status.Installed || status.Status != "running" {
		t.Fatalf("unexpected service status: %+v", status)
	}
	if status.ConfigPath != configFile {
		t.Fatalf("unexpected config path: %+v", status)
	}
}

func TestInspectGatewayServiceTreatsStatusErrorAsNotInstalled(t *testing.T) {
	original := newService
	t.Cleanup(func() { newService = original })

	stub := &stubService{statusErr: errors.New("service not installed")}
	newService = func(service.Interface, *service.Config) (service.Service, error) {
		return stub, nil
	}

	status, err := InspectGatewayService("")
	if err != nil {
		t.Fatalf("InspectGatewayService failed: %v", err)
	}
	if status.Installed || status.Status != "not_installed" {
		t.Fatalf("unexpected service status: %+v", status)
	}
}

func TestRestartGatewayServiceCallsRestart(t *testing.T) {
	original := newService
	t.Cleanup(func() { newService = original })

	stub := &stubService{}
	newService = func(service.Interface, *service.Config) (service.Service, error) {
		return stub, nil
	}

	if err := RestartGatewayService(""); err != nil {
		t.Fatalf("RestartGatewayService failed: %v", err)
	}
	if stub.restartCalls != 1 {
		t.Fatalf("expected restart to be called once, got %d", stub.restartCalls)
	}
}
