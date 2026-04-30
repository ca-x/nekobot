package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// NotificationBinding connects event types in a given scope to a notification route.
type NotificationBinding struct {
	ent.Schema
}

// Fields of the NotificationBinding.
func (NotificationBinding) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("scope").NotEmpty(),
		field.String("target").Default(""),
		field.String("route_id").NotEmpty(),
		field.String("event_types_json").Default("[]"),
		field.Bool("enabled").Default(true),
		field.String("tenant_id").Default(""),
		field.String("owner_user_id").Default(""),
		field.Enum("visibility").Values("private", "shared", "system").Default("shared"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the NotificationBinding.
func (NotificationBinding) Edges() []ent.Edge {
	return nil
}

// Indexes of the NotificationBinding.
func (NotificationBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("route_id"),
		index.Fields("scope", "target"),
		index.Fields("enabled"),
		index.Fields("tenant_id", "visibility"),
		index.Fields("owner_user_id", "visibility"),
		index.Fields("updated_at"),
	}
}
