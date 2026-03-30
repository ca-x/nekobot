package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AgentRuntime stores first-class agent runtime definitions.
type AgentRuntime struct {
	ent.Schema
}

// Fields of the AgentRuntime.
func (AgentRuntime) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("name").NotEmpty(),
		field.String("display_name").Default(""),
		field.String("description").Default(""),
		field.Bool("enabled").Default(true),
		field.String("provider").Default(""),
		field.String("model").Default(""),
		field.String("prompt_id").Default(""),
		field.String("skills_json").Default("[]"),
		field.String("tools_json").Default("[]"),
		field.String("policy_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the AgentRuntime.
func (AgentRuntime) Edges() []ent.Edge {
	return nil
}

// Indexes of the AgentRuntime.
func (AgentRuntime) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("enabled"),
		index.Fields("updated_at"),
	}
}
