// Package message provides shared message types used across nekobot packages.
package message

// MessageID is a unique identifier for a message.
type MessageID int

// Message represents a single message in a conversation.
type Message struct {
	ID          string       `json:"id,omitempty"`
	Role        string       `json:"role"` // "system", "user", "assistant", "tool"
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID  string       `json:"tool_call_id,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Attachment records a file bound to a persisted chat message.
type Attachment struct {
	AttachmentID    string `json:"attachment_id"`
	Target          string `json:"target"`
	OwnerID         string `json:"owner_id,omitempty"`
	Filename        string `json:"filename"`
	MimeType        string `json:"mime_type,omitempty"`
	SizeBytes       int64  `json:"size_bytes"`
	StorageRef      string `json:"storage_ref,omitempty"`
	CreatedTimeUnix int64  `json:"created_time_unix,omitempty"`
}
