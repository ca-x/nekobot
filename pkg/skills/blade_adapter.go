package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bladeskills "github.com/go-kratos/blades/skills"
)

// BladesSkillAdapter wraps a nekobot *Skill to implement blade's skills interfaces.
type BladesSkillAdapter struct {
	skill *Skill
}

// NewBladesSkillAdapter creates a new adapter wrapping a nekobot skill.
func NewBladesSkillAdapter(skill *Skill) *BladesSkillAdapter {
	return &BladesSkillAdapter{skill: skill}
}

// Name returns the skill name suitable for blade (kebab-case).
// Uses the skill ID as it's typically kebab-case compliant.
func (a *BladesSkillAdapter) Name() string {
	if a.skill == nil {
		return ""
	}
	if a.skill.ID != "" {
		return a.skill.ID
	}
	return a.skill.Name
}

// Description returns the skill description.
func (a *BladesSkillAdapter) Description() string {
	if a.skill == nil {
		return ""
	}
	return a.skill.Description
}

// Instruction returns the skill instructions.
func (a *BladesSkillAdapter) Instruction() string {
	if a.skill == nil {
		return ""
	}
	return a.skill.Instructions
}

// Frontmatter returns blade-compatible frontmatter from the nekobot skill.
func (a *BladesSkillAdapter) Frontmatter() bladeskills.Frontmatter {
	if a.skill == nil {
		return bladeskills.Frontmatter{}
	}

	metadata := make(map[string]string)

	// Structured fields.
	if a.skill.Name != "" {
		metadata["display_name"] = a.skill.Name
	}
	if a.skill.Version != "" {
		metadata["version"] = a.skill.Version
	}
	if a.skill.Author != "" {
		metadata["author"] = a.skill.Author
	}
	if len(a.skill.Tags) > 0 {
		metadata["tags"] = strings.Join(a.skill.Tags, ",")
	}
	metadata["enabled"] = fmt.Sprint(a.skill.Enabled)
	metadata["always"] = fmt.Sprint(a.skill.Always)

	// Flatten Metadata map.
	flattenMap("", a.skill.Metadata, metadata)

	return bladeskills.Frontmatter{
		Name:        a.Name(),
		Description: a.Description(),
		Metadata:    metadata,
	}
}

// Resources returns blade-compatible resources from the nekobot skill.
// For filesystem-based skills, loads references/ and scripts/ subdirectories.
// For embedded (builtin://) skills, returns empty resources.
func (a *BladesSkillAdapter) Resources() bladeskills.Resources {
	if a.skill == nil {
		return bladeskills.Resources{}
	}

	resourceDir := a.resourceDir()
	if resourceDir == "" {
		return bladeskills.Resources{}
	}

	resources := bladeskills.Resources{
		References: make(map[string]string),
		Assets:     make(map[string][]byte),
		Scripts:    make(map[string]string),
	}

	loadTextDir(filepath.Join(resourceDir, "references"), resources.References)
	loadTextDir(filepath.Join(resourceDir, "scripts"), resources.Scripts)

	return resources
}

// resourceDir returns the directory containing skill resources.
// Returns empty string for embedded/builtin skills.
func (a *BladesSkillAdapter) resourceDir() string {
	if a.skill == nil || a.skill.FilePath == "" {
		return ""
	}
	if strings.HasPrefix(a.skill.FilePath, "builtin://") {
		return ""
	}
	return filepath.Dir(a.skill.FilePath)
}

// flattenMap recursively flattens a map[string]interface{} into a map[string]string
// using dot-notation for nested keys.
func flattenMap(prefix string, src map[string]interface{}, dst map[string]string) {
	for key, value := range src {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			flattenMap(fullKey, v, dst)
		case map[interface{}]interface{}:
			converted := make(map[string]interface{}, len(v))
			for mk, mv := range v {
				converted[fmt.Sprint(mk)] = mv
			}
			flattenMap(fullKey, converted, dst)
		default:
			dst[fullKey] = fmt.Sprint(value)
		}
	}
}

// loadTextDir reads all files from a directory into a map[string]string.
// Silently skips non-existent directories.
func loadTextDir(dir string, dst map[string]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		dst[entry.Name()] = string(data)
	}
}
