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

func (s *stubService) Run() error                                        { return nil }
func (s *stubService) Start() error                                      { return nil }
func (s *stubService) Stop() error                                       { return nil }
func (s *stubService) Restart() error                                    { s.restartCalls++; return s.restartErr }
func (s *stubService) Install() error                                    { return nil }
func (s *stubService) Uninstall() error                                  { return nil }
func (s *stubService) Logger(chan<- error) (service.Logger, error)       { return nil, nil }
func (s *stubService) SystemLogger(chan<- error) (service.Logger, error) { return nil, nil }
func (s *stubService) String() string                                    { return "stub" }
func (s *stubService) Platform() string                                  { return service.Platform() }
func (s *stubService) Status() (service.Status, error)                   { return s.status, s.statusErr }

func TestGatewayConfigDefaultArguments(t *testing.T) {
	t.Setenv(config.ConfigPathEnv, "")
	got := GatewayConfig("").Arguments
	want := []string{"gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestNekoClientdConfigDefaultArguments(t *testing.T) {
	t.Setenv(config.ConfigPathEnv, "")
	got := NekoClientdConfig("").Arguments
	want := []string{"run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestGatewayConfigIncludesConfigFlag(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "service-config.json")
	got := GatewayConfig(configFile).Arguments
	want := []string{"-c", configFile, "gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestInspectNekoClientdServiceTreatsStatusErrorAsNotInstalled(t *testing.T) {
	original := newService
	t.Cleanup(func() { newService = original })
	stub := &stubService{statusErr: errors.New("service not installed")}
	newService = func(service.Interface, *service.Config) (service.Service, error) { return stub, nil }
	status, err := InspectNekoClientdService("")
	if err != nil {
		t.Fatalf("InspectNekoClientdService failed: %v", err)
	}
	if status.Installed || status.Status != "not_installed" {
		t.Fatalf("unexpected service status: %+v", status)
	}
}

func TestRestartNekoClientdServiceCallsRestart(t *testing.T) {
	original := newService
	t.Cleanup(func() { newService = original })
	stub := &stubService{}
	newService = func(service.Interface, *service.Config) (service.Service, error) { return stub, nil }
	if err := RestartNekoClientdService(""); err != nil {
		t.Fatalf("RestartNekoClientdService failed: %v", err)
	}
	if stub.restartCalls != 1 {
		t.Fatalf("expected restart to be called once, got %d", stub.restartCalls)
	}
}
