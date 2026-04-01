package execenv

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const (
	// EnvExecenv marks which execenv implementation prepared the process.
	EnvExecenv = "NEKOBOT_EXECENV"
	// EnvSessionID propagates the logical session identifier into child processes.
	EnvSessionID = "NEKOBOT_SESSION_ID"
	// EnvRuntimeID propagates the runtime identifier into child processes.
	EnvRuntimeID = "NEKOBOT_RUNTIME_ID"
	// EnvTaskID propagates the task identifier into child processes.
	EnvTaskID = "NEKOBOT_TASK_ID"
)

// StartSpec describes one execution environment preparation request.
type StartSpec struct {
	SessionID string
	Command   string
	Workdir   string
	RuntimeID string
	TaskID    string
	Env       []string
}

// Prepared contains normalized execution inputs and cleanup hooks.
type Prepared struct {
	Workdir string
	Env     []string
	Cleanup func() error
}

// Preparer prepares one execution environment.
type Preparer interface {
	Prepare(ctx context.Context, spec StartSpec) (Prepared, error)
}

// DefaultPreparer performs minimal local environment preparation.
type DefaultPreparer struct{}

// NewDefaultPreparer creates the default local execenv preparer.
func NewDefaultPreparer() *DefaultPreparer {
	return &DefaultPreparer{}
}

// Prepare normalizes workdir, prepares the directory, and injects runtime metadata env vars.
func (p *DefaultPreparer) Prepare(_ context.Context, spec StartSpec) (Prepared, error) {
	workdir, err := normalizeWorkdir(spec.Workdir)
	if err != nil {
		return Prepared{}, fmt.Errorf("normalize workdir: %w", err)
	}
	if workdir != "" {
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			return Prepared{}, fmt.Errorf("prepare workdir: %w", err)
		}
	}

	env := buildEnv(spec)
	return Prepared{
		Workdir: workdir,
		Env:     env,
		Cleanup: func() error { return nil },
	}, nil
}

func buildEnv(spec StartSpec) []string {
	env := append([]string{}, spec.Env...)
	env = setEnvValue(env, "TERM", normalizedTerm(env))
	env = setEnvValue(env, "COLORTERM", normalizedColorTerm(env))
	env = setEnvValue(env, EnvExecenv, "default")
	if trimmed := strings.TrimSpace(spec.SessionID); trimmed != "" {
		env = setEnvValue(env, EnvSessionID, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.RuntimeID); trimmed != "" {
		env = setEnvValue(env, EnvRuntimeID, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.TaskID); trimmed != "" {
		env = setEnvValue(env, EnvTaskID, trimmed)
	}
	return env
}

func normalizedTerm(env []string) string {
	current := getEnvValue(env, "TERM")
	if current == "" || strings.EqualFold(current, "dumb") {
		return "xterm-256color"
	}
	return current
}

func normalizedColorTerm(env []string) string {
	current := getEnvValue(env, "COLORTERM")
	if current == "" {
		return "truecolor"
	}
	return current
}

func getEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(entry, prefix))
		}
	}
	return ""
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func normalizeWorkdir(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", nil
	}

	path = expandTildePath(path)
	path = os.ExpandEnv(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	return filepath.Clean(path), nil
}

func expandTildePath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}

	sepIdx := strings.IndexRune(path, '/')
	prefix := path
	suffix := ""
	if sepIdx >= 0 {
		prefix = path[:sepIdx]
		suffix = path[sepIdx:]
	}

	if prefix == "~" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return path
		}
		return home + suffix
	}

	userName := strings.TrimPrefix(prefix, "~")
	if userName == "" {
		return path
	}
	u, err := user.Lookup(userName)
	if err != nil || strings.TrimSpace(u.HomeDir) == "" {
		return path
	}
	return u.HomeDir + suffix
}
