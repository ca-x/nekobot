package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ConfigSection stores runtime config sections.
type ConfigSection struct {
	ent.Schema
}

// Fields of the ConfigSection.
func (ConfigSection) Fields() []ent.Field {
	return []ent.Field{
		field.String("section").NotEmpty(),
		field.String("payload_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the ConfigSection.
func (ConfigSection) Edges() []ent.Edge {
	return nil
}

// Indexes of the ConfigSection.
func (ConfigSection) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("section").Unique(),
	}
}
