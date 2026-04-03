package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// PermissionRule stores persisted tool-governance rules.
type PermissionRule struct {
	ent.Schema
}

// Fields of the PermissionRule.
func (PermissionRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(0),
		field.String("tool_name").NotEmpty(),
		field.String("session_id").Default(""),
		field.String("runtime_id").Default(""),
		field.String("action").NotEmpty(),
		field.String("description").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the PermissionRule.
func (PermissionRule) Edges() []ent.Edge {
	return nil
}

// Indexes of the PermissionRule.
func (PermissionRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tool_name"),
		index.Fields("enabled"),
		index.Fields("priority"),
		index.Fields("session_id"),
		index.Fields("runtime_id"),
	}
}
