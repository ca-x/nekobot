package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

// Agent interface defines minimal agent capabilities needed by SubagentManager.
// This avoids circular imports (agent -> tools -> agent).
type Agent interface {
	Chat(ctx context.Context, message string) (string, error)
}

// SubagentTask represents a task being executed by a subagent.
type SubagentTask struct {
	ID          string
	Label       string
	Task        string
	Status      tasks.State
	Result      string
	Error       error
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Channel     string // Origin channel
	ChatID      string // Origin chat ID
}

// NotifyFunc is called when a task completes or fails. It receives the task
// so the caller can route the notification to the origin channel.
type NotifyFunc func(task *SubagentTask)

// Notification contains a rendered subagent completion message.
type Notification struct {
	ID        string
	Channel   string
	ChatID    string
	Content   string
	Data      map[string]interface{}
	Timestamp time.Time
}

// OutboundSender sends notification messages to origin channels.
type OutboundSender interface {
	SendNotification(msg *Notification) error
}

type taskLifecycle interface {
	Enqueue(task tasks.Task) (tasks.Task, error)
	Claim(taskID, runtimeID string) (tasks.Task, error)
	Start(taskID string) (tasks.Task, error)
	Complete(taskID string) (tasks.Task, error)
	Fail(taskID, lastError string) (tasks.Task, error)
	Cancel(taskID string) (tasks.Task, error)
}

// SubagentManager manages subagent task execution.
type SubagentManager struct {
	log        *logger.Logger
	agent      Agent // Use interface instead of concrete type
	tasks      map[string]*SubagentTask
	mu         sync.RWMutex
	maxTasks   int
	taskQueue  chan *SubagentTask
	onComplete NotifyFunc
	taskSvc    taskLifecycle
}

// NewSubagentManager creates a new subagent manager.
func NewSubagentManager(log *logger.Logger, agent Agent, maxTasks int) *SubagentManager {
	if maxTasks <= 0 {
		maxTasks = 10 // Default max concurrent tasks
	}

	sm := &SubagentManager{
		log:       log,
		agent:     agent,
		tasks:     make(map[string]*SubagentTask),
		maxTasks:  maxTasks,
		taskQueue: make(chan *SubagentTask, maxTasks),
	}

	// Start workers
	for i := 0; i < maxTasks; i++ {
		go sm.worker(i)
	}

	return sm
}

// Spawn spawns a new subagent task.
func (sm *SubagentManager) Spawn(ctx context.Context, task, label, channel, chatID string) (string, error) {
	taskID := uuid.New().String()

	if label == "" {
		label = taskID[:8]
	}

	sm.log.Info("Spawning subagent",
		zap.String("task_id", taskID),
		zap.String("label", label),
		zap.String("task", task[:min(len(task), 100)]))

	subagentTask := &SubagentTask{
		ID:        taskID,
		Label:     label,
		Task:      task,
		Status:    tasks.StatePending,
		CreatedAt: time.Now(),
		Channel:   channel,
		ChatID:    chatID,
	}

	sm.mu.Lock()
	sm.tasks[taskID] = subagentTask
	sm.mu.Unlock()
	sm.enqueueManagedTask(subagentTask)

	// Queue task for execution
	select {
	case sm.taskQueue <- subagentTask:
		sm.log.Info("Task queued", zap.String("task_id", taskID))
	default:
		sm.mu.Lock()
		subagentTask.Status = tasks.StateFailed
		subagentTask.Error = fmt.Errorf("task queue full")
		subagentTask.CompletedAt = time.Now()
		sm.mu.Unlock()
		sm.failManagedTask(subagentTask, subagentTask.Error)
		return "", fmt.Errorf("task queue full, max %d concurrent tasks", sm.maxTasks)
	}

	return taskID, nil
}

// GetTask retrieves a task by ID.
func (sm *SubagentManager) GetTask(taskID string) (*SubagentTask, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks lists all tasks.
func (sm *SubagentManager) ListTasks() []*SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tasks := make([]*SubagentTask, 0, len(sm.tasks))
	for _, task := range sm.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// CancelTask cancels a pending or running task.
func (sm *SubagentManager) CancelTask(taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if tasks.IsFinal(task.Status) {
		return fmt.Errorf("task already %s", task.Status)
	}

	task.Status = tasks.StateFailed
	task.Error = fmt.Errorf("cancelled")
	task.CompletedAt = time.Now()
	sm.cancelManagedTask(task)

	return nil
}

// worker processes tasks from the queue.
func (sm *SubagentManager) worker(workerID int) {
	sm.log.Info("Subagent worker started", zap.Int("worker_id", workerID))

	for task := range sm.taskQueue {
		sm.executeTask(task)
	}
}

// executeTask executes a single task.
func (sm *SubagentManager) executeTask(task *SubagentTask) {
	sm.mu.Lock()
	task.Status = tasks.StateRunning
	task.StartedAt = time.Now()
	sm.mu.Unlock()
	sm.startManagedTask(task)

	sm.log.Info("Executing subagent task",
		zap.String("task_id", task.ID),
		zap.String("label", task.Label))

	// Create isolated context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := sm.agent.Chat(ctx, task.Task)

	sm.mu.Lock()
	task.CompletedAt = time.Now()
	if err != nil {
		task.Status = tasks.StateFailed
		task.Error = err
		sm.log.Error("Subagent task failed",
			zap.String("task_id", task.ID),
			zap.Error(err))
	} else {
		task.Status = tasks.StateCompleted
		task.Result = result
		sm.log.Info("Subagent task completed",
			zap.String("task_id", task.ID))
	}
	sm.mu.Unlock()
	if err != nil {
		sm.failManagedTask(task, err)
	} else {
		sm.completeManagedTask(task)
	}

	// Notify origin channel if callback is configured
	if sm.onComplete != nil {
		sm.onComplete(task)
	}
}

// SetNotifyFunc sets the callback invoked when a task completes or fails.
func (sm *SubagentManager) SetNotifyFunc(fn NotifyFunc) {
	sm.onComplete = fn
}

// SetTaskService attaches a shared task lifecycle service to the subagent manager.
func (sm *SubagentManager) SetTaskService(svc taskLifecycle) {
	if sm == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.taskSvc = svc
}

// HasTaskService reports whether the manager has a shared task lifecycle service.
func (sm *SubagentManager) HasTaskService() bool {
	if sm == nil {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.taskSvc != nil
}

// Stop stops all workers by closing the task queue.
func (sm *SubagentManager) Stop() {
	if sm == nil {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.taskQueue == nil {
		return
	}

	close(sm.taskQueue)
	sm.taskQueue = nil
}

// PruneTasks removes completed tasks older than the specified duration.
func (sm *SubagentManager) PruneTasks(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	pruned := 0

	for id, task := range sm.tasks {
		if tasks.IsFinal(task.Status) && task.CompletedAt.Before(cutoff) {
			delete(sm.tasks, id)
			pruned++
		}
	}

	sm.log.Info("Pruned subagent tasks",
		zap.Int("count", pruned))

	return pruned
}

// Snapshot returns the shared task model view for this subagent task.
func (t *SubagentTask) Snapshot() tasks.Task {
	if t == nil {
		return tasks.Task{}
	}

	snapshot := tasks.Task{
		ID:          t.ID,
		Type:        tasks.TypeBackgroundAgent,
		State:       t.Status,
		Summary:     t.Task,
		SessionID:   t.ChatID,
		CreatedAt:   t.CreatedAt,
		StartedAt:   t.StartedAt,
		CompletedAt: t.CompletedAt,
		Metadata: map[string]any{
			"label":   t.Label,
			"channel": t.Channel,
		},
	}
	if t.Error != nil {
		snapshot.LastError = t.Error.Error()
	}
	if t.Status == tasks.StateRequiresAction {
		snapshot.PendingAction = "manual_intervention"
	}
	return snapshot
}

// GetTaskSnapshot retrieves one task as the shared task model.
func (sm *SubagentManager) GetTaskSnapshot(taskID string) (tasks.Task, error) {
	task, err := sm.GetTask(taskID)
	if err != nil {
		return tasks.Task{}, err
	}
	return task.Snapshot(), nil
}

// ListTaskSnapshots lists all tasks as shared task model snapshots.
func (sm *SubagentManager) ListTaskSnapshots() []tasks.Task {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]tasks.Task, 0, len(sm.tasks))
	for _, task := range sm.tasks {
		result = append(result, task.Snapshot())
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SendTaskNotification emits a text notification for a finished task.
func SendTaskNotification(sender OutboundSender, task *SubagentTask) error {
	if sender == nil {
		return fmt.Errorf("outbound sender is required")
	}
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if task.Channel == "" {
		return fmt.Errorf("task %s missing origin channel", task.ID)
	}
	if task.ChatID == "" {
		return fmt.Errorf("task %s missing origin chat id", task.ID)
	}

	msg := &Notification{
		ID:      "subagent:" + task.ID,
		Channel: task.Channel,
		ChatID:  task.ChatID,
		Content: formatTaskNotification(task),
		Data: map[string]interface{}{
			"task_id":   task.ID,
			"status":    string(task.Status),
			"label":     task.Label,
			"task_type": string(tasks.TypeBackgroundAgent),
		},
		Timestamp: time.Now(),
	}

	return sender.SendNotification(msg)
}

func formatTaskNotification(task *SubagentTask) string {
	label := task.Label
	if label == "" {
		label = task.ID
	}

	switch task.Status {
	case tasks.StateCompleted:
		result := task.Result
		if result == "" {
			result = "(no result)"
		}
		return fmt.Sprintf("Subagent task [%s] completed.\n%s", label, result)
	case tasks.StateFailed:
		errText := "unknown error"
		if task.Error != nil {
			errText = task.Error.Error()
		}
		return fmt.Sprintf("Subagent task [%s] failed.\n%s", label, errText)
	default:
		return fmt.Sprintf("Subagent task [%s] status changed: %s", label, task.Status)
	}
}

func (sm *SubagentManager) enqueueManagedTask(task *SubagentTask) {
	svc := sm.lifecycleService()
	if svc == nil || task == nil {
		return
	}
	if _, err := svc.Enqueue(task.managedTask()); err != nil {
		sm.log.Warn("Failed to enqueue managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
	}
}

func (sm *SubagentManager) startManagedTask(task *SubagentTask) {
	svc := sm.lifecycleService()
	if svc == nil || task == nil {
		return
	}
	if _, err := svc.Claim(task.ID, "subagent"); err != nil {
		sm.log.Warn("Failed to claim managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
		return
	}
	if _, err := svc.Start(task.ID); err != nil {
		sm.log.Warn("Failed to start managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
	}
}

func (sm *SubagentManager) completeManagedTask(task *SubagentTask) {
	svc := sm.lifecycleService()
	if svc == nil || task == nil {
		return
	}
	if _, err := svc.Complete(task.ID); err != nil {
		sm.log.Warn("Failed to complete managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
	}
}

func (sm *SubagentManager) failManagedTask(task *SubagentTask, taskErr error) {
	svc := sm.lifecycleService()
	if svc == nil || task == nil || taskErr == nil {
		return
	}
	if _, err := svc.Fail(task.ID, taskErr.Error()); err != nil {
		sm.log.Warn("Failed to fail managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
	}
}

func (sm *SubagentManager) cancelManagedTask(task *SubagentTask) {
	svc := sm.lifecycleService()
	if svc == nil || task == nil {
		return
	}
	if _, err := svc.Cancel(task.ID); err != nil {
		sm.log.Warn("Failed to cancel managed subagent task", zap.String("task_id", task.ID), zap.Error(err))
	}
}

func (sm *SubagentManager) lifecycleService() taskLifecycle {
	if sm == nil {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.taskSvc
}

func (t *SubagentTask) managedTask() tasks.Task {
	if t == nil {
		return tasks.Task{}
	}
	return tasks.Task{
		ID:        t.ID,
		Type:      tasks.TypeBackgroundAgent,
		Summary:   t.Task,
		SessionID: t.ChatID,
		CreatedAt: t.CreatedAt,
		Metadata: map[string]any{
			"label":   t.Label,
			"channel": t.Channel,
		},
	}
}
