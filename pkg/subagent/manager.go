package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Agent interface defines minimal agent capabilities needed by SubagentManager.
// This avoids circular imports (agent -> tools -> agent).
type Agent interface {
	Chat(ctx context.Context, sess Session, message string) (string, error)
}

// Session defines the minimal conversation state required by subagent tasks.
type Session interface {
	GetMessages() []Message
	AddMessage(Message)
}

// Message represents a role/content chat turn used by subagent sessions.
type Message struct {
	Role    string
	Content string
}

// SubagentTask represents a task being executed by a subagent.
type SubagentTask struct {
	ID          string
	Label       string
	Task        string
	Status      string // "pending", "running", "completed", "failed"
	Result      string
	Error       error
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Channel     string // Origin channel
	ChatID      string // Origin chat ID
}

// SubagentManager manages subagent task execution.
type SubagentManager struct {
	log       *logger.Logger
	agent     Agent // Use interface instead of concrete type
	tasks     map[string]*SubagentTask
	mu        sync.RWMutex
	maxTasks  int
	taskQueue chan *SubagentTask
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
		Status:    "pending",
		CreatedAt: time.Now(),
		Channel:   channel,
		ChatID:    chatID,
	}

	sm.mu.Lock()
	sm.tasks[taskID] = subagentTask
	sm.mu.Unlock()

	// Queue task for execution
	select {
	case sm.taskQueue <- subagentTask:
		sm.log.Info("Task queued", zap.String("task_id", taskID))
	default:
		sm.mu.Lock()
		subagentTask.Status = "failed"
		subagentTask.Error = fmt.Errorf("task queue full")
		sm.mu.Unlock()
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

	if task.Status == "completed" || task.Status == "failed" {
		return fmt.Errorf("task already %s", task.Status)
	}

	task.Status = "failed"
	task.Error = fmt.Errorf("cancelled")
	task.CompletedAt = time.Now()

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
	task.Status = "running"
	task.StartedAt = time.Now()
	sm.mu.Unlock()

	sm.log.Info("Executing subagent task",
		zap.String("task_id", task.ID),
		zap.String("label", task.Label))

	// Create isolated context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sess := &taskSession{messages: make([]Message, 0, 2)}
	result, err := sm.agent.Chat(ctx, sess, task.Task)

	sm.mu.Lock()
	task.CompletedAt = time.Now()
	if err != nil {
		task.Status = "failed"
		task.Error = err
		sm.log.Error("Subagent task failed",
			zap.String("task_id", task.ID),
			zap.Error(err))
	} else {
		task.Status = "completed"
		task.Result = result
		sm.log.Info("Subagent task completed",
			zap.String("task_id", task.ID))
	}
	sm.mu.Unlock()

	// TODO: Send notification to origin channel if configured
}

// PruneTasks removes completed tasks older than the specified duration.
func (sm *SubagentManager) PruneTasks(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	pruned := 0

	for id, task := range sm.tasks {
		if (task.Status == "completed" || task.Status == "failed") &&
			task.CompletedAt.Before(cutoff) {
			delete(sm.tasks, id)
			pruned++
		}
	}

	sm.log.Info("Pruned subagent tasks",
		zap.Int("count", pruned))

	return pruned
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type taskSession struct {
	messages []Message
}

func (s *taskSession) GetMessages() []Message {
	return s.messages
}

func (s *taskSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}
