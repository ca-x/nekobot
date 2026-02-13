package workspace

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// RenderTemplate renders a template string with variables.
func RenderTemplate(templateStr string, vars map[string]string) (string, error) {
	// Use simple string replacement for now (more efficient than text/template for simple cases)
	// Can be upgraded to full text/template if needed for complex logic

	result := templateStr

	for key, value := range vars {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// RenderTemplateAdvanced renders a template with full text/template support.
// Use this for templates with logic (if/range/etc).
func RenderTemplateAdvanced(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("template").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// TemplateVars holds all available template variables.
type TemplateVars struct {
	// Date and Time
	Date      string // 2006-01-02
	DayOfWeek string // Monday, Tuesday, etc.
	Timezone  string // MST, PST, etc.

	// Agent Info
	AgentID   string // Agent identifier
	AgentName string // Agent display name
	Version   string // Nekobot version

	// Workspace
	Workspace string // Workspace path

	// Configuration
	Model    string // LLM model name
	Provider string // Provider name (anthropic, openai, etc.)
}

// ToMap converts TemplateVars to a map for simple rendering.
func (v *TemplateVars) ToMap() map[string]string {
	return map[string]string{
		"Date":      v.Date,
		"DayOfWeek": v.DayOfWeek,
		"Timezone":  v.Timezone,
		"AgentID":   v.AgentID,
		"AgentName": v.AgentName,
		"Version":   v.Version,
		"Workspace": v.Workspace,
		"Model":     v.Model,
		"Provider":  v.Provider,
	}
}
