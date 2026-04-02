package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ModelCatalog stores globally managed model definitions.
type ModelCatalog struct {
	ent.Schema
}

// Fields of the ModelCatalog.
func (ModelCatalog) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("model_id").NotEmpty(),
		field.String("display_name").Default(""),
		field.String("developer").Default(""),
		field.String("family").Default(""),
		field.String("type").Default(""),
		field.String("capabilities_json").Default("[]"),
		field.String("catalog_source").Default("builtin"),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the ModelCatalog.
func (ModelCatalog) Edges() []ent.Edge {
	return nil
}

// Indexes of the ModelCatalog.
func (ModelCatalog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_id").Unique(),
		index.Fields("enabled"),
		index.Fields("catalog_source"),
	}
}
