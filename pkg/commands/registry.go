package commands

import (
	"fmt"
	"sort"
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
	return strings.HasPrefix(text, "/")
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
	cmdName := normalizeCommandToken(parts[0])

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	return cmdName, args
}

// MalformedCommandMessage returns the standard user-facing reply for slash text
// that was recognized as a command attempt but has no command token.
func MalformedCommandMessage() string {
	return "❌ 命令格式不完整。请输入 `/help` 查看可用命令，或使用 `/help <command>` 查看具体用法。"
}

// UnknownCommandMessage returns the standard user-facing reply for unsupported
// slash commands. Unknown commands are handled locally so they do not become
// ordinary agent prompts.
func (r *Registry) UnknownCommandMessage(cmdName string) string {
	cmdName = strings.TrimPrefix(normalizeCommandToken(cmdName), "/")
	if cmdName == "" {
		return MalformedCommandMessage()
	}

	hint := r.availableCommandHint()
	if hint == "" {
		return fmt.Sprintf("❌ 未知命令 `/%s`。", cmdName)
	}
	return fmt.Sprintf("❌ 未知命令 `/%s`。\n\n%s", cmdName, hint)
}

func (r *Registry) availableCommandHint() string {
	r.mu.RLock()
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	r.mu.RUnlock()

	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	for i, name := range names {
		names[i] = "/" + name
	}
	return "可用命令：" + strings.Join(names, ", ") + "\n普通文本请不要以 `/` 开头。"
}

func normalizeCommandToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" {
		return ""
	}

	// Telegram/group commands can be in /cmd@bot format.
	if at := strings.Index(token, "@"); at > 0 {
		token = token[:at]
	}
	return strings.TrimSpace(token)
}
