package tools

import (
	"errors"
	"testing"
	"time"
)

func TestResolveBrowserMode(t *testing.T) {
	tests := []struct {
		input string
		want  BrowserConnectionMode
	}{
		{input: "", want: BrowserModeAuto},
		{input: "auto", want: BrowserModeAuto},
		{input: "direct", want: BrowserModeDirect},
		{input: "relay", want: BrowserModeRelay},
		{input: "DIRECT", want: BrowserModeDirect},
		{input: "RELAY", want: BrowserModeRelay},
		{input: "weird", want: BrowserModeAuto},
	}

	for _, tt := range tests {
		if got := resolveBrowserMode(tt.input); got != tt.want {
			t.Fatalf("resolveBrowserMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBrowserSessionStartWithModeAutoFallsBackToLaunch(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	var connectCalls []int
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls = append(connectCalls, port)
		return errors.New("not running")
	}
	launched := false
	session.launchFn = func(timeout time.Duration) error {
		launched = true
		session.ready = true
		session.mode = BrowserModeDirect
		return nil
	}

	if err := session.StartWithMode(3*time.Second, BrowserModeAuto); err != nil {
		t.Fatalf("StartWithMode failed: %v", err)
	}
	if !launched {
		t.Fatal("expected launch fallback")
	}
	if len(connectCalls) != 3 {
		t.Fatalf("expected 3 connect attempts, got %d", len(connectCalls))
	}
	if !session.IsReady() {
		t.Fatal("expected session ready")
	}
	if got := session.ConnectionMode(); got != BrowserModeDirect {
		t.Fatalf("expected direct mode after launch, got %q", got)
	}
}

func TestBrowserSessionStartWithModeDirectUsesExistingInstance(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	connectCalls := 0
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls++
		session.ready = true
		session.mode = BrowserModeDirect
		return nil
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called when existing browser connects")
		return nil
	}

	if err := session.StartWithMode(2*time.Second, BrowserModeDirect); err != nil {
		t.Fatalf("StartWithMode failed: %v", err)
	}
	if connectCalls != 1 {
		t.Fatalf("expected 1 connect attempt, got %d", connectCalls)
	}
	if got := session.ConnectionMode(); got != BrowserModeDirect {
		t.Fatalf("expected direct mode, got %q", got)
	}
}

func TestBrowserSessionStartWithModeDirectUsesExistingInstanceOnFallbackPort(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	var connectCalls []int
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls = append(connectCalls, port)
		if port == 9223 {
			session.ready = true
			session.mode = BrowserModeDirect
			return nil
		}
		return errors.New("not running")
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called when fallback debug port connects")
		return nil
	}

	if err := session.StartWithMode(2*time.Second, BrowserModeDirect); err != nil {
		t.Fatalf("StartWithMode failed: %v", err)
	}
	if len(connectCalls) != 2 {
		t.Fatalf("expected 2 direct attach attempts, got %d", len(connectCalls))
	}
	if got := connectCalls[0]; got != 9222 {
		t.Fatalf("expected first direct attempt on 9222, got %d", got)
	}
	if got := connectCalls[1]; got != 9223 {
		t.Fatalf("expected second direct attempt on 9223, got %d", got)
	}
	if got := session.ConnectionMode(); got != BrowserModeDirect {
		t.Fatalf("expected direct mode, got %q", got)
	}
}

func TestBrowserSessionStartWithModeRelayUsesExistingInstanceWithoutLaunch(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	var connectCalls []int
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls = append(connectCalls, port)
		if port != 9222 {
			t.Fatalf("expected relay mode to try the default relay port first, got %d", port)
		}
		session.ready = true
		session.mode = BrowserModeRelay
		return nil
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called in relay mode")
		return nil
	}

	if err := session.StartWithMode(2*time.Second, BrowserModeRelay); err != nil {
		t.Fatalf("StartWithMode failed: %v", err)
	}
	if len(connectCalls) != 1 {
		t.Fatalf("expected 1 connect attempt, got %d", len(connectCalls))
	}
	if got := session.ConnectionMode(); got != BrowserModeRelay {
		t.Fatalf("expected relay mode, got %q", got)
	}
}

func TestBrowserSessionStartWithModeRelayFailsWithoutExistingInstance(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	var connectCalls []int
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls = append(connectCalls, port)
		return errors.New("not running")
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called in relay mode")
		return nil
	}

	err := session.StartWithMode(2*time.Second, BrowserModeRelay)
	if err == nil {
		t.Fatal("expected relay mode error without existing instance")
	}
	if len(connectCalls) != 3 {
		t.Fatalf("expected 3 connect attempts, got %d", len(connectCalls))
	}
	if session.IsReady() {
		t.Fatal("expected session to remain not ready")
	}
}

func TestBrowserSessionStartWithOptionsDirectPrefersCustomPort(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	var connectCalls []int
	session.connectFn = func(port int, timeout time.Duration) error {
		connectCalls = append(connectCalls, port)
		if port != 9555 {
			t.Fatalf("expected custom debug port 9555 first, got %d", port)
		}
		session.ready = true
		session.mode = BrowserModeDirect
		return nil
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called when custom debug port connects")
		return nil
	}

	if err := session.StartWithOptions(2*time.Second, BrowserStartOptions{
		Mode:  BrowserModeDirect,
		Ports: []int{9555},
	}); err != nil {
		t.Fatalf("StartWithOptions failed: %v", err)
	}
	if len(connectCalls) != 1 {
		t.Fatalf("expected 1 custom port connect attempt, got %d", len(connectCalls))
	}
	if got := session.ConnectionMode(); got != BrowserModeDirect {
		t.Fatalf("expected direct mode, got %q", got)
	}
}

func TestBrowserSessionStartWithOptionsRelayUsesCustomEndpointWithoutLaunch(t *testing.T) {
	session := &BrowserSession{
		timeout: 5 * time.Second,
	}

	endpointCalls := 0
	session.connectEndpointFn = func(endpoint string, timeout time.Duration) error {
		endpointCalls++
		if endpoint != "http://chrome.internal:9333" {
			t.Fatalf("expected custom endpoint, got %q", endpoint)
		}
		session.ready = true
		session.mode = BrowserModeRelay
		return nil
	}
	session.connectFn = func(port int, timeout time.Duration) error {
		t.Fatalf("port-based connect should not be called when custom endpoint is provided, got %d", port)
		return nil
	}
	session.launchFn = func(timeout time.Duration) error {
		t.Fatal("launch should not be called in relay mode with custom endpoint")
		return nil
	}

	if err := session.StartWithOptions(2*time.Second, BrowserStartOptions{
		Mode:     BrowserModeRelay,
		Endpoint: "http://chrome.internal:9333",
	}); err != nil {
		t.Fatalf("StartWithOptions failed: %v", err)
	}
	if endpointCalls != 1 {
		t.Fatalf("expected 1 endpoint connect attempt, got %d", endpointCalls)
	}
	if got := session.ConnectionMode(); got != BrowserModeRelay {
		t.Fatalf("expected relay mode, got %q", got)
	}
}
