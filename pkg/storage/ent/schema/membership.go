package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Membership links users to tenants with per-tenant role.
type Membership struct {
	ent.Schema
}

// Fields of the Membership.
func (Membership) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("user_id").NotEmpty(),
		field.String("tenant_id").NotEmpty(),
		field.String("role").Default("member"),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Membership.
func (Membership) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("memberships").
			Field("user_id").
			Required().
			Unique(),
		edge.From("tenant", Tenant.Type).
			Ref("memberships").
			Field("tenant_id").
			Required().
			Unique(),
	}
}

// Indexes of the Membership.
func (Membership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "tenant_id").Unique(),
		index.Fields("tenant_id", "enabled"),
		index.Fields("user_id", "enabled"),
	}
}
