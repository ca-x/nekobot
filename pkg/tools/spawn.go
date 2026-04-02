package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
	"nekobot/pkg/subagent"
	"nekobot/pkg/tasks"
)

// SpawnTool provides subagent spawning for async task execution.
type SpawnTool struct {
	log         *logger.Logger
	manager     *subagent.SubagentManager
	currentChan string
	currentChat string
}

type spawnContextKey string

const (
	spawnContextChannelKey spawnContextKey = "channel"
	spawnContextChatKey    spawnContextKey = "chat"
)

// NewSpawnTool creates a new spawn tool.
func NewSpawnTool(log *logger.Logger, manager *subagent.SubagentManager) *SpawnTool {
	return &SpawnTool{
		log:     log,
		manager: manager,
	}
}

// WithSpawnContext stores origin routing for the spawn tool.
func WithSpawnContext(ctx context.Context, channel, chatID string) context.Context {
	ctx = context.WithValue(ctx, spawnContextChannelKey, strings.TrimSpace(channel))
	return context.WithValue(ctx, spawnContextChatKey, strings.TrimSpace(chatID))
}

// SetCurrent sets the current channel and chat context.
func (t *SpawnTool) SetCurrent(channel, chatID string) {
	t.currentChan = channel
	t.currentChat = chatID
}

// Name returns the tool name.
func (t *SpawnTool) Name() string {
	return "spawn"
}

// Description returns the tool description.
func (t *SpawnTool) Description() string {
	return `Spawn a subagent to execute a task in the background. Use this for:
- Long-running tasks that would block the main conversation
- Tasks that can be delegated and checked later
- Parallel execution of independent tasks

The subagent runs independently and you can check its status later using the task ID.`
}

// Parameters returns the tool parameters schema.
func (t *SpawnTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"spawn", "status", "list", "cancel"},
				"description": "Action to perform: spawn, status, list, or cancel",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Task description for the subagent (required for spawn)",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Optional label for the task (for spawn)",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "Task ID (required for status and cancel)",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the spawn tool.
func (t *SpawnTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	if t.manager == nil {
		return "", fmt.Errorf("subagent manager not initialized")
	}

	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter is required")
	}

	switch action {
	case "spawn":
		return t.spawn(ctx, params)
	case "status":
		return t.status(params)
	case "list":
		return t.list()
	case "cancel":
		return t.cancel(params)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// spawn spawns a new subagent task.
func (t *SpawnTool) spawn(ctx context.Context, params map[string]interface{}) (string, error) {
	task, ok := params["task"].(string)
	if !ok || task == "" {
		return "", fmt.Errorf("task parameter is required for spawn")
	}

	label, _ := params["label"].(string)

	t.log.Info("Spawning subagent",
		zap.String("task", task[:min(len(task), 100)]),
		zap.String("label", label))

	channel, chatID := t.routeFromContext(ctx)
	taskID, err := t.manager.Spawn(ctx, task, label, channel, chatID)
	if err != nil {
		return "", fmt.Errorf("failed to spawn subagent: %w", err)
	}

	return fmt.Sprintf("Subagent spawned successfully\nTask ID: %s\nLabel: %s\nStatus: pending\n\nUse `spawn status %s` to check progress.",
		taskID, label, taskID), nil
}

// status checks the status of a task.
func (t *SpawnTool) status(params map[string]interface{}) (string, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return "", fmt.Errorf("task_id parameter is required for status")
	}

	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Task ID: %s\n", task.ID)
	_, _ = fmt.Fprintf(&sb, "Label: %s\n", task.Label)
	_, _ = fmt.Fprintf(&sb, "Status: %s\n", task.Status)
	_, _ = fmt.Fprintf(&sb, "Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))

	if !task.StartedAt.IsZero() {
		_, _ = fmt.Fprintf(&sb, "Started: %s\n", task.StartedAt.Format("2006-01-02 15:04:05"))
	}

	if !task.CompletedAt.IsZero() {
		_, _ = fmt.Fprintf(&sb, "Completed: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
		duration := task.CompletedAt.Sub(task.StartedAt)
		_, _ = fmt.Fprintf(&sb, "Duration: %s\n", duration.Round(time.Second))
	}

	_, _ = fmt.Fprintf(&sb, "\nTask: %s\n", task.Task)

	if task.Status == tasks.StateCompleted && task.Result != "" {
		_, _ = fmt.Fprintf(&sb, "\nResult:\n%s\n", task.Result)
	}

	if task.Error != nil {
		_, _ = fmt.Fprintf(&sb, "\nError: %s\n", task.Error)
	}

	return sb.String(), nil
}

// list lists all tasks.
func (t *SpawnTool) list() (string, error) {
	taskList := t.manager.ListTasks()

	if len(taskList) == 0 {
		return "No subagent tasks found", nil
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Subagent Tasks (%d total):\n\n", len(taskList))

	// Group by status
	statuses := map[tasks.State][]*subagent.SubagentTask{
		tasks.StatePending:   {},
		tasks.StateRunning:   {},
		tasks.StateCompleted: {},
		tasks.StateFailed:    {},
	}

	for _, task := range taskList {
		statuses[task.Status] = append(statuses[task.Status], task)
	}

	// Display by status
	for _, status := range []tasks.State{tasks.StateRunning, tasks.StatePending, tasks.StateCompleted, tasks.StateFailed} {
		taskList := statuses[status]
		if len(taskList) == 0 {
			continue
		}

		_, _ = fmt.Fprintf(&sb, "## %s (%d)\n\n", strings.ToUpper(string(status)), len(taskList))

		for _, task := range taskList {
			_, _ = fmt.Fprintf(&sb, "- [%s] %s (ID: %s)\n",
				task.Label, task.Task[:min(len(task.Task), 60)], task.ID[:8])

			if tasks.IsFinal(task.Status) {
				duration := task.CompletedAt.Sub(task.StartedAt)
				_, _ = fmt.Fprintf(&sb, "  Completed in %s\n", duration.Round(time.Second))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// cancel cancels a task.
func (t *SpawnTool) cancel(params map[string]interface{}) (string, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return "", fmt.Errorf("task_id parameter is required for cancel")
	}

	if err := t.manager.CancelTask(taskID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Task %s cancelled successfully", taskID), nil
}

func (t *SpawnTool) routeFromContext(ctx context.Context) (string, string) {
	channel := strings.TrimSpace(t.currentChan)
	chatID := strings.TrimSpace(t.currentChat)

	if ctx == nil {
		return channel, chatID
	}

	if value, ok := ctx.Value(spawnContextChannelKey).(string); ok && strings.TrimSpace(value) != "" {
		channel = strings.TrimSpace(value)
	}
	if value, ok := ctx.Value(spawnContextChatKey).(string); ok && strings.TrimSpace(value) != "" {
		chatID = strings.TrimSpace(value)
	}

	return channel, chatID
}
