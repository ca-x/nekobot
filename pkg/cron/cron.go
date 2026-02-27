// Package cron provides cron job scheduling for the agent.
package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

// ScheduleKind defines the type of schedule.
type ScheduleKind string

const (
	// ScheduleCron uses a standard cron expression.
	ScheduleCron ScheduleKind = "cron"
	// ScheduleAt runs once at a specific time.
	ScheduleAt ScheduleKind = "at"
	// ScheduleEvery runs at fixed intervals.
	ScheduleEvery ScheduleKind = "every"
)

// simpleSession is a simple session implementation for cron jobs.
type simpleSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func (s *simpleSession) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages
}

func (s *simpleSession) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// Job represents a cron job.
type Job struct {
	ID             string       `json:"id"`                         // Unique job ID.
	Name           string       `json:"name"`                       // Human-readable name.
	ScheduleKind   ScheduleKind `json:"schedule_kind"`              // "cron", "at", or "every".
	Schedule       string       `json:"schedule,omitempty"`         // Cron expression (for "cron" kind).
	AtTime         *time.Time   `json:"at_time,omitempty"`          // Target time (for "at" kind).
	EveryDuration  string       `json:"every_duration,omitempty"`   // Duration string (for "every" kind, e.g. "5m", "1h").
	Prompt         string       `json:"prompt"`                     // Task prompt for agent.
	Enabled        bool         `json:"enabled"`                    // Whether job is enabled.
	DeleteAfterRun bool         `json:"delete_after_run,omitempty"` // Auto-delete after execution (for "at" jobs).
	CreatedAt      time.Time    `json:"created_at"`                 // Creation timestamp.
	LastRun        time.Time    `json:"last_run"`                   // Last execution time.
	NextRun        time.Time    `json:"next_run"`                   // Next scheduled run.
	RunCount       int          `json:"run_count"`                  // Total executions.
	LastError      string       `json:"last_error"`                 // Last error message.
	LastSuccess    bool         `json:"last_success"`               // Whether last run succeeded.
}

// Manager manages cron jobs.
type Manager struct {
	log    *logger.Logger
	agent  *agent.Agent
	client *ent.Client

	// Cron scheduler (for "cron" kind jobs).
	scheduler *cron.Cron
	jobs      map[string]*Job         // Job ID -> Job.
	entries   map[string]cron.EntryID // Job ID -> Cron entry ID.
	mu        sync.RWMutex

	// Lifecycle.
	ctx    context.Context
	cancel context.CancelFunc
}

const (
	tickerInterval = 5 * time.Second
)

// New creates a new cron manager.
func New(log *logger.Logger, ag *agent.Agent, client *ent.Client) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		log:       log,
		agent:     ag,
		client:    client,
		scheduler: cron.New(),
		jobs:      make(map[string]*Job),
		entries:   make(map[string]cron.EntryID),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the cron manager.
func (m *Manager) Start() error {
	m.log.Info("Starting cron manager")
	if m.client == nil {
		return fmt.Errorf("runtime ent client is nil")
	}
	if m.agent == nil {
		return fmt.Errorf("agent is nil")
	}

	if err := m.loadJobs(m.ctx); err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	// Schedule loaded jobs.
	for _, job := range m.jobs {
		if !job.Enabled {
			continue
		}
		if err := m.scheduleJob(job); err != nil {
			m.log.Error("Failed to schedule job",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}

	// Start cron scheduler (for cron-type jobs).
	m.scheduler.Start()

	// Start ticker for at/every jobs.
	go m.tickerLoop()

	return nil
}

// Stop stops the cron manager.
func (m *Manager) Stop() error {
	m.log.Info("Stopping cron manager")

	// Stop scheduler.
	ctx := m.scheduler.Stop()
	<-ctx.Done()

	m.cancel()

	m.log.Info("Cron manager stopped")
	return nil
}

// AddJob adds a new cron job (backward compatible: assumes "cron" kind).
func (m *Manager) AddJob(name, schedule, prompt string) (*Job, error) {
	return m.AddCronJob(name, schedule, prompt)
}

// AddCronJob adds a new job with a cron expression schedule.
func (m *Manager) AddCronJob(name, schedule, prompt string) (*Job, error) {
	if _, err := cron.ParseStandard(schedule); err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	job := &Job{
		ID:           generateJobID(),
		Name:         name,
		ScheduleKind: ScheduleCron,
		Schedule:     schedule,
		Prompt:       prompt,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}

	return m.addAndSchedule(job)
}

// AddAtJob adds a one-time job that runs at a specific time.
func (m *Manager) AddAtJob(name string, at time.Time, prompt string, deleteAfterRun bool) (*Job, error) {
	if at.Before(time.Now()) {
		return nil, fmt.Errorf("scheduled time %s is in the past", at.Format(time.RFC3339))
	}

	job := &Job{
		ID:             generateJobID(),
		Name:           name,
		ScheduleKind:   ScheduleAt,
		AtTime:         new(at),
		Prompt:         prompt,
		Enabled:        true,
		DeleteAfterRun: deleteAfterRun,
		CreatedAt:      time.Now(),
		NextRun:        at,
	}

	return m.addAndSchedule(job)
}

// AddEveryJob adds a recurring job that runs at fixed intervals.
func (m *Manager) AddEveryJob(name, every, prompt string) (*Job, error) {
	duration, err := time.ParseDuration(every)
	if err != nil {
		return nil, fmt.Errorf("invalid duration %q: %w", every, err)
	}
	if duration < time.Second {
		return nil, fmt.Errorf("interval must be at least 1 second")
	}

	job := &Job{
		ID:            generateJobID(),
		Name:          name,
		ScheduleKind:  ScheduleEvery,
		EveryDuration: every,
		Prompt:        prompt,
		Enabled:       true,
		CreatedAt:     time.Now(),
		NextRun:       time.Now().Add(duration),
	}

	return m.addAndSchedule(job)
}

func (m *Manager) addAndSchedule(job *Job) (*Job, error) {
	if err := m.createJob(m.ctx, job); err != nil {
		return nil, fmt.Errorf("persisting job: %w", err)
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	if err := m.scheduleJob(job); err != nil {
		m.mu.Lock()
		delete(m.jobs, job.ID)
		m.mu.Unlock()

		if removeErr := m.deleteJob(m.ctx, job.ID); removeErr != nil {
			m.log.Error("Failed to rollback persisted job",
				zap.String("job_id", job.ID),
				zap.Error(removeErr))
		}
		return nil, fmt.Errorf("scheduling job: %w", err)
	}

	if err := m.updateJobState(m.ctx, job); err != nil {
		m.log.Error("Failed to persist scheduled job state", zap.Error(err))
	}

	m.log.Info("Added job",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("kind", string(job.ScheduleKind)))

	return job, nil
}

// RemoveJob removes a cron job.
func (m *Manager) RemoveJob(jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Remove from cron scheduler if it's a cron-type job.
	if entryID, exists := m.entries[jobID]; exists {
		m.scheduler.Remove(entryID)
		delete(m.entries, jobID)
	}

	delete(m.jobs, jobID)
	m.mu.Unlock()

	if err := m.deleteJob(m.ctx, jobID); err != nil {
		return fmt.Errorf("deleting job from storage: %w", err)
	}

	m.log.Info("Removed job",
		zap.String("job_id", jobID),
		zap.String("name", job.Name))

	return nil
}

// EnableJob enables a job.
func (m *Manager) EnableJob(jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Enabled {
		m.mu.Unlock()
		return nil
	}

	job.Enabled = true
	if err := m.scheduleJob(job); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("scheduling job: %w", err)
	}
	jobCopy := *job
	m.mu.Unlock()

	if err := m.updateJobState(m.ctx, &jobCopy); err != nil {
		return fmt.Errorf("persisting enabled job: %w", err)
	}

	m.log.Info("Enabled job", zap.String("job_id", jobID))
	return nil
}

// DisableJob disables a job.
func (m *Manager) DisableJob(jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if !job.Enabled {
		m.mu.Unlock()
		return nil
	}

	job.Enabled = false
	if entryID, exists := m.entries[jobID]; exists {
		m.scheduler.Remove(entryID)
		delete(m.entries, jobID)
	}
	jobCopy := *job
	m.mu.Unlock()

	if err := m.updateJobState(m.ctx, &jobCopy); err != nil {
		return fmt.Errorf("persisting disabled job: %w", err)
	}

	m.log.Info("Disabled job", zap.String("job_id", jobID))
	return nil
}

// ListJobs returns all jobs.
func (m *Manager) ListJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
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

	jobCopy := *job
	return &jobCopy, nil
}

// scheduleJob schedules a job based on its kind.
// Caller must handle locking.
func (m *Manager) scheduleJob(job *Job) error {
	kind := normalizeScheduleKind(job.ScheduleKind)

	switch kind {
	case ScheduleCron:
		return m.scheduleCronJob(job)
	case ScheduleAt:
		// "at" jobs are handled by the ticker loop.
		if job.AtTime != nil {
			job.NextRun = *job.AtTime
		}
		return nil
	case ScheduleEvery:
		// "every" jobs are handled by the ticker loop.
		if job.NextRun.IsZero() {
			d, err := time.ParseDuration(job.EveryDuration)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			job.NextRun = time.Now().Add(d)
		}
		return nil
	default:
		return fmt.Errorf("unknown schedule kind: %s", kind)
	}
}

func (m *Manager) scheduleCronJob(job *Job) error {
	if entryID, exists := m.entries[job.ID]; exists {
		m.scheduler.Remove(entryID)
	}

	entryID, err := m.scheduler.AddFunc(job.Schedule, func() {
		m.executeJob(job.ID)
	})
	if err != nil {
		return err
	}

	m.entries[job.ID] = entryID
	job.NextRun = m.scheduler.Entry(entryID).Next
	return nil
}

// tickerLoop polls for due "at" and "every" jobs.
func (m *Manager) tickerLoop() {
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkTimerJobs()
		}
	}
}

// checkTimerJobs finds and executes due "at" and "every" jobs.
func (m *Manager) checkTimerJobs() {
	now := time.Now()

	m.mu.Lock()
	dueJobIDs := make([]string, 0)
	jobsToPersist := make([]*Job, 0)
	for _, job := range m.jobs {
		if !job.Enabled {
			continue
		}

		kind := normalizeScheduleKind(job.ScheduleKind)
		if kind == ScheduleCron {
			continue
		}

		if job.NextRun.IsZero() || job.NextRun.After(now) {
			continue
		}

		dueJobIDs = append(dueJobIDs, job.ID)
		// Clear NextRun to prevent duplicate execution while the current run is in flight.
		job.NextRun = time.Time{}
		jobCopy := *job
		jobsToPersist = append(jobsToPersist, &jobCopy)
	}
	m.mu.Unlock()

	for _, job := range jobsToPersist {
		if err := m.updateJobState(m.ctx, job); err != nil {
			m.log.Error("Failed to persist timer job lock",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}

	for _, jobID := range dueJobIDs {
		m.executeJob(jobID)
	}
}

// executeJob executes a job.
func (m *Manager) executeJob(jobID string) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	if !exists || !job.Enabled {
		m.mu.RUnlock()
		return
	}
	jobName := job.Name
	prompt := job.Prompt
	m.mu.RUnlock()

	m.log.Info("Executing job",
		zap.String("job_id", jobID),
		zap.String("name", jobName))

	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	defer cancel()

	fullPrompt := fmt.Sprintf(`# Cron Job: %s

Scheduled task execution at %s:

%s`,
		jobName,
		time.Now().Format(time.RFC3339),
		prompt)

	sess := &simpleSession{messages: make([]agent.Message, 0)}
	response, chatErr := m.chatAgent(ctx, sess, fullPrompt)

	finishedAt := time.Now()
	var (
		deleteAfterRun bool
		jobSnapshot    *Job
	)

	m.mu.Lock()
	job, exists = m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return
	}

	job.LastRun = finishedAt
	job.RunCount++
	if chatErr != nil {
		job.LastSuccess = false
		job.LastError = chatErr.Error()
		m.log.Error("Job failed",
			zap.String("job_id", jobID),
			zap.Error(chatErr))
	} else {
		job.LastSuccess = true
		job.LastError = ""
		m.log.Info("Job completed",
			zap.String("job_id", jobID),
			zap.String("response_preview", truncate(response, 100)))
	}

	switch normalizeScheduleKind(job.ScheduleKind) {
	case ScheduleAt:
		if job.DeleteAfterRun {
			if entryID, ok := m.entries[jobID]; ok {
				m.scheduler.Remove(entryID)
				delete(m.entries, jobID)
			}
			delete(m.jobs, jobID)
			deleteAfterRun = true
			m.log.Info("Deleted one-time job after execution", zap.String("job_id", jobID))
		} else {
			job.Enabled = false
			job.NextRun = time.Time{}
		}
	case ScheduleEvery:
		d, parseErr := time.ParseDuration(job.EveryDuration)
		if parseErr != nil {
			m.log.Error("Failed to parse every duration after execution",
				zap.String("job_id", jobID),
				zap.Error(parseErr))
			job.NextRun = time.Time{}
		} else {
			job.NextRun = time.Now().Add(d)
		}
	case ScheduleCron:
		if entryID, ok := m.entries[jobID]; ok {
			job.NextRun = m.scheduler.Entry(entryID).Next
		}
	}

	if !deleteAfterRun {
		copied := *job
		jobSnapshot = &copied
	}
	m.mu.Unlock()

	if deleteAfterRun {
		if err := m.deleteJob(m.ctx, jobID); err != nil {
			m.log.Error("Failed to delete one-time job from storage",
				zap.String("job_id", jobID),
				zap.Error(err))
		}
		return
	}

	if jobSnapshot != nil {
		if err := m.updateJobState(m.ctx, jobSnapshot); err != nil {
			m.log.Error("Failed to persist job execution state",
				zap.String("job_id", jobID),
				zap.Error(err))
		}
	}
}

func (m *Manager) chatAgent(ctx context.Context, sess agent.SessionInterface, prompt string) (response string, err error) {
	if m.agent == nil {
		return "", fmt.Errorf("agent is nil")
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("agent chat panic: %v", recovered)
		}
	}()

	return m.agent.Chat(ctx, sess, prompt)
}

func (m *Manager) loadJobs(ctx context.Context) error {
	entities, err := m.client.CronJob.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("querying cron jobs: %w", err)
	}

	loadedJobs := make(map[string]*Job, len(entities))
	for _, entity := range entities {
		job := jobFromEntity(entity)
		loadedJobs[job.ID] = job
	}

	m.mu.Lock()
	m.jobs = loadedJobs
	m.entries = make(map[string]cron.EntryID, len(loadedJobs))
	m.mu.Unlock()

	m.log.Info("Loaded cron jobs", zap.Int("count", len(entities)))
	return nil
}

func (m *Manager) createJob(ctx context.Context, job *Job) error {
	create := m.client.CronJob.Create().
		SetID(job.ID).
		SetName(job.Name).
		SetScheduleKind(string(normalizeScheduleKind(job.ScheduleKind))).
		SetSchedule(job.Schedule).
		SetEveryDuration(job.EveryDuration).
		SetPrompt(job.Prompt).
		SetEnabled(job.Enabled).
		SetDeleteAfterRun(job.DeleteAfterRun).
		SetRunCount(job.RunCount).
		SetLastError(job.LastError).
		SetLastSuccess(job.LastSuccess)

	if !job.CreatedAt.IsZero() {
		create.SetCreatedAt(job.CreatedAt)
	}
	if job.AtTime != nil {
		create.SetAtTime(*job.AtTime)
	}
	if !job.LastRun.IsZero() {
		create.SetLastRun(job.LastRun)
	}
	if !job.NextRun.IsZero() {
		create.SetNextRun(job.NextRun)
	}

	if _, err := create.Save(ctx); err != nil {
		return fmt.Errorf("creating cron job %s: %w", job.ID, err)
	}
	return nil
}

func (m *Manager) updateJobState(ctx context.Context, job *Job) error {
	update := m.client.CronJob.UpdateOneID(job.ID).
		SetName(job.Name).
		SetScheduleKind(string(normalizeScheduleKind(job.ScheduleKind))).
		SetSchedule(job.Schedule).
		SetEveryDuration(job.EveryDuration).
		SetPrompt(job.Prompt).
		SetEnabled(job.Enabled).
		SetDeleteAfterRun(job.DeleteAfterRun).
		SetRunCount(job.RunCount).
		SetLastError(job.LastError).
		SetLastSuccess(job.LastSuccess)

	if job.AtTime == nil {
		update.ClearAtTime()
	} else {
		update.SetAtTime(*job.AtTime)
	}
	if job.LastRun.IsZero() {
		update.ClearLastRun()
	} else {
		update.SetLastRun(job.LastRun)
	}
	if job.NextRun.IsZero() {
		update.ClearNextRun()
	} else {
		update.SetNextRun(job.NextRun)
	}

	if err := update.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("job not found: %s", job.ID)
		}
		return fmt.Errorf("updating cron job %s: %w", job.ID, err)
	}
	return nil
}

func (m *Manager) deleteJob(ctx context.Context, jobID string) error {
	err := m.client.CronJob.DeleteOneID(jobID).Exec(ctx)
	if err == nil || ent.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("deleting cron job %s: %w", jobID, err)
}

func jobFromEntity(entity *ent.CronJob) *Job {
	job := &Job{
		ID:             entity.ID,
		Name:           entity.Name,
		ScheduleKind:   normalizeScheduleKind(ScheduleKind(entity.ScheduleKind)),
		Schedule:       entity.Schedule,
		EveryDuration:  entity.EveryDuration,
		Prompt:         entity.Prompt,
		Enabled:        entity.Enabled,
		DeleteAfterRun: entity.DeleteAfterRun,
		CreatedAt:      entity.CreatedAt,
		RunCount:       entity.RunCount,
		LastError:      entity.LastError,
		LastSuccess:    entity.LastSuccess,
	}
	if entity.AtTime != nil {
		at := *entity.AtTime
		job.AtTime = &at
	}
	if entity.LastRun != nil {
		job.LastRun = *entity.LastRun
	}
	if entity.NextRun != nil {
		job.NextRun = *entity.NextRun
	}
	return job
}

func normalizeScheduleKind(kind ScheduleKind) ScheduleKind {
	if kind == "" {
		return ScheduleCron
	}
	return kind
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
