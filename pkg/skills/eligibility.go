package skills

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// EligibilityChecker checks if a skill meets system requirements.
type EligibilityChecker struct{}

// NewEligibilityChecker creates a new eligibility checker.
func NewEligibilityChecker() *EligibilityChecker {
	return &EligibilityChecker{}
}

// Check checks if a skill is eligible to run on the current system.
func (c *EligibilityChecker) Check(skill *Skill) (bool, []string) {
	if skill.Requirements == nil {
		return true, nil
	}

	var reasons []string

	// Check OS compatibility
	if !c.CheckOS(skill.Requirements) {
		reasons = append(reasons, "incompatible operating system")
	}

	// Check required binaries
	missingBinaries := c.CheckBinaries(skill.Requirements.Binaries)
	if len(missingBinaries) > 0 {
		reasons = append(reasons, "missing binaries: "+strings.Join(missingBinaries, ", "))
	}

	// Check any-of binary requirements
	if len(skill.Requirements.AnyBinaries) > 0 && !c.CheckAnyBinaries(skill.Requirements.AnyBinaries) {
		reasons = append(reasons, "missing any-of binaries: "+strings.Join(skill.Requirements.AnyBinaries, ", "))
	}

	// Check required environment variables
	missingEnvVars := c.CheckEnvVars(skill.Requirements.Env)
	if len(missingEnvVars) > 0 {
		reasons = append(reasons, "missing environment variables: "+strings.Join(missingEnvVars, ", "))
	}

	// Check required tools
	missingTools := c.CheckTools(skill.Requirements.Tools)
	if len(missingTools) > 0 {
		reasons = append(reasons, "missing tools: "+strings.Join(missingTools, ", "))
	}

	eligible := len(reasons) == 0
	return eligible, reasons
}

// CheckOS checks if the current OS is compatible with the skill.
func (c *EligibilityChecker) CheckOS(req *SkillRequirements) bool {
	if req.Custom == nil {
		return true // No OS restriction
	}

	// Check for os field in custom requirements
	osReq, ok := req.Custom["os"]
	if !ok {
		return true // No OS restriction
	}

	// Handle string or array of strings
	switch v := osReq.(type) {
	case string:
		return c.matchesOS(v)
	case []interface{}:
		for _, os := range v {
			if osStr, ok := os.(string); ok && c.matchesOS(osStr) {
				return true
			}
		}
		return false
	case []string:
		for _, os := range v {
			if c.matchesOS(os) {
				return true
			}
		}
		return false
	default:
		return true // Unknown format, allow by default
	}
}

// matchesOS checks if an OS string matches the current OS.
func (c *EligibilityChecker) matchesOS(os string) bool {
	os = strings.ToLower(strings.TrimSpace(os))
	currentOS := runtime.GOOS

	switch os {
	case "any", "*", "all":
		return true
	case "unix":
		return currentOS != "windows"
	case "mac", "macos", "osx":
		return currentOS == "darwin"
	case "linux":
		return currentOS == "linux"
	case "windows", "win":
		return currentOS == "windows"
	default:
		return os == currentOS
	}
}

// CheckBinaries checks which required binaries are missing.
func (c *EligibilityChecker) CheckBinaries(binaries []string) []string {
	var missing []string

	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}

	return missing
}

// CheckAnyBinaries reports whether at least one required binary exists.
func (c *EligibilityChecker) CheckAnyBinaries(binaries []string) bool {
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// CheckEnvVars checks which required environment variables are missing.
func (c *EligibilityChecker) CheckEnvVars(envVars []string) []string {
	var missing []string

	for _, envVar := range envVars {
		if os.Getenv(envVar) == "" {
			missing = append(missing, envVar)
		}
	}

	return missing
}

// CheckTools checks which required tools are missing.
// Tools are similar to binaries but may have version requirements.
func (c *EligibilityChecker) CheckTools(tools []string) []string {
	var missing []string

	for _, tool := range tools {
		// Parse tool name and optional version
		// Format: "tool" or "tool@version"
		parts := strings.SplitN(tool, "@", 2)
		toolName := parts[0]

		// Check if tool exists in PATH
		if _, err := exec.LookPath(toolName); err != nil {
			missing = append(missing, tool)
			continue
		}

		// Check version if specified
		if len(parts) > 1 {
			requiredVersion := parts[1]
			installedVersion := c.getToolVersion(toolName)
			if installedVersion == "" || !c.versionSatisfies(installedVersion, requiredVersion) {
				missing = append(missing, tool)
			}
		}
	}

	return missing
}

// CheckLanguages checks which required language versions are missing.
func (c *EligibilityChecker) CheckLanguages(languages map[string]string) map[string]string {
	missing := make(map[string]string)

	for lang, version := range languages {
		installed := c.getLanguageVersion(lang)
		if installed == "" {
			missing[lang] = version
			continue
		}

		// Compare versions
		if !c.versionSatisfies(installed, version) {
			missing[lang] = version
		}
	}

	return missing
}

// getLanguageVersion tries to get the installed version of a language.
func (c *EligibilityChecker) getLanguageVersion(lang string) string {
	var cmd *exec.Cmd

	switch strings.ToLower(lang) {
	case "go", "golang":
		cmd = exec.Command("go", "version")
	case "python", "python3":
		cmd = exec.Command("python3", "--version")
	case "node", "nodejs":
		cmd = exec.Command("node", "--version")
	case "ruby":
		cmd = exec.Command("ruby", "--version")
	case "java":
		cmd = exec.Command("java", "-version")
	default:
		return ""
	}

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// DetectBinary checks if a binary exists and returns its path.
func (c *EligibilityChecker) DetectBinary(bin string) (string, bool) {
	path, err := exec.LookPath(bin)
	return path, err == nil
}

// versionRegex matches version-like strings (e.g. "1.21.0", "v18.0.0", "3.11").
var versionRegex = regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

// getToolVersion returns the version string of an installed tool.
func (c *EligibilityChecker) getToolVersion(tool string) string {
	// Try common version flags
	for _, flag := range []string{"--version", "-version", "version"} {
		cmd := exec.Command(tool, flag)
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	return ""
}

// versionSatisfies checks whether installedRaw contains a version >= required.
func (c *EligibilityChecker) versionSatisfies(installedRaw, required string) bool {
	instMajor, instMinor, instPatch, ok := parseVersion(installedRaw)
	if !ok {
		return false
	}
	reqMajor, reqMinor, reqPatch, ok := parseVersion(required)
	if !ok {
		return false
	}
	if instMajor != reqMajor {
		return instMajor > reqMajor
	}
	if instMinor != reqMinor {
		return instMinor > reqMinor
	}
	return instPatch >= reqPatch
}

// parseVersion extracts major.minor.patch from a version string.
func parseVersion(raw string) (major, minor, patch int, ok bool) {
	matches := versionRegex.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(matches[1])
	if len(matches) > 2 && matches[2] != "" {
		minor, _ = strconv.Atoi(matches[2])
	}
	if len(matches) > 3 && matches[3] != "" {
		patch, _ = strconv.Atoi(matches[3])
	}
	return major, minor, patch, true
}
