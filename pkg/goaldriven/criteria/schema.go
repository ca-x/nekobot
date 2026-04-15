package criteria

import (
	"fmt"
	"strings"
)

// Schema validates criteria sets.
type Schema struct{}

// NewSchema creates a criteria schema validator.
func NewSchema() *Schema { return &Schema{} }

// Validate validates one criteria set.
func (s *Schema) Validate(set Set) error {
	if len(set.Criteria) == 0 {
		return fmt.Errorf("criteria cannot be empty")
	}
	ids := make(map[string]struct{}, len(set.Criteria))
	for _, item := range set.Criteria {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("criterion id is required")
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("criterion id %q must be unique", item.ID)
		}
		ids[item.ID] = struct{}{}
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("criterion %q title is required", item.ID)
		}
		if item.Scope.Kind == "" {
			return fmt.Errorf("criterion %q scope kind is required", item.ID)
		}
		if strings.TrimSpace(item.Scope.Source) == "" {
			return fmt.Errorf("criterion %q scope source is required", item.ID)
		}
		if item.Type == TypeManualConfirmation {
			prompt, _ := item.Definition["prompt"].(string)
			if strings.TrimSpace(prompt) == "" {
				return fmt.Errorf("criterion %q manual_confirmation prompt is required", item.ID)
			}
			continue
		}
		if item.Type == TypeCommand {
			command, _ := item.Definition["command"].(string)
			if strings.TrimSpace(command) == "" {
				return fmt.Errorf("criterion %q command is required", item.ID)
			}
			if _, ok := item.Definition["expect_exit_code"]; !ok {
				return fmt.Errorf("criterion %q expect_exit_code is required", item.ID)
			}
			continue
		}
		if item.Type == TypeFileExists {
			path, _ := item.Definition["path"].(string)
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("criterion %q path is required", item.ID)
			}
			continue
		}
		if item.Type == TypeFileContains {
			path, _ := item.Definition["path"].(string)
			contains, _ := item.Definition["contains"].(string)
			if strings.TrimSpace(path) == "" || strings.TrimSpace(contains) == "" {
				return fmt.Errorf("criterion %q path and contains are required", item.ID)
			}
			continue
		}
		return fmt.Errorf("criterion %q has unsupported type %q", item.ID, item.Type)
	}
	return nil
}
