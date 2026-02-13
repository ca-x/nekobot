// Package cron provides cron job scheduling for the agent.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/logger"
)

// Job represents a cron job.
type Job struct {
	ID          string    `json:"id"`           // Unique job ID
	Name        string    `json:"name"`         // Human-readable name
	Schedule    string    `json:"schedule"`     // Cron expression
	Prompt      string    `json:"prompt"`       // Task prompt for agent
	Enabled     bool      `json:"enabled"`      // Whether job is enabled
	CreatedAt   time.Time `json:"created_at"`   // Creation timestamp
	LastRun     time.Time `json:"last_run"`     // Last execution time
	NextRun     time.Time `json:"next_run"`     // Next scheduled run
	RunCount    int       `json:"run_count"`    // Total executions
	LastError   string    `json:"last_error"`   // Last error message
	LastSuccess bool      `json:"last_success"` // Whether last run succeeded
}

// Manager manages cron jobs.
type Manager struct {
	log       *logger.Logger
	agent     *agent.Agent
	workspace string
	jobsFile  string

	// Cron scheduler
	scheduler *cron.Cron
	jobs      map[string]*Job     // Job ID -> Job
	entries   map[string]cron.EntryID // Job ID -> Cron entry ID
	mu        sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

const (
	jobsFileName = "jobs.json"
)

// New creates a new cron manager.
func New(log *logger.Logger, ag *agent.Agent, workspace string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		log:       log,
		agent:     ag,
		workspace: workspace,
		jobsFile:  filepath.Join(workspace, jobsFileName),
		scheduler: cron.New(cron.WithSeconds()),
		jobs:      make(map[string]*Job),
		entries:   make(map[string]cron.EntryID),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the cron manager.
func (m *Manager) Start() error {
	m.log.Info("Starting cron manager")

	// Load jobs from disk
	if err := m.loadJobs(); err != nil {
		m.log.Warn("Failed to load jobs", zap.Error(err))
	}

	// Schedule loaded jobs
	for _, job := range m.jobs {
		if job.Enabled {
			if err := m.scheduleJob(job); err != nil {
				m.log.Error("Failed to schedule job",
					zap.String("job_id", job.ID),
					zap.Error(err))
			}
		}
	}

	// Start scheduler
	m.scheduler.Start()

	return nil
}

// Stop stops the cron manager.
func (m *Manager) Stop() error {
	m.log.Info("Stopping cron manager")

	// Stop scheduler
	ctx := m.scheduler.Stop()
	<-ctx.Done()

	m.cancel()

	m.log.Info("Cron manager stopped")
	return nil
}

// AddJob adds a new cron job.
func (m *Manager) AddJob(name, schedule, prompt string) (*Job, error) {
	// Validate schedule
	if _, err := cron.ParseStandard(schedule); err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	job := &Job{
		ID:        generateJobID(),
		Name:      name,
		Schedule:  schedule,
		Prompt:    prompt,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	// Schedule the job
	if err := m.scheduleJob(job); err != nil {
		return nil, fmt.Errorf("scheduling job: %w", err)
	}

	// Save to disk
	if err := m.saveJobs(); err != nil {
		m.log.Error("Failed to save jobs", zap.Error(err))
	}

	m.log.Info("Added cron job",
		zap.String("job_id", job.ID),
		zap.String("name", name),
		zap.String("schedule", schedule))

	return job, nil
}

// RemoveJob removes a cron job.
func (m *Manager) RemoveJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Remove from scheduler
	if entryID, exists := m.entries[jobID]; exists {
		m.scheduler.Remove(entryID)
		delete(m.entries, jobID)
	}

	// Remove from jobs map
	delete(m.jobs, jobID)

	// Save to disk
	if err := m.saveJobs(); err != nil {
		m.log.Error("Failed to save jobs", zap.Error(err))
	}

	m.log.Info("Removed cron job",
		zap.String("job_id", jobID),
		zap.String("name", job.Name))

	return nil
}

// EnableJob enables a job.
func (m *Manager) EnableJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Enabled {
		return nil // Already enabled
	}

	job.Enabled = true

	// Schedule the job
	if err := m.scheduleJob(job); err != nil {
		return fmt.Errorf("scheduling job: %w", err)
	}

	// Save to disk
	if err := m.saveJobs(); err != nil {
		m.log.Error("Failed to save jobs", zap.Error(err))
	}

	m.log.Info("Enabled cron job", zap.String("job_id", jobID))
	return nil
}

// DisableJob disables a job.
func (m *Manager) DisableJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if !job.Enabled {
		return nil // Already disabled
	}

	job.Enabled = false

	// Remove from scheduler
	if entryID, exists := m.entries[jobID]; exists {
		m.scheduler.Remove(entryID)
		delete(m.entries, jobID)
	}

	// Save to disk
	if err := m.saveJobs(); err != nil {
		m.log.Error("Failed to save jobs", zap.Error(err))
	}

	m.log.Info("Disabled cron job", zap.String("job_id", jobID))
	return nil
}

// ListJobs returns all jobs.
func (m *Manager) ListJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		// Create a copy
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}

	return jobs
}

// GetJob returns a job by ID.
func (m *Manager) GetJob(jobID string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Return a copy
	jobCopy := *job
	return &jobCopy, nil
}

// scheduleJob schedules a job in the cron scheduler.
// Caller must hold m.mu lock.
func (m *Manager) scheduleJob(job *Job) error {
	// Remove existing entry if present
	if entryID, exists := m.entries[job.ID]; exists {
		m.scheduler.Remove(entryID)
	}

	// Add to scheduler
	entryID, err := m.scheduler.AddFunc(job.Schedule, func() {
		m.executeJob(job.ID)
	})
	if err != nil {
		return err
	}

	m.entries[job.ID] = entryID

	// Update next run time
	entry := m.scheduler.Entry(entryID)
	job.NextRun = entry.Next

	return nil
}

// executeJob executes a job.
func (m *Manager) executeJob(jobID string) {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists || !job.Enabled {
		m.mu.Unlock()
		return
	}

	jobName := job.Name
	prompt := job.Prompt
	m.mu.Unlock()

	m.log.Info("Executing cron job",
		zap.String("job_id", jobID),
		zap.String("name", jobName))

	// Execute with agent
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	defer cancel()

	fullPrompt := fmt.Sprintf(`# Cron Job: %s

Scheduled task execution at %s:

%s`,
		jobName,
		time.Now().Format(time.RFC3339),
		prompt)

	response, err := m.agent.Chat(ctx, fullPrompt)

	// Update job status
	m.mu.Lock()
	if job, exists := m.jobs[jobID]; exists {
		job.LastRun = time.Now()
		job.RunCount++

		if err != nil {
			job.LastSuccess = false
			job.LastError = err.Error()
			m.log.Error("Cron job failed",
				zap.String("job_id", jobID),
				zap.Error(err))
		} else {
			job.LastSuccess = true
			job.LastError = ""
			m.log.Info("Cron job completed",
				zap.String("job_id", jobID),
				zap.String("response_preview", truncate(response, 100)))
		}

		// Update next run time
		if entryID, exists := m.entries[jobID]; exists {
			entry := m.scheduler.Entry(entryID)
			job.NextRun = entry.Next
		}
	}
	m.mu.Unlock()

	// Save updated state
	if err := m.saveJobs(); err != nil {
		m.log.Error("Failed to save jobs after execution", zap.Error(err))
	}
}

// loadJobs loads jobs from disk.
func (m *Manager) loadJobs() error {
	data, err := os.ReadFile(m.jobsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No jobs file yet
		}
		return err
	}

	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return fmt.Errorf("unmarshaling jobs: %w", err)
	}

	m.mu.Lock()
	for _, job := range jobs {
		m.jobs[job.ID] = job
	}
	m.mu.Unlock()

	m.log.Info("Loaded cron jobs", zap.Int("count", len(jobs)))
	return nil
}

// saveJobs saves jobs to disk.
func (m *Manager) saveJobs() error {
	m.mu.RLock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling jobs: %w", err)
	}

	if err := os.WriteFile(m.jobsFile, data, 0644); err != nil {
		return fmt.Errorf("writing jobs file: %w", err)
	}

	return nil
}

// generateJobID generates a unique job ID.
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
