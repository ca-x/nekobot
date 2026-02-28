package skills

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Validator validates skill definitions and metadata.
type Validator struct {
	// NamePattern defines valid skill name pattern
	NamePattern *regexp.Regexp
}

// NewValidator creates a new skill validator.
func NewValidator() *Validator {
	return &Validator{
		// Skill names must be lowercase alphanumeric with hyphens
		NamePattern: regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`),
	}
}

// Validate validates a skill and returns any diagnostics.
func (v *Validator) Validate(skill *Skill) []Diagnostic {
	var diagnostics []Diagnostic

	// Validate ID
	diagnostics = append(diagnostics, v.ValidateID(skill.ID)...)

	// Validate name
	diagnostics = append(diagnostics, v.ValidateName(skill.Name)...)

	// Validate description
	diagnostics = append(diagnostics, v.ValidateDescription(skill.Description)...)

	// Validate instructions
	diagnostics = append(diagnostics, v.ValidateInstructions(skill.Instructions)...)

	// Validate requirements
	if skill.Requirements != nil {
		diagnostics = append(diagnostics, v.ValidateRequirements(skill.Requirements)...)
	}

	// Validate always flag with enabled setting.
	diagnostics = append(diagnostics, v.ValidateAlways(skill)...)

	return diagnostics
}

// ValidateID validates the skill ID.
func (v *Validator) ValidateID(id string) []Diagnostic {
	var diagnostics []Diagnostic

	if id == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticError,
			Message:  "Skill ID is required",
			Field:    "id",
			Fixable:  false,
		})
		return diagnostics
	}

	if !v.NamePattern.MatchString(id) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticError,
			Message:  "Skill ID must be lowercase alphanumeric with hyphens (e.g., 'my-skill')",
			Field:    "id",
			Fixable:  true,
		})
	}

	if len(id) > 64 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill ID should be 64 characters or less",
			Field:    "id",
			Fixable:  false,
		})
	}

	return diagnostics
}

// ValidateName validates the skill name.
func (v *Validator) ValidateName(name string) []Diagnostic {
	var diagnostics []Diagnostic

	if name == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill name is recommended (defaults to ID)",
			Field:    "name",
			Fixable:  true,
		})
		return diagnostics
	}

	if len(name) > 100 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill name should be 100 characters or less",
			Field:    "name",
			Fixable:  false,
		})
	}

	return diagnostics
}

// ValidateDescription validates the skill description.
func (v *Validator) ValidateDescription(description string) []Diagnostic {
	var diagnostics []Diagnostic

	if description == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill description is recommended for documentation",
			Field:    "description",
			Fixable:  false,
		})
		return diagnostics
	}

	if len(description) > 500 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill description should be 500 characters or less",
			Field:    "description",
			Fixable:  false,
		})
	}

	return diagnostics
}

// ValidateInstructions validates the skill instructions.
func (v *Validator) ValidateInstructions(instructions string) []Diagnostic {
	var diagnostics []Diagnostic

	if strings.TrimSpace(instructions) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticError,
			Message:  "Skill instructions cannot be empty",
			Field:    "instructions",
			Fixable:  false,
		})
		return diagnostics
	}

	if len(instructions) < 50 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Skill instructions seem very short (< 50 chars)",
			Field:    "instructions",
			Fixable:  false,
		})
	}

	return diagnostics
}

// ValidateRequirements validates skill requirements.
func (v *Validator) ValidateRequirements(req *SkillRequirements) []Diagnostic {
	var diagnostics []Diagnostic

	// Validate binaries
	for _, bin := range req.Binaries {
		if strings.TrimSpace(bin) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticError,
				Message:  "Binary requirement cannot be empty",
				Field:    "requirements.binaries",
				Fixable:  false,
			})
		}
	}

	// Validate env vars
	for _, envVar := range req.Env {
		if strings.TrimSpace(envVar) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticError,
				Message:  "Environment variable requirement cannot be empty",
				Field:    "requirements.env",
				Fixable:  false,
			})
		}
		if !strings.Contains(envVar, "_") && strings.ToUpper(envVar) != envVar {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticWarning,
				Message:  fmt.Sprintf("Environment variable '%s' should typically be UPPERCASE", envVar),
				Field:    "requirements.env",
				Fixable:  true,
			})
		}
	}

	return diagnostics
}

// ValidateAlways validates always-on skill constraints.
func (v *Validator) ValidateAlways(skill *Skill) []Diagnostic {
	var diagnostics []Diagnostic

	if !skill.Always {
		return diagnostics
	}

	if !skill.Enabled {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Always-on skill should also be enabled",
			Field:    "always",
			Fixable:  true,
		})
	}

	return diagnostics
}

// ValidateFile validates a skill file before loading.
func (v *Validator) ValidateFile(path string) []Diagnostic {
	var diagnostics []Diagnostic

	// Check file extension
	if filepath.Ext(path) != ".md" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticError,
			Message:  "Skill files must have .md extension",
			Field:    "file",
			Fixable:  false,
		})
	}

	// Check filename
	filename := filepath.Base(path)
	if filename[0] == '.' {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Message:  "Hidden skill files (starting with .) are discouraged",
			Field:    "file",
			Fixable:  false,
		})
	}

	return diagnostics
}

// HasErrors checks if diagnostics contain any errors.
func HasErrors(diagnostics []Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == DiagnosticError {
			return true
		}
	}
	return false
}

// FilterBySeverity filters diagnostics by severity level.
func FilterBySeverity(diagnostics []Diagnostic, severity DiagnosticSeverity) []Diagnostic {
	var filtered []Diagnostic
	for _, d := range diagnostics {
		if d.Severity == severity {
			filtered = append(filtered, d)
		}
	}
	return filtered
}
