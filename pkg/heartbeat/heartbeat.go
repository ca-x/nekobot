// Package heartbeat provides periodic autonomous task execution for the agent.
package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

// Heartbeat manages periodic autonomous agent tasks.
type Heartbeat struct {
	log       *logger.Logger
	agent     *agent.Agent
	state     state.KV
	workspace string
	interval  time.Duration

	// Lifecycle
	ctx       context.Context
	cancel    context.CancelFunc
	ticker    *time.Ticker
	wg        sync.WaitGroup
	enabled   bool
	enabledMu sync.RWMutex
}

// Config configures the heartbeat system.
type Config struct {
	Enabled   bool          // Enable heartbeat
	Interval  time.Duration // Heartbeat interval (default: 1 hour)
	Workspace string        // Workspace directory
}

const (
	heartbeatFile     = "HEARTBEAT.md"
	stateKeyEnabled   = "heartbeat.enabled"
	stateKeyLastRun   = "heartbeat.last_run"
	stateKeyRunCount  = "heartbeat.run_count"
	defaultPrompt     = "# Heartbeat Tasks\n\nNo tasks defined. Create HEARTBEAT.md in your workspace to define periodic tasks."
)

// New creates a new heartbeat system.
func New(log *logger.Logger, ag *agent.Agent, st state.KV, cfg *Config) *Heartbeat {
	if cfg.Interval == 0 {
		cfg.Interval = 1 * time.Hour
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Check if enabled in state (persists across restarts)
	enabled := cfg.Enabled
	if storedEnabled, exists, _ := st.GetBool(ctx, stateKeyEnabled); exists {
		enabled = storedEnabled
	}

	return &Heartbeat{
		log:       log,
		agent:     ag,
		state:     st,
		workspace: cfg.Workspace,
		interval:  cfg.Interval,
		ctx:       ctx,
		cancel:    cancel,
		enabled:   enabled,
	}
}

// Start starts the heartbeat system.
func (h *Heartbeat) Start() error {
	h.enabledMu.RLock()
	enabled := h.enabled
	h.enabledMu.RUnlock()

	if !enabled {
		h.log.Info("Heartbeat is disabled, not starting")
		return nil
	}

	h.log.Info("Starting heartbeat", zap.Duration("interval", h.interval))

	h.ticker = time.NewTicker(h.interval)

	h.wg.Add(1)
	go h.run()

	return nil
}

// Stop stops the heartbeat system.
func (h *Heartbeat) Stop() error {
	h.log.Info("Stopping heartbeat")

	if h.ticker != nil {
		h.ticker.Stop()
	}

	h.cancel()
	h.wg.Wait()

	h.log.Info("Heartbeat stopped")
	return nil
}

// Enable enables the heartbeat system.
func (h *Heartbeat) Enable() {
	h.enabledMu.Lock()
	h.enabled = true
	h.enabledMu.Unlock()

	ctx := context.Background()
	h.state.Set(ctx, stateKeyEnabled, true)
	h.log.Info("Heartbeat enabled")

	// Start if not already running
	if h.ticker == nil {
		h.Start()
	}
}

// Disable disables the heartbeat system.
func (h *Heartbeat) Disable() {
	h.enabledMu.Lock()
	h.enabled = false
	h.enabledMu.Unlock()

	ctx := context.Background()
	h.state.Set(ctx, stateKeyEnabled, false)
	h.log.Info("Heartbeat disabled")
}

// IsEnabled returns whether heartbeat is enabled.
func (h *Heartbeat) IsEnabled() bool {
	h.enabledMu.RLock()
	defer h.enabledMu.RUnlock()
	return h.enabled
}

// GetStats returns heartbeat statistics.
func (h *Heartbeat) GetStats() map[string]interface{} {
	ctx := context.Background()
	lastRun, _ := h.state.GetString(ctx, stateKeyLastRun)
	runCount, _ := h.state.GetInt(ctx, stateKeyRunCount)

	return map[string]interface{}{
		"enabled":   h.IsEnabled(),
		"interval":  h.interval.String(),
		"last_run":  lastRun,
		"run_count": runCount,
	}
}

// TriggerNow immediately triggers a heartbeat execution.
func (h *Heartbeat) TriggerNow() error {
	h.log.Info("Manual heartbeat trigger")
	return h.execute()
}

// run is the main heartbeat loop.
func (h *Heartbeat) run() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ticker.C:
			h.enabledMu.RLock()
			enabled := h.enabled
			h.enabledMu.RUnlock()

			if !enabled {
				continue
			}

			if err := h.execute(); err != nil {
				h.log.Error("Heartbeat execution failed", zap.Error(err))
			}

		case <-h.ctx.Done():
			return
		}
	}
}

// execute runs a heartbeat cycle.
func (h *Heartbeat) execute() error {
	h.log.Info("Executing heartbeat")

	// Load heartbeat prompt
	prompt, err := h.loadHeartbeatPrompt()
	if err != nil {
		return fmt.Errorf("loading heartbeat prompt: %w", err)
	}

	// Build full prompt with context
	fullPrompt := fmt.Sprintf(`# Heartbeat Execution

Current time: %s

You are running a periodic heartbeat check. Review the following tasks and execute any that are due:

%s

If no tasks are due or defined, respond with "No heartbeat tasks to execute at this time."`,
		time.Now().Format(time.RFC3339),
		prompt)

	// Execute with agent
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Minute)
	defer cancel()

	response, err := h.agent.Chat(ctx, fullPrompt)
	if err != nil {
		return fmt.Errorf("agent chat failed: %w", err)
	}

	h.log.Info("Heartbeat completed",
		zap.String("response_preview", truncate(response, 100)))

	// Update state
	ctx2 := context.Background()
	h.state.Set(ctx2, stateKeyLastRun, time.Now().Format(time.RFC3339))
	h.state.UpdateFunc(ctx2, stateKeyRunCount, func(current interface{}) interface{} {
		if current == nil {
			return 1
		}
		if count, ok := current.(int); ok {
			return count + 1
		}
		// Handle float64 from JSON unmarshaling
		if count, ok := current.(float64); ok {
			return int(count) + 1
		}
		return 1
	})

	return nil
}

// loadHeartbeatPrompt loads the heartbeat prompt from HEARTBEAT.md.
func (h *Heartbeat) loadHeartbeatPrompt() (string, error) {
	path := filepath.Join(h.workspace, heartbeatFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultPrompt, nil
		}
		return "", err
	}

	if len(data) == 0 {
		return defaultPrompt, nil
	}

	return string(data), nil
}

// SetInterval changes the heartbeat interval.
func (h *Heartbeat) SetInterval(interval time.Duration) {
	h.interval = interval

	// Restart ticker if running
	if h.ticker != nil {
		h.ticker.Stop()
		h.ticker = time.NewTicker(interval)
	}

	h.log.Info("Heartbeat interval updated", zap.Duration("interval", interval))
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
