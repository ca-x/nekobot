package tools

import (
	"context"
	"testing"
	"time"

	"nekobot/pkg/logger"
	"nekobot/pkg/subagent"
)

type spawnTestAgent struct{}

func (a *spawnTestAgent) Chat(ctx context.Context, message string) (string, error) {
	return "ok", nil
}

func TestSpawnToolUsesContextRouteForSpawnedTask(t *testing.T) {
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Level = logger.LevelError
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	manager := subagent.NewSubagentManager(log, &spawnTestAgent{}, 1)
	defer manager.Stop()

	tool := NewSpawnTool(log, manager)
	ctx := WithSpawnContext(context.Background(), "wechat", "wechat:user-1")

	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action": "spawn",
		"task":   "check status",
		"label":  "status-check",
	}); err != nil {
		t.Fatalf("Execute spawn failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		tasks := manager.ListTasks()
		if len(tasks) == 1 {
			task := tasks[0]
			if task.Channel != "wechat" {
				t.Fatalf("expected channel %q, got %q", "wechat", task.Channel)
			}
			if task.ChatID != "wechat:user-1" {
				t.Fatalf("expected chat id %q, got %q", "wechat:user-1", task.ChatID)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for spawned task")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
