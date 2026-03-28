package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

var (
	browserSession     *BrowserSession
	browserSessionOnce sync.Once
)

// BrowserConnectionMode controls how the browser session attaches to Chrome.
type BrowserConnectionMode string

const (
	BrowserModeAuto   BrowserConnectionMode = "auto"
	BrowserModeDirect BrowserConnectionMode = "direct"
)

// BrowserSession manages a persistent browser session.
type BrowserSession struct {
	client    *cdp.Client
	conn      *rpcc.Conn
	cancel    context.CancelFunc
	cmd       *exec.Cmd
	debugURL  string
	timeout   time.Duration
	mu        sync.RWMutex
	ready     bool
	log       *logger.Logger
	mode      BrowserConnectionMode
	connectFn func(port int, timeout time.Duration) error
	launchFn  func(timeout time.Duration) error
}

// GetBrowserSession returns the singleton browser session.
func GetBrowserSession(log *logger.Logger) *BrowserSession {
	browserSessionOnce.Do(func() {
		browserSession = &BrowserSession{
			timeout: 30 * time.Second,
			log:     log,
		}
		browserSession.connectFn = browserSession.connect
		browserSession.launchFn = browserSession.launch
	})
	return browserSession
}

// Start starts the browser session.
func (s *BrowserSession) Start(timeout time.Duration) error {
	return s.StartWithMode(timeout, BrowserModeAuto)
}

// StartWithMode starts the browser session with the requested connection mode.
func (s *BrowserSession) StartWithMode(timeout time.Duration, mode BrowserConnectionMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return nil // Already started
	}

	if timeout > 0 {
		s.timeout = timeout
	}
	mode = resolveBrowserMode(string(mode))
	if s.connectFn == nil {
		s.connectFn = s.connect
	}
	if s.launchFn == nil {
		s.launchFn = s.launch
	}

	switch mode {
	case BrowserModeDirect:
		if err := s.connectFn(9222, s.timeout); err == nil {
			s.mode = BrowserModeDirect
			return nil
		}
		return s.launchFn(s.timeout)
	default:
		for _, port := range []int{9222, 9223, 9224} {
			if err := s.connectFn(port, s.timeout); err == nil {
				s.mode = BrowserModeDirect
				return nil
			}
		}
		return s.launchFn(s.timeout)
	}
}

func (s *BrowserSession) launch(timeout time.Duration) error {
	// Launch Chrome with remote debugging
	_, cancel := context.WithTimeout(context.Background(), s.timeout)
	s.cancel = cancel

	// Try to find Chrome executable
	chromePath := findChrome()
	if chromePath == "" {
		return fmt.Errorf("chrome not found in PATH")
	}

	s.log.Info("Starting Chrome browser",
		zap.String("path", chromePath))

	// Launch Chrome with remote debugging port
	s.cmd = exec.Command(chromePath,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--remote-debugging-port=9222",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-default-apps",
		"--disable-sync",
		"--metrics-recording-only",
		"--no-first-run",
		"--safebrowsing-disable-auto-update",
		"--disable-blink-features=AutomationControlled",
	)

	if err := s.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start chrome: %w", err)
	}

	// Wait for Chrome to be ready
	time.Sleep(2 * time.Second)

	if err := s.connect(9222, timeout); err != nil {
		if stopErr := s.Stop(); stopErr != nil {
			s.log.Warn("Failed to stop browser session after launch failure", zap.Error(stopErr))
		}
		return err
	}
	s.mode = BrowserModeDirect

	s.log.Info("Browser session started",
		zap.String("debug_url", s.debugURL))

	return nil
}

func (s *BrowserSession) connect(port int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	devt := devtool.New(fmt.Sprintf("http://127.0.0.1:%d", port))
	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		return fmt.Errorf("failed to get page target: %w", err)
	}

	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		return fmt.Errorf("failed to connect to chrome: %w", err)
	}

	s.conn = conn
	s.client = cdp.NewClient(conn)
	s.debugURL = pt.WebSocketDebuggerURL
	s.ready = true
	return nil
}

// Stop stops the browser session.
func (s *BrowserSession) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return nil
	}

	s.log.Info("Stopping browser session")

	if s.conn != nil {
		_ = s.conn.Close()
	}

	if s.cancel != nil {
		s.cancel()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}

	s.ready = false
	s.client = nil
	s.conn = nil
	s.mode = BrowserModeAuto

	return nil
}

// IsReady returns whether the session is ready.
func (s *BrowserSession) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// GetClient returns the CDP client.
func (s *BrowserSession) GetClient() (*cdp.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready || s.client == nil {
		return nil, fmt.Errorf("browser session not ready")
	}

	return s.client, nil
}

// ConnectionMode returns the active browser connection mode.
func (s *BrowserSession) ConnectionMode() BrowserConnectionMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func resolveBrowserMode(mode string) BrowserConnectionMode {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case string(BrowserModeDirect):
		return BrowserModeDirect
	default:
		return BrowserModeAuto
	}
}

// findChrome finds the Chrome executable in PATH.
func findChrome() string {
	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
	}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	return ""
}
