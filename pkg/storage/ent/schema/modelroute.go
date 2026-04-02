package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ModelRoute stores provider assignments and routing metadata for one model.
type ModelRoute struct {
	ent.Schema
}

// Fields of the ModelRoute.
func (ModelRoute) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("model_id").NotEmpty(),
		field.String("provider_name").NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Bool("is_default").Default(false),
		field.Int("weight_override").Default(0),
		field.String("aliases_json").Default("[]"),
		field.String("regex_rules_json").Default("[]"),
		field.String("metadata_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the ModelRoute.
func (ModelRoute) Edges() []ent.Edge {
	return nil
}

// Indexes of the ModelRoute.
func (ModelRoute) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_id", "provider_name").Unique(),
		index.Fields("model_id"),
		index.Fields("provider_name"),
		index.Fields("enabled"),
	}
}
