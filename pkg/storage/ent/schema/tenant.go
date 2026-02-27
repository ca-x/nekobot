package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Tenant stores tenant-level isolation boundary for runtime data.
type Tenant struct {
	ent.Schema
}

// Fields of the Tenant.
func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("slug").NotEmpty(),
		field.String("name").NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Tenant.
func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("memberships", Membership.Type),
	}
}

// Indexes of the Tenant.
func (Tenant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").Unique(),
		index.Fields("enabled"),
	}
}
