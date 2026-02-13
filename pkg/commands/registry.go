package commands

import (
	"fmt"
	"strings"
	"sync"
)

// Registry manages command registration and lookup.
type Registry struct {
	commands map[string]*Command
	mu       sync.RWMutex
}

// NewRegistry creates a new command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
	}
}

// Register registers a new command.
func (r *Registry) Register(cmd *Command) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if cmd.Name == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	// Normalize command name (lowercase, no /)
	cmd.Name = strings.ToLower(strings.TrimPrefix(cmd.Name, "/"))

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[cmd.Name]; exists {
		return fmt.Errorf("command %s already registered", cmd.Name)
	}

	r.commands[cmd.Name] = cmd
	return nil
}

// Get retrieves a command by name.
func (r *Registry) Get(name string) (*Command, bool) {
	// Normalize name
	name = strings.ToLower(strings.TrimPrefix(name, "/"))

	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd, exists := r.commands[name]
	return cmd, exists
}

// List returns all registered commands.
func (r *Registry) List() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}

	return cmds
}

// IsCommand checks if a text starts with a command.
func (r *Registry) IsCommand(text string) bool {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return false
	}

	// Extract command name (first word)
	parts := strings.SplitN(text, " ", 2)
	cmdName := strings.TrimPrefix(parts[0], "/")

	_, exists := r.Get(cmdName)
	return exists
}

// Parse parses a command from text.
// Returns command name and arguments.
func (r *Registry) Parse(text string) (string, string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}

	// Remove leading /
	text = strings.TrimPrefix(text, "/")

	// Split command and args
	parts := strings.SplitN(text, " ", 2)
	cmdName := strings.ToLower(parts[0])

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	return cmdName, args
}
