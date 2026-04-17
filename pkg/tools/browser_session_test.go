package tools

import (
	"context"
	"strings"
	"os/exec"
	"errors"
	"testing"
	"time"

	"github.com/mafredri/cdp/devtool"
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

type testBrowserDevTools struct{}

func (testBrowserDevTools) List(context.Context) ([]*devtool.Target, error) { return nil, nil }
func (testBrowserDevTools) Create(context.Context) (*devtool.Target, error) { return nil, nil }
func (testBrowserDevTools) CreateURL(context.Context, string) (*devtool.Target, error) {
	return nil, nil
}
func (testBrowserDevTools) Activate(context.Context, *devtool.Target) error { return nil }
func (testBrowserDevTools) Close(context.Context, *devtool.Target) error    { return nil }

func TestBrowserSessionGetDevToolsRequiresReadySession(t *testing.T) {
	session := &BrowserSession{}

	_, err := session.GetDevTools()
	if err == nil {
		t.Fatal("expected browser session not ready error")
	}
	if err.Error() != "browser session not ready" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBrowserSessionGetDevToolsReturnsFactoryForEndpoint(t *testing.T) {
	session := &BrowserSession{
		ready:    true,
		endpoint: "http://chrome.internal:9333",
	}

	called := 0
	session.devtoolsFactory = func(endpoint string) browserDevTools {
		called++
		if endpoint != "http://chrome.internal:9333" {
			t.Fatalf("expected endpoint propagated to devtools factory, got %q", endpoint)
		}
		return testBrowserDevTools{}
	}

	devtools, err := session.GetDevTools()
	if err != nil {
		t.Fatalf("GetDevTools returned error: %v", err)
	}
	if devtools == nil {
		t.Fatal("expected devtools instance")
	}
	if called != 1 {
		t.Fatalf("expected devtools factory to be called once, got %d", called)
	}
}

func TestBrowserSessionStartWithModeAutoCleansUpAfterLaunchFailure(t *testing.T) {
	session := &BrowserSession{timeout: 5 * time.Second, log: newToolsTestLogger(t)}
	session.connectFn = func(port int, timeout time.Duration) error {
		return errors.New("not running")
	}
	cmd := exec.Command("bash", "-lc", "sleep 30")
	session.launchFn = func(timeout time.Duration) error {
		_, cancel := context.WithTimeout(context.Background(), timeout)
		session.cancel = cancel
		session.cmd = cmd
		if err := session.cmd.Start(); err != nil {
			return err
		}
		if err := session.stopLocked(); err != nil {
			return err
		}
		return errors.New("failed to connect after launch")
	}

	err := session.StartWithMode(2*time.Second, BrowserModeAuto)
	if err == nil {
		t.Fatal("expected launch failure")
	}
	if session.cmd != nil || session.cancel != nil || session.conn != nil || session.client != nil || session.ready {
		t.Fatalf("expected failed launch cleanup, got %+v", session.Status())
	}
}

func TestBrowserSessionLaunchWithoutChromeCleansUpCancel(t *testing.T) {
	session := &BrowserSession{timeout: 5 * time.Second, log: newToolsTestLogger(t)}
	session.findChromeFn = func() string { return "" }

	err := session.launch(2 * time.Second)
	if err == nil {
		t.Fatal("expected chrome-not-found failure")
	}
	if !strings.Contains(err.Error(), "chrome not found in PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.cancel != nil || session.cmd != nil || session.conn != nil || session.client != nil || session.ready {
		t.Fatalf("expected launch failure without chrome to leave no session state, got ready=%v cmd=%v cancel=%v conn=%v client=%v",
			session.ready, session.cmd, session.cancel, session.conn, session.client)
	}
}
