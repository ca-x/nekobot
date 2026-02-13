// Package heartbeat implements periodic autonomous task execution.
package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/session"
)

const (
	stateFile     = "memory/heartbeat-state.json"
	heartbeatFile = "HEARTBEAT.md"
	sessionKey    = "heartbeat:system"
	defaultPrompt = "Check workspace health and report any issues."
)

// State stores heartbeat execution state.
type State struct {
	LastRun       time.Time `json:"last_run"`
	RunCount      int       `json:"run_count"`
	LastDuration  string    `json:"last_duration"`
	LastError     string    `json:"last_error,omitempty"`
	NextScheduled time.Time `json:"next_scheduled"`
}

// Task represents a heartbeat task extracted from HEARTBEAT.md.
type Task struct {
	Name   string
	Prompt string
}

// Service manages periodic heartbeat execution.
type Service struct {
	log    *logger.Logger
	config *config.Config
	agent  *agent.Agent
	sess   *session.Manager
	bus    bus.Bus

	workspacePath string
	interval      time.Duration
	enabled       bool
	running       bool
	ticker        *time.Ticker
	stopCh        chan struct{}
	mu            sync.RWMutex

	state State
}

// NewService creates a new heartbeat service.
func NewService(
	log *logger.Logger,
	cfg *config.Config,
	ag *agent.Agent,
	sm *session.Manager,
	b bus.Bus,
) *Service {
	interval := time.Duration(cfg.Heartbeat.IntervalMinutes) * time.Minute
	if interval < 5*time.Minute {
		interval = 5 * time.Minute // Minimum 5 minutes
	}

	return &Service{
		log:           log,
		config:        cfg,
		agent:         ag,
		sess:          sm,
		bus:           b,
		workspacePath: cfg.WorkspacePath(),
		interval:      interval,
		enabled:       cfg.Heartbeat.Enabled,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the heartbeat service.
func (s *Service) Start(ctx context.Context) error {
	if !s.enabled {
		s.log.Info("Heartbeat disabled in config")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("heartbeat already running")
	}

	// Load state
	if err := s.loadState(); err != nil {
		s.log.Warn("Failed to load heartbeat state, starting fresh", zap.Error(err))
	}

	s.running = true
	s.ticker = time.NewTicker(s.interval)

	s.log.Info("Heartbeat service started",
		zap.Duration("interval", s.interval),
		zap.Time("last_run", s.state.LastRun))

	// Start heartbeat loop
	go s.run(ctx)

	return nil
}

// Stop stops the heartbeat service.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)

	if s.ticker != nil {
		s.ticker.Stop()
	}

	// Save final state
	if err := s.saveState(); err != nil {
		s.log.Error("Failed to save heartbeat state", zap.Error(err))
	}

	s.log.Info("Heartbeat service stopped")
	return nil
}

// run is the main heartbeat loop.
func (s *Service) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			s.executeHeartbeat(ctx)
		}
	}
}

// executeHeartbeat executes a heartbeat cycle.
func (s *Service) executeHeartbeat(ctx context.Context) {
	s.log.Info("Executing heartbeat cycle",
		zap.Int("run_count", s.state.RunCount+1))

	start := time.Now()

	// Load tasks from HEARTBEAT.md
	tasks, err := s.loadTasks()
	if err != nil {
		s.log.Error("Failed to load heartbeat tasks", zap.Error(err))
		s.updateState(start, err)
		return
	}

	if len(tasks) == 0 {
		s.log.Warn("No heartbeat tasks found, using default")
		tasks = []Task{{Name: "Default", Prompt: defaultPrompt}}
	}

	// Get or create heartbeat session
	sess, err := s.sess.Get(sessionKey)
	if err != nil {
		s.log.Error("Failed to get heartbeat session", zap.Error(err))
		s.updateState(start, err)
		return
	}

	// Execute each task
	for i, task := range tasks {
		s.log.Debug("Executing heartbeat task",
			zap.Int("task_num", i+1),
			zap.String("task_name", task.Name))

		// Execute task with timeout
		taskCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_, err := s.agent.Chat(taskCtx, sess, task.Prompt)
		cancel()

		if err != nil {
			s.log.Error("Heartbeat task failed",
				zap.String("task_name", task.Name),
				zap.Error(err))
			// Continue with other tasks
		}
	}

	duration := time.Since(start)
	s.log.Info("Heartbeat cycle completed",
		zap.Duration("duration", duration),
		zap.Int("tasks", len(tasks)))

	s.updateState(start, nil)
}

// loadTasks loads tasks from HEARTBEAT.md.
func (s *Service) loadTasks() ([]Task, error) {
	heartbeatPath := filepath.Join(s.workspacePath, heartbeatFile)

	// Check if file exists
	if _, err := os.Stat(heartbeatPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("HEARTBEAT.md not found at %s", heartbeatPath)
	}

	// Read file
	content, err := os.ReadFile(heartbeatPath)
	if err != nil {
		return nil, fmt.Errorf("reading HEARTBEAT.md: %w", err)
	}

	// Parse tasks from markdown
	tasks := s.parseTasksFromMarkdown(string(content))
	return tasks, nil
}

// parseTasksFromMarkdown extracts tasks from markdown content.
func (s *Service) parseTasksFromMarkdown(content string) []Task {
	var tasks []Task

	// Regex to find code blocks with ```prompt
	re := regexp.MustCompile(`(?s)### Task \d+: (.+?)\s+` + "```prompt\n(.+?)```")
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) == 3 {
			tasks = append(tasks, Task{
				Name:   match[1],
				Prompt: match[2],
			})
		}
	}

	// Also look for custom tasks
	customRe := regexp.MustCompile(`(?s)### Example: (.+?)\s+` + "```prompt\n(.+?)```")
	customMatches := customRe.FindAllStringSubmatch(content, -1)

	for _, match := range customMatches {
		if len(match) == 3 {
			tasks = append(tasks, Task{
				Name:   match[1],
				Prompt: match[2],
			})
		}
	}

	return tasks
}

// updateState updates the heartbeat state.
func (s *Service) updateState(startTime time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := time.Since(startTime)

	s.state.LastRun = startTime
	s.state.RunCount++
	s.state.LastDuration = duration.String()
	s.state.NextScheduled = startTime.Add(s.interval)

	if err != nil {
		s.state.LastError = err.Error()
	} else {
		s.state.LastError = ""
	}

	// Save state
	if err := s.saveState(); err != nil {
		s.log.Error("Failed to save heartbeat state", zap.Error(err))
	}
}

// loadState loads heartbeat state from disk.
func (s *Service) loadState() error {
	statePath := filepath.Join(s.workspacePath, stateFile)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with default state
			s.state = State{
				NextScheduled: time.Now().Add(s.interval),
			}
			return nil
		}
		return fmt.Errorf("reading state file: %w", err)
	}

	if err := json.Unmarshal(data, &s.state); err != nil {
		return fmt.Errorf("unmarshaling state: %w", err)
	}

	return nil
}

// saveState saves heartbeat state to disk.
func (s *Service) saveState() error {
	statePath := filepath.Join(s.workspacePath, stateFile)

	// Ensure directory exists
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// GetState returns the current heartbeat state.
func (s *Service) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// IsRunning returns whether the heartbeat service is running.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerNow triggers an immediate heartbeat execution.
func (s *Service) TriggerNow(ctx context.Context) error {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if !running {
		return fmt.Errorf("heartbeat service not running")
	}

	go s.executeHeartbeat(ctx)
	return nil
}
