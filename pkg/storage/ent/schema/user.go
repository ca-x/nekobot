package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// User stores WebUI users for authentication and tenancy membership.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("username").NotEmpty(),
		field.String("nickname").Default(""),
		field.String("password_hash").Default(""),
		field.String("role").Default("member"),
		field.Bool("enabled").Default(true),
		field.Time("last_login").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("memberships", Membership.Type),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("username").Unique(),
		index.Fields("role", "enabled"),
	}
}
