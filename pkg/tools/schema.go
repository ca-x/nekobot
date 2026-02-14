package tools

import (
	"encoding/json"
	"fmt"
)

// ObjectSchema describes a JSON-schema object using typed properties.
type ObjectSchema[T any] struct {
	Type       string       `json:"type"`
	Properties map[string]T `json:"properties"`
	Required   []string     `json:"required,omitempty"`
}

// ParamSchema describes a single JSON-schema parameter.
type ParamSchema struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Minimum     *int   `json:"minimum,omitempty"`
	Maximum     *int   `json:"maximum,omitempty"`
}

func intPtr(v int) *int { return &v }

// MustSchemaMap converts typed schema structs to map form expected by providers.
func MustSchemaMap[T any](schema T) map[string]interface{} {
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("marshal schema: %v", err))
	}

	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(fmt.Sprintf("unmarshal schema: %v", err))
	}
	return out
}

// DecodeArgs decodes map-style tool arguments into a typed struct.
func DecodeArgs[T any](args map[string]interface{}) (T, error) {
	var out T
	raw, err := json.Marshal(args)
	if err != nil {
		return out, fmt.Errorf("marshal args: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode args: %w", err)
	}
	return out, nil
}
