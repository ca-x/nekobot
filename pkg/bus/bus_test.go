package bus

import (
	"context"
	"testing"
	"time"

	"nekobot/pkg/logger"
)

func TestLocalBus(t *testing.T) {
	// Create logger
	log, err := logger.New(&logger.Config{
		Level:  "info",
		Output: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create bus
	bus := NewLocalBus(log, 10)
	if err := bus.Start(); err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	// Register handler
	received := make(chan *Message, 1)
	bus.RegisterHandler("test", func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})

	// Send message
	testMsg := &Message{
		ID:        "test-1",
		ChannelID: "test",
		SessionID: "session-1",
		UserID:    "user-1",
		Username:  "testuser",
		Type:      MessageTypeText,
		Content:   "Hello, world!",
		Timestamp: time.Now(),
	}

	if err := bus.SendInbound(testMsg); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.ID != testMsg.ID {
			t.Errorf("Expected message ID %s, got %s", testMsg.ID, msg.ID)
		}
		if msg.Content != testMsg.Content {
			t.Errorf("Expected content %s, got %s", testMsg.Content, msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Check metrics
	metrics := bus.GetMetrics()
	if metrics["messages_in"] != 1 {
		t.Errorf("Expected 1 inbound message, got %d", metrics["messages_in"])
	}
}

func TestBusMultipleHandlers(t *testing.T) {
	log, _ := logger.New(&logger.Config{Level: "error", Output: "console"})
	bus := NewLocalBus(log, 10)
	bus.Start()
	defer bus.Stop()

	// Register multiple handlers
	count := 0
	bus.RegisterHandler("test", func(ctx context.Context, msg *Message) error {
		count++
		return nil
	})
	bus.RegisterHandler("test", func(ctx context.Context, msg *Message) error {
		count++
		return nil
	})

	// Send message
	testMsg := &Message{
		ID:        "test-1",
		ChannelID: "test",
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	bus.SendInbound(testMsg)
	time.Sleep(100 * time.Millisecond)

	if count != 2 {
		t.Errorf("Expected 2 handler calls, got %d", count)
	}
}

func TestBusOutbound(t *testing.T) {
	log, _ := logger.New(&logger.Config{Level: "error", Output: "console"})
	bus := NewLocalBus(log, 10)
	bus.Start()
	defer bus.Stop()

	received := make(chan *Message, 1)
	bus.RegisterHandler("test", func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})

	testMsg := &Message{
		ID:        "test-1",
		ChannelID: "test",
		Content:   "Response",
		Timestamp: time.Now(),
	}

	bus.SendOutbound(testMsg)

	select {
	case msg := <-received:
		if msg.Content != "Response" {
			t.Errorf("Expected content 'Response', got %s", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for outbound message")
	}

	metrics := bus.GetMetrics()
	if metrics["messages_out"] != 1 {
		t.Errorf("Expected 1 outbound message, got %d", metrics["messages_out"])
	}
}

func BenchmarkBusThroughput(b *testing.B) {
	log, _ := logger.New(&logger.Config{Level: "error", Output: "console"})
	bus := NewLocalBus(log, 1000)
	bus.Start()
	defer bus.Stop()

	// Register no-op handler
	bus.RegisterHandler("test", func(ctx context.Context, msg *Message) error {
		return nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			msg := &Message{
				ID:        time.Now().Format("20060102150405.000000"),
				ChannelID: "test",
				Content:   "Benchmark message",
				Timestamp: time.Now(),
			}
			bus.SendInbound(msg)
			i++
		}
	})
}
