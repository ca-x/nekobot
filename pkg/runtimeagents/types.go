package runtimeagents

import "time"

// AgentRuntime defines one independently configurable runtime object.
type AgentRuntime struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	PromptID    string                 `json:"prompt_id"`
	Skills      []string               `json:"skills"`
	Tools       []string               `json:"tools"`
	Policy      map[string]interface{} `json:"policy"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
