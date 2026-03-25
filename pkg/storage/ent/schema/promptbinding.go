package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// PromptBinding stores prompt bindings for global, channel, and session scopes.
type PromptBinding struct {
	ent.Schema
}

// Fields of the PromptBinding.
func (PromptBinding) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.Enum("scope").Values("global", "channel", "session").Default("global"),
		field.String("target").Default(""),
		field.String("prompt_id").NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(100),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the PromptBinding.
func (PromptBinding) Edges() []ent.Edge {
	return nil
}

// Indexes of the PromptBinding.
func (PromptBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope", "target", "priority"),
		index.Fields("prompt_id"),
		index.Fields("scope", "target", "prompt_id").Unique(),
	}
}
