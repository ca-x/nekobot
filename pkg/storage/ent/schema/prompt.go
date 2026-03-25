package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Prompt stores reusable prompt templates.
type Prompt struct {
	ent.Schema
}

// Fields of the Prompt.
func (Prompt) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("prompt_key").NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("description").Default(""),
		field.Enum("mode").Values("system", "user").Default("system"),
		field.String("template").NotEmpty(),
		field.Bool("enabled").Default(true),
		field.String("tags_json").Default("[]"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Prompt.
func (Prompt) Edges() []ent.Edge {
	return nil
}

// Indexes of the Prompt.
func (Prompt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("prompt_key").Unique(),
		index.Fields("mode"),
		index.Fields("enabled"),
		index.Fields("updated_at"),
	}
}
