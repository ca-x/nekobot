package tools

import (
	"context"
	"strings"
	"sync"
	"time"
)

// StreamingUpdate represents a real-time output update.
type StreamingUpdate struct {
	SessionID string `json:"session_id"`
	Output    string `json:"output"`
	Lines     int    `json:"lines"`
	Done      bool   `json:"done"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Error     string `json:"error,omitempty"`
}

// StreamingHandler receives real-time output updates.
type StreamingHandler func(update StreamingUpdate)

// streamingKey is the context key for streaming handlers.
type streamingKey struct{}
type streamingSessionIDKey struct{}

// WithStreamingHandler attaches a streaming handler to the context.
func WithStreamingHandler(ctx context.Context, handler StreamingHandler) context.Context {
	return context.WithValue(ctx, streamingKey{}, handler)
}

// WithStreamingSessionID attaches a session ID for streaming updates to the context.
func WithStreamingSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, streamingSessionIDKey{}, sessionID)
}

// GetStreamingHandler retrieves the streaming handler from context.
func GetStreamingHandler(ctx context.Context) StreamingHandler {
	if handler, ok := ctx.Value(streamingKey{}).(StreamingHandler); ok {
		return handler
	}
	return nil
}

// GetStreamingSessionID retrieves the streaming session ID from context.
func GetStreamingSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value(streamingSessionIDKey{}).(string); ok {
		return sessionID
	}
	return ""
}

// StreamWriter provides real-time output streaming for commands.
type StreamWriter struct {
	mu        sync.Mutex
	sessionID string
	output    strings.Builder
	lines     int
	handler   StreamingHandler
	interval  time.Duration
	lastSend  time.Time
	done      bool
}

// NewStreamWriter creates a new streaming writer.
func NewStreamWriter(handler StreamingHandler, sessionID string, interval time.Duration) *StreamWriter {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &StreamWriter{
		sessionID: sessionID,
		handler:   handler,
		interval:  interval,
	}
}

// Write implements io.Writer for streaming output.
func (sw *StreamWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.done {
		return 0, nil
	}

	n, err = sw.output.Write(p)
	sw.lines += countLines(string(p))

	// Send update if interval elapsed
	if time.Since(sw.lastSend) >= sw.interval {
		sw.sendUpdate()
	}

	return n, err
}

// WriteString writes a string and potentially sends an update.
func (sw *StreamWriter) WriteString(s string) (n int, err error) {
	return sw.Write([]byte(s))
}

// Flush sends any pending output.
func (sw *StreamWriter) Flush() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.sendUpdate()
}

// Finish marks the stream as done and sends final update.
func (sw *StreamWriter) Finish(exitCode int, errMsg string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.done = true
	if sw.handler != nil {
		update := StreamingUpdate{
			SessionID: sw.sessionID,
			Output:    sw.output.String(),
			Lines:     sw.lines,
			Done:      true,
			ExitCode:  exitCode,
			Error:     errMsg,
		}
		sw.handler(update)
	}
}

// Output returns the complete output.
func (sw *StreamWriter) Output() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.output.String()
}

// Lines returns the line count.
func (sw *StreamWriter) Lines() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.lines
}

func (sw *StreamWriter) sendUpdate() {
	if sw.handler == nil || sw.done {
		return
	}
	sw.lastSend = time.Now()
	update := StreamingUpdate{
		SessionID: sw.sessionID,
		Output:    sw.output.String(),
		Lines:     sw.lines,
		Done:      false,
	}
	sw.handler(update)
}

func countLines(s string) int {
	count := 0
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}
