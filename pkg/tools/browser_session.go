package tools

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
	"nekobot/pkg/logger"
)

var (
	browserSession     *BrowserSession
	browserSessionOnce sync.Once
)

// BrowserSession manages a persistent browser session.
type BrowserSession struct {
	client     *cdp.Client
	conn       *rpcc.Conn
	cancel     context.CancelFunc
	cmd        *exec.Cmd
	debugURL   string
	timeout    time.Duration
	mu         sync.RWMutex
	ready      bool
	log        *logger.Logger
}

// GetBrowserSession returns the singleton browser session.
func GetBrowserSession(log *logger.Logger) *BrowserSession {
	browserSessionOnce.Do(func() {
		browserSession = &BrowserSession{
			timeout: 30 * time.Second,
			log:     log,
		}
	})
	return browserSession
}

// Start starts the browser session.
func (s *BrowserSession) Start(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return nil // Already started
	}

	if timeout > 0 {
		s.timeout = timeout
	}

	// Launch Chrome with remote debugging
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	s.cancel = cancel

	// Try to find Chrome executable
	chromePath := findChrome()
	if chromePath == "" {
		return fmt.Errorf("chrome not found in PATH")
	}

	s.log.Info("Starting Chrome browser",
		logger.String("path", chromePath))

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

	// Connect to Chrome DevTools
	devt := devtool.New("http://127.0.0.1:9222")

	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to get page target: %w", err)
	}

	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to connect to chrome: %w", err)
	}

	s.conn = conn
	s.client = cdp.NewClient(conn)
	s.debugURL = pt.WebSocketDebuggerURL
	s.ready = true

	s.log.Info("Browser session started",
		logger.String("debug_url", s.debugURL))

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
