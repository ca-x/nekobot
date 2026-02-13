// Package qmd provides process execution helpers for QMD commands.
package qmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// ProcessExecutor executes QMD commands.
type ProcessExecutor struct {
	log     *logger.Logger
	command string
	timeout time.Duration
}

// NewProcessExecutor creates a new process executor.
func NewProcessExecutor(log *logger.Logger, command string, timeout time.Duration) *ProcessExecutor {
	if command == "" {
		command = "qmd"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &ProcessExecutor{
		log:     log,
		command: command,
		timeout: timeout,
	}
}

// CheckAvailable checks if QMD is installed and accessible.
func (p *ProcessExecutor) CheckAvailable() (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := p.execute(ctx, "--version")
	if err != nil {
		return false, "", err
	}

	version := strings.TrimSpace(string(output))
	return true, version, nil
}

// CreateCollection creates a new QMD collection.
func (p *ProcessExecutor) CreateCollection(ctx context.Context, name, path, pattern string) error {
	_, err := p.execute(ctx, "create", name, path, pattern)
	if err != nil {
		return fmt.Errorf("creating collection %s: %w", name, err)
	}

	p.log.Info("Created QMD collection",
		zap.String("name", name),
		zap.String("path", path),
		zap.String("pattern", pattern))

	return nil
}

// UpdateCollection updates an existing collection.
func (p *ProcessExecutor) UpdateCollection(ctx context.Context, name string) error {
	_, err := p.execute(ctx, "update", name)
	if err != nil {
		return fmt.Errorf("updating collection %s: %w", name, err)
	}

	p.log.Debug("Updated QMD collection", zap.String("name", name))
	return nil
}

// DeleteCollection deletes a collection.
func (p *ProcessExecutor) DeleteCollection(ctx context.Context, name string) error {
	_, err := p.execute(ctx, "delete", name)
	if err != nil {
		return fmt.Errorf("deleting collection %s: %w", name, err)
	}

	p.log.Info("Deleted QMD collection", zap.String("name", name))
	return nil
}

// ListCollections lists all collections.
func (p *ProcessExecutor) ListCollections(ctx context.Context) ([]byte, error) {
	output, err := p.execute(ctx, "list")
	if err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}

	return output, nil
}

// Search performs a semantic search in a collection.
func (p *ProcessExecutor) Search(ctx context.Context, name, query string, limit int) ([]byte, error) {
	args := []string{"search", name, query}
	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	output, err := p.execute(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("searching collection %s: %w", name, err)
	}

	return output, nil
}

// execute runs a QMD command with arguments.
func (p *ProcessExecutor) execute(ctx context.Context, args ...string) ([]byte, error) {
	// Create command with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	p.log.Debug("Executing QMD command",
		zap.String("command", p.command),
		zap.Strings("args", args))

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}

		// Check if QMD is not found
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("qmd command not found (install from: https://github.com/username/qmd)")
		}

		return nil, fmt.Errorf("qmd command failed: %s", errMsg)
	}

	return stdout.Bytes(), nil
}

// IsAvailable is a convenience method to check if QMD is available.
func (p *ProcessExecutor) IsAvailable() bool {
	available, _, _ := p.CheckAvailable()
	return available
}
