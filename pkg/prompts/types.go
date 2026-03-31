package prompts

import "time"

const (
	ModeSystem = "system"
	ModeUser   = "user"

	ScopeGlobal  = "global"
	ScopeChannel = "channel"
	ScopeSession = "session"
)

// Prompt defines a reusable prompt template.
type Prompt struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Mode        string    `json:"mode"`
	Template    string    `json:"template"`
	Enabled     bool      `json:"enabled"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Binding defines where a prompt applies.
type Binding struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"`
	Target    string    `json:"target"`
	PromptID  string    `json:"prompt_id"`
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolveInput describes a runtime prompt resolution request.
type ResolveInput struct {
	Channel           string         `json:"channel"`
	SessionID         string         `json:"session_id"`
	UserID            string         `json:"user_id"`
	Username          string         `json:"username"`
	RequestedProvider string         `json:"requested_provider"`
	RequestedModel    string         `json:"requested_model"`
	RequestedFallback []string       `json:"requested_fallback"`
	Workspace         string         `json:"workspace"`
	ExplicitPromptIDs []string       `json:"explicit_prompt_ids,omitempty"`
	Custom            map[string]any `json:"custom,omitempty"`
}

// AppliedPrompt describes one resolved prompt application.
type AppliedPrompt struct {
	BindingID string `json:"binding_id"`
	PromptID  string `json:"prompt_id"`
	PromptKey string `json:"prompt_key"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	Scope     string `json:"scope"`
	Target    string `json:"target"`
	Priority  int    `json:"priority"`
	Content   string `json:"content"`
}

// ResolvedPromptSet describes the final rendered prompt set.
type ResolvedPromptSet struct {
	SystemText string          `json:"system_text"`
	UserText   string          `json:"user_text"`
	Applied    []AppliedPrompt `json:"applied"`
}

// SessionBindingSet is the chat-friendly shape for session bindings.
type SessionBindingSet struct {
	SystemPromptIDs []string `json:"system_prompt_ids"`
	UserPromptIDs   []string `json:"user_prompt_ids"`
	Bindings        []Binding `json:"bindings"`
}
