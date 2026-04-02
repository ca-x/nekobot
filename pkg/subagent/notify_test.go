package subagent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/tasks"
)

type outboundRecorder struct {
	messages []*Notification
}

func (r *outboundRecorder) SendNotification(msg *Notification) error {
	r.messages = append(r.messages, msg)
	return nil
}

func TestSendTaskNotificationSendsOutboundMessageForCompletedTask(t *testing.T) {
	recorder := &outboundRecorder{}
	task := &SubagentTask{
		ID:          "task-1",
		Label:       "research",
		Task:        "collect findings",
		Status:      "completed",
		Result:      "final answer",
		Channel:     "telegram",
		ChatID:      "telegram:42",
		CompletedAt: time.Unix(1_700_000_000, 0),
	}

	if err := SendTaskNotification(recorder, task); err != nil {
		t.Fatalf("SendTaskNotification failed: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("expected 1 outbound message, got %d", len(recorder.messages))
	}

	msg := recorder.messages[0]
	if msg.Channel != "telegram" {
		t.Fatalf("expected channel %q, got %q", "telegram", msg.Channel)
	}
	if msg.ChatID != "telegram:42" {
		t.Fatalf("expected session %q, got %q", "telegram:42", msg.ChatID)
	}
	if !strings.Contains(msg.Content, "research") {
		t.Fatalf("expected content to include task label, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "final answer") {
		t.Fatalf("expected content to include task result, got %q", msg.Content)
	}
	if msg.Data["task_id"] != "task-1" {
		t.Fatalf("expected task_id metadata, got %#v", msg.Data["task_id"])
	}
	if msg.Data["status"] != "completed" {
		t.Fatalf("expected status metadata, got %#v", msg.Data["status"])
	}
	if msg.Data["task_type"] != string(tasks.TypeBackgroundAgent) {
		t.Fatalf("expected task_type metadata, got %#v", msg.Data["task_type"])
	}
}

func TestSendTaskNotificationRequiresOriginRoute(t *testing.T) {
	recorder := &outboundRecorder{}
	task := &SubagentTask{
		ID:      "task-2",
		Label:   "orphan",
		Status:  "completed",
		Result:  "done",
		Channel: "",
		ChatID:  "telegram:42",
	}

	if err := SendTaskNotification(recorder, task); err == nil {
		t.Fatal("expected SendTaskNotification to fail without origin channel")
	}
}

func TestSubagentTaskSnapshotMapsToSharedTaskModel(t *testing.T) {
	completedAt := time.Unix(1_700_000_100, 0)
	task := &SubagentTask{
		ID:          "task-3",
		Label:       "planner",
		Task:        "draft a rollout plan",
		Status:      tasks.StateFailed,
		Error:       fmt.Errorf("planner crashed"),
		CreatedAt:   time.Unix(1_700_000_000, 0),
		StartedAt:   time.Unix(1_700_000_010, 0),
		CompletedAt: completedAt,
		Channel:     "websocket",
		ChatID:      "webui-chat:alice",
	}

	snapshot := task.Snapshot()
	if snapshot.ID != "task-3" {
		t.Fatalf("unexpected snapshot id: %q", snapshot.ID)
	}
	if snapshot.Type != tasks.TypeBackgroundAgent {
		t.Fatalf("unexpected task type: %q", snapshot.Type)
	}
	if snapshot.State != tasks.StateFailed {
		t.Fatalf("unexpected task state: %q", snapshot.State)
	}
	if snapshot.Summary != "draft a rollout plan" {
		t.Fatalf("unexpected summary: %q", snapshot.Summary)
	}
	if snapshot.SessionID != "webui-chat:alice" {
		t.Fatalf("unexpected session id: %q", snapshot.SessionID)
	}
	if snapshot.LastError != "planner crashed" {
		t.Fatalf("unexpected last error: %q", snapshot.LastError)
	}
	if snapshot.CompletedAt != completedAt {
		t.Fatalf("unexpected completed time: %v", snapshot.CompletedAt)
	}
	if snapshot.Metadata["label"] != "planner" {
		t.Fatalf("unexpected metadata label: %#v", snapshot.Metadata["label"])
	}
	if snapshot.Metadata["channel"] != "websocket" {
		t.Fatalf("unexpected metadata channel: %#v", snapshot.Metadata["channel"])
	}
}
