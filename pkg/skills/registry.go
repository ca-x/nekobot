package skills

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nekobot/pkg/providers"
)

const defaultRegistrySearchTimeout = 30 * time.Second

// RegistryClient wraps remote skill discovery and installation helpers.
type RegistryClient struct {
	proxyURL string
	client   *http.Client
}

// NewRegistryClient creates a proxy-aware remote skills client.
func NewRegistryClient(proxyURL string) (*RegistryClient, error) {
	httpClient, err := providers.NewHTTPClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, fmt.Errorf("create proxy http client: %w", err)
	}
	return &RegistryClient{
		proxyURL: strings.TrimSpace(proxyURL),
		client:   httpClient,
	}, nil
}

// Search queries the external skills registry through the `npx skills find` CLI.
func (c *RegistryClient) Search(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("search query is required")
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return "", fmt.Errorf("npx not installed: %w", err)
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRegistrySearchTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "npx", "skills", "find", query)
	cmd.Env = skillsProxyEnv(os.Environ(), c.proxyURL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("skills registry search timed out: %w", ctx.Err())
		}
		combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if combined == "" {
			return "", fmt.Errorf("skills registry search failed: %w", err)
		}
		return strings.TrimSpace(combined), fmt.Errorf("skills registry search failed: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	return output, nil
}

// Install clones a remote git repository or copies a local skill path into the target skills directory.
func (c *RegistryClient) Install(ctx context.Context, source, targetDir string) (string, error) {
	source = strings.TrimSpace(source)
	targetDir = strings.TrimSpace(targetDir)
	if source == "" {
		return "", fmt.Errorf("skill source is required")
	}
	if targetDir == "" {
		return "", fmt.Errorf("target directory is required")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create skills directory: %w", err)
	}

	if isRemoteSkillSource(source) {
		return c.installRemote(ctx, source, targetDir)
	}
	return c.installLocal(source, targetDir)
}

func (c *RegistryClient) installRemote(ctx context.Context, source, targetDir string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not installed: %w", err)
	}

	targetPath := filepath.Join(targetDir, repoNameFromSource(source))
	if err := os.RemoveAll(targetPath); err != nil {
		return "", fmt.Errorf("clear install target %s: %w", targetPath, err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", source, targetPath)
	cmd.Env = skillsProxyEnv(os.Environ(), c.proxyURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("clone skill repository: %w", err)
	}
	return targetPath, nil
}

func (c *RegistryClient) installLocal(source, targetDir string) (string, error) {
	sourcePath, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve skill source: %w", err)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat skill source: %w", err)
	}

	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	if err := os.RemoveAll(targetPath); err != nil {
		return "", fmt.Errorf("clear install target %s: %w", targetPath, err)
	}

	if info.IsDir() {
		if err := copyDir(sourcePath, targetPath); err != nil {
			return "", fmt.Errorf("copy skill directory: %w", err)
		}
		return targetPath, nil
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create skills directory: %w", err)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}
	if err := os.WriteFile(targetPath, data, info.Mode()); err != nil {
		return "", fmt.Errorf("write installed skill file: %w", err)
	}
	return targetPath, nil
}

func isRemoteSkillSource(source string) bool {
	lowered := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(lowered, "http://") || strings.HasPrefix(lowered, "https://")
}

func repoNameFromSource(source string) string {
	trimmed := strings.TrimSpace(source)
	parts := strings.Split(trimmed, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	if name == "" {
		return "skill"
	}
	return name
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return os.MkdirAll(dst, 0o755)
		}

		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
