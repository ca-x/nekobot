package skills

import "time"

// SkillEntry represents a skill loaded from a source with full metadata.
type SkillEntry struct {
	*Skill

	// Discovery metadata
	Source   string    // Source name (e.g., "builtin", "user", "github")
	Priority int       // Source priority (higher = more important)
	LoadedAt time.Time // When the skill was loaded

	// Installation metadata
	Installed      bool      // Whether dependencies are installed
	InstallStatus  string    // "pending", "installing", "installed", "failed"
	InstallError   error     // Last installation error
	InstalledAt    time.Time // When dependencies were installed
	LastCheckedAt  time.Time // Last eligibility check

	// Eligibility status
	Eligible         bool     // Whether skill meets all requirements
	MissingBinaries  []string // Missing binary dependencies
	MissingEnvVars   []string // Missing environment variables
	MissingPaths     []string // Missing config paths
	IneligibleReason string   // Why the skill is not eligible
}

// SkillChangeEvent represents a change to a skill file.
type SkillChangeEvent struct {
	Type      ChangeType // created, modified, deleted
	SkillName string     // Name of affected skill
	SkillID   string     // ID of affected skill
	Path      string     // File path that changed
	Timestamp time.Time  // When the change occurred
}

// ChangeType enumerates skill change types.
type ChangeType string

const (
	ChangeTypeCreated  ChangeType = "created"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeDeleted  ChangeType = "deleted"
)

// InstallSpec defines how to install a skill dependency.
type InstallSpec struct {
	Method   string                 // "brew", "go", "uv", "npm", "download"
	Package  string                 // Package name or URL
	Version  string                 // Version (optional)
	Options  map[string]interface{} // Method-specific options
	PostHook string                 // Command to run after installation (optional)
}

// ParseInstallSpec parses install specifications from requirements.
func ParseInstallSpec(method, pkg string, opts map[string]interface{}) InstallSpec {
	spec := InstallSpec{
		Method:  method,
		Package: pkg,
		Options: make(map[string]interface{}),
	}

	if opts != nil {
		spec.Options = opts
		if version, ok := opts["version"].(string); ok {
			spec.Version = version
		}
		if postHook, ok := opts["post_hook"].(string); ok {
			spec.PostHook = postHook
		}
	}

	return spec
}

// InstallResult represents the result of a dependency installation.
type InstallResult struct {
	Success    bool      // Whether installation succeeded
	Method     string    // Installation method used
	Package    string    // Package that was installed
	Output     string    // Installation output
	Error      error     // Error if failed
	Duration   time.Duration // How long it took
	InstalledAt time.Time // When it was installed
}

// Diagnostic represents a validation issue.
type Diagnostic struct {
	Severity DiagnosticSeverity // error, warning, info
	Message  string             // Human-readable message
	Field    string             // Field that has the issue
	Line     int                // Line number in file (if applicable)
	Fixable  bool               // Whether this can be auto-fixed
}

// DiagnosticSeverity indicates how serious a diagnostic is.
type DiagnosticSeverity string

const (
	DiagnosticError   DiagnosticSeverity = "error"
	DiagnosticWarning DiagnosticSeverity = "warning"
	DiagnosticInfo    DiagnosticSeverity = "info"
)

// SkillSnapshot represents a cached view of all loaded skills.
type SkillSnapshot struct {
	Skills      []*SkillEntry `json:"skills"`
	LoadedAt    time.Time     `json:"loaded_at"`
	TotalSkills int           `json:"total_skills"`
	Eligible    int           `json:"eligible"`
	Enabled     int           `json:"enabled"`
	Sources     []string      `json:"sources"`
}

// WatcherStatus represents the status of the file watcher.
type WatcherStatus struct {
	Active      bool      // Whether watcher is active
	WatchPaths  []string  // Paths being watched
	LastEvent   time.Time // Last event received
	EventCount  int       // Total events processed
	ErrorCount  int       // Total errors encountered
	LastError   error     // Last error (if any)
}
