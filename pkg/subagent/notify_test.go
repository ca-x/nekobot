package subagent

import (
	"strings"
	"testing"
	"time"
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
