package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AttachToken stores one-time attach tokens for browser redirect/login flows.
type AttachToken struct {
	ent.Schema
}

// Fields of the AttachToken.
func (AttachToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("token").Unique(),
		field.String("session_id"),
		field.String("owner").Default(""),
		field.Time("expires_at"),
		field.Time("used_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the AttachToken.
func (AttachToken) Edges() []ent.Edge {
	return nil
}

// Indexes of the AttachToken.
func (AttachToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token").Unique(),
		index.Fields("session_id"),
		index.Fields("owner"),
		index.Fields("expires_at"),
	}
}
