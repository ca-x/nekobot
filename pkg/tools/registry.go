// Package tools provides the tool system for agent actions.
package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ExecutionHook is called after each tool execution with timing and result info.
type ExecutionHook func(ctx context.Context, toolName string, args map[string]interface{}, result string, duration time.Duration, err error)
type BeforeExecutionHook func(ctx context.Context, toolName string, args map[string]interface{})

// Tool represents an action that the agent can perform.
type Tool interface {
	// Name returns the unique name of the tool.
	Name() string

	// Description returns a human-readable description for the agent.
	Description() string

	// Parameters returns the JSON schema for tool parameters.
	Parameters() map[string]interface{}

	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// Registry manages available tools for the agent.
type Registry struct {
	mu         sync.RWMutex
	tools      map[string]Tool
	beforeHook BeforeExecutionHook
	hook       ExecutionHook // Optional execution hook for auditing/logging
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// Returns an error if a tool with the same name already exists.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// MustRegister registers a tool and panics on error.
// Use this for built-in tools that should always succeed.
func (r *Registry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

// Replace registers a tool, replacing any existing tool with the same name.
func (r *Registry) Replace(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// sortedNames returns tool names in sorted order for deterministic iteration.
// Deterministic ordering improves LLM provider KV cache hit rates.
func (r *Registry) sortedNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List returns all registered tool names in sorted order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.sortedNames()
}

// GetDescriptions returns formatted descriptions of all tools for the agent.
// Tools are returned in sorted order for deterministic system prompts.
func (r *Registry) GetDescriptions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.sortedNames()
	descriptions := make([]string, 0, len(names))
	for _, name := range names {
		tool := r.tools[name]
		desc := fmt.Sprintf("**%s**: %s", tool.Name(), tool.Description())
		descriptions = append(descriptions, desc)
	}
	return descriptions
}

// GetToolDefinitions returns tool definitions in the format expected by LLM providers.
// Tools are returned in sorted order for cache stability.
func (r *Registry) GetToolDefinitions() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.sortedNames()
	definitions := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		tool := r.tools[name]
		definitions = append(definitions, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Parameters(),
			},
		})
	}
	return definitions
}

// Execute runs a tool by name with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, exists := r.Get(name)
	if !exists {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	start := time.Now()
	if r.beforeHook != nil {
		r.beforeHook(ctx, name, args)
	}
	result, err := tool.Execute(ctx, args)
	duration := time.Since(start)

	// Call hook if registered
	if r.hook != nil {
		r.hook(ctx, name, args, result, duration, err)
	}

	return result, err
}

// SetHook sets the execution hook for all tool executions.
// The hook is called after each tool execution with timing information.
func (r *Registry) SetHook(hook ExecutionHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hook = hook
}

// SetBeforeHook sets a hook that runs immediately before tool execution.
func (r *Registry) SetBeforeHook(hook BeforeExecutionHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.beforeHook = hook
}
