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
		{input: "DIRECT", want: BrowserModeDirect},
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
