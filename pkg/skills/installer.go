package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Installer handles dependency installation for skills.
type Installer struct {
	log      *logger.Logger
	proxyURL string
}

// NewInstaller creates a new skill installer.
func NewInstaller(log *logger.Logger) *Installer {
	return NewInstallerWithProxy(log, "")
}

// NewInstallerWithProxy creates a new skill installer with proxy support.
func NewInstallerWithProxy(log *logger.Logger, proxyURL string) *Installer {
	return &Installer{
		log:      log,
		proxyURL: strings.TrimSpace(proxyURL),
	}
}

// Install installs a dependency using the specified method.
func (i *Installer) Install(ctx context.Context, spec InstallSpec) InstallResult {
	startTime := time.Now()
	result := InstallResult{
		Method:  spec.Method,
		Package: spec.Package,
	}

	i.log.Info("Installing dependency",
		zap.String("method", spec.Method),
		zap.String("package", spec.Package))

	var err error
	var output string

	switch spec.Method {
	case "brew":
		output, err = i.installBrew(ctx, spec)
	case "apt":
		output, err = i.installApt(ctx, spec)
	case "go":
		output, err = i.installGo(ctx, spec)
	case "npm":
		output, err = i.installNpm(ctx, spec)
	case "pip", "uv":
		output, err = i.installPython(ctx, spec)
	case "download":
		output, err = i.installDownload(ctx, spec)
	case "command":
		output, err = i.runCommand(ctx, spec.Package)
	default:
		err = fmt.Errorf("unknown install method: %s", spec.Method)
	}

	result.Duration = time.Since(startTime)
	result.Output = output
	result.Error = err
	result.Success = err == nil
	if result.Success {
		result.InstalledAt = time.Now()
	}

	if err != nil {
		i.log.Error("Installation failed",
			zap.String("method", spec.Method),
			zap.String("package", spec.Package),
			zap.Error(err))
	} else {
		i.log.Info("Installation successful",
			zap.String("method", spec.Method),
			zap.String("package", spec.Package),
			zap.Duration("duration", result.Duration))
	}

	// Run post-hook if specified
	if result.Success && spec.PostHook != "" {
		i.log.Info("Running post-installation hook",
			zap.String("command", spec.PostHook))

		hookOutput, hookErr := i.runCommand(ctx, spec.PostHook)
		if hookErr != nil {
			i.log.Warn("Post-installation hook failed",
				zap.String("command", spec.PostHook),
				zap.Error(hookErr))
		} else {
			result.Output += "\n" + hookOutput
		}
	}

	return result
}

// installBrew installs a package using Homebrew.
func (i *Installer) installBrew(ctx context.Context, spec InstallSpec) (string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return "", fmt.Errorf("homebrew is only available on macOS and Linux")
	}

	// Check if brew is installed
	if _, err := exec.LookPath("brew"); err != nil {
		return "", fmt.Errorf("homebrew not installed: %w", err)
	}

	pkg := spec.Package
	if spec.Version != "" {
		pkg = fmt.Sprintf("%s@%s", pkg, spec.Version)
	}

	cmd := exec.CommandContext(ctx, "brew", "install", pkg)
	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// installApt installs a package using apt-get.
func (i *Installer) installApt(ctx context.Context, spec InstallSpec) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("apt is only available on Linux")
	}

	if _, err := exec.LookPath("apt-get"); err != nil {
		return "", fmt.Errorf("apt-get not installed: %w", err)
	}

	pkg := spec.Package
	if spec.Version != "" {
		pkg = fmt.Sprintf("%s=%s", pkg, spec.Version)
	}

	cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", pkg)
	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// installGo installs a Go package using go install.
func (i *Installer) installGo(ctx context.Context, spec InstallSpec) (string, error) {
	// Check if go is installed
	if _, err := exec.LookPath("go"); err != nil {
		return "", fmt.Errorf("go not installed: %w", err)
	}

	pkg := spec.Package
	if spec.Version != "" {
		pkg = fmt.Sprintf("%s@%s", pkg, spec.Version)
	} else {
		pkg = fmt.Sprintf("%s@latest", pkg)
	}

	cmd := exec.CommandContext(ctx, "go", "install", pkg)
	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// installNpm installs a package using npm.
func (i *Installer) installNpm(ctx context.Context, spec InstallSpec) (string, error) {
	// Check if npm is installed
	if _, err := exec.LookPath("npm"); err != nil {
		return "", fmt.Errorf("npm not installed: %w", err)
	}

	args := []string{"install", "-g"}

	pkg := spec.Package
	if spec.Version != "" {
		pkg = fmt.Sprintf("%s@%s", pkg, spec.Version)
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// installPython installs a Python package using pip or uv.
func (i *Installer) installPython(ctx context.Context, spec InstallSpec) (string, error) {
	// Prefer uv if specified and available
	var tool string
	if spec.Method == "uv" {
		if _, err := exec.LookPath("uv"); err == nil {
			tool = "uv"
		} else {
			return "", fmt.Errorf("uv not installed")
		}
	} else {
		// Try pip3 first, fallback to pip
		if _, err := exec.LookPath("pip3"); err == nil {
			tool = "pip3"
		} else if _, err := exec.LookPath("pip"); err == nil {
			tool = "pip"
		} else {
			return "", fmt.Errorf("pip not installed")
		}
	}

	args := []string{"install"}

	pkg := spec.Package
	if spec.Version != "" {
		pkg = fmt.Sprintf("%s==%s", pkg, spec.Version)
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// installDownload downloads a file from a URL.
func (i *Installer) installDownload(ctx context.Context, spec InstallSpec) (string, error) {
	// URL should be in spec.Package
	url := spec.Package
	if url == "" {
		return "", fmt.Errorf("download URL not specified")
	}

	// Destination path from options
	dest, ok := spec.Options["dest"].(string)
	if !ok || dest == "" {
		return "", fmt.Errorf("download destination not specified")
	}

	// Use curl or wget
	var cmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		cmd = exec.CommandContext(ctx, "curl", "-L", "-o", dest, url)
	} else if _, err := exec.LookPath("wget"); err == nil {
		cmd = exec.CommandContext(ctx, "wget", "-O", dest, url)
	} else {
		return "", fmt.Errorf("neither curl nor wget available")
	}

	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runCommand runs an arbitrary shell command.
func (i *Installer) runCommand(ctx context.Context, command string) (string, error) {
	// Run in shell for complex commands
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Env = i.proxyEnv()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (i *Installer) proxyEnv() []string {
	return skillsProxyEnv(os.Environ(), i.proxyURL)
}

// CanInstall checks if an install method is available on the current system.
func (i *Installer) CanInstall(method string) bool {
	switch method {
	case "brew":
		_, err := exec.LookPath("brew")
		return err == nil && (runtime.GOOS == "darwin" || runtime.GOOS == "linux")
	case "apt":
		_, err := exec.LookPath("apt-get")
		return err == nil && runtime.GOOS == "linux"
	case "go":
		_, err := exec.LookPath("go")
		return err == nil
	case "npm":
		_, err := exec.LookPath("npm")
		return err == nil
	case "pip":
		_, err1 := exec.LookPath("pip")
		_, err2 := exec.LookPath("pip3")
		return err1 == nil || err2 == nil
	case "uv":
		_, err := exec.LookPath("uv")
		return err == nil
	case "download":
		_, err1 := exec.LookPath("curl")
		_, err2 := exec.LookPath("wget")
		return err1 == nil || err2 == nil
	case "command":
		if runtime.GOOS == "windows" {
			_, err := exec.LookPath("cmd")
			return err == nil
		}
		_, err := exec.LookPath("sh")
		return err == nil
	default:
		return false
	}
}

// ParseRequirementsToSpecs converts skill requirements to install specs.
func ParseRequirementsToSpecs(req *SkillRequirements) []InstallSpec {
	var specs []InstallSpec

	if req == nil || req.Custom == nil {
		return specs
	}

	// Look for install specifications in custom requirements
	if installData, ok := req.Custom["install"]; ok {
		switch v := installData.(type) {
		case map[string]interface{}:
			// Single install spec
			specs = append(specs, parseInstallMap(v))
		case []interface{}:
			// Multiple install specs
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					specs = append(specs, parseInstallMap(m))
				}
			}
		}
	}

	return specs
}

// parseInstallMap parses an install spec from a map.
func parseInstallMap(m map[string]interface{}) InstallSpec {
	spec := InstallSpec{
		Options: make(map[string]interface{}),
	}

	if method, ok := m["method"].(string); ok {
		spec.Method = method
	} else if kind, ok := m["kind"].(string); ok {
		spec.Method = normalizeInstallMethod(kind)
	}
	if pkg, ok := m["package"].(string); ok {
		spec.Package = pkg
	} else if formula, ok := m["formula"].(string); ok {
		spec.Package = formula
	} else if command, ok := m["command"].(string); ok {
		spec.Package = command
	}
	if version, ok := m["version"].(string); ok {
		spec.Version = version
	}
	if postHook, ok := m["post_hook"].(string); ok {
		spec.PostHook = postHook
	} else if postHook, ok := m["postHook"].(string); ok {
		spec.PostHook = postHook
	}

	// Copy all other fields to options
	for k, v := range m {
		if k != "method" &&
			k != "kind" &&
			k != "package" &&
			k != "formula" &&
			k != "command" &&
			k != "version" &&
			k != "post_hook" &&
			k != "postHook" {
			spec.Options[k] = v
		}
	}

	return spec
}

func normalizeInstallMethod(method string) string {
	switch strings.TrimSpace(strings.ToLower(method)) {
	case "node":
		return "npm"
	case "python":
		return "pip"
	case "custom":
		return "command"
	default:
		return strings.TrimSpace(strings.ToLower(method))
	}
}

// GetInstallSummary returns a human-readable summary of install results.
func GetInstallSummary(results []InstallResult) string {
	if len(results) == 0 {
		return "No dependencies installed"
	}

	var parts []string
	successful := 0
	failed := 0

	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
			parts = append(parts, fmt.Sprintf("Failed: %s (%s)", result.Package, result.Method))
		}
	}

	summary := fmt.Sprintf("Installed %d/%d dependencies", successful, len(results))
	if failed > 0 {
		summary += "\n" + strings.Join(parts, "\n")
	}

	return summary
}
