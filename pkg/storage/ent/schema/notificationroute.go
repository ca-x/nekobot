package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// NotificationRoute stores a named notification routing target (e.g. a channel account + config).
type NotificationRoute struct {
	ent.Schema
}

// Fields of the NotificationRoute.
func (NotificationRoute) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("name").NotEmpty(),
		field.String("description").Default(""),
		field.Bool("enabled").Default(true),
		field.String("channel_account_id").Default(""),
		field.String("target_config_json").Default("{}"),
		field.String("tenant_id").Default(""),
		field.String("owner_user_id").Default(""),
		field.Enum("visibility").Values("private", "shared", "system").Default("shared"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the NotificationRoute.
func (NotificationRoute) Edges() []ent.Edge {
	return nil
}

// Indexes of the NotificationRoute.
func (NotificationRoute) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("channel_account_id"),
		index.Fields("enabled"),
		index.Fields("tenant_id", "visibility"),
		index.Fields("owner_user_id", "visibility"),
		index.Fields("updated_at"),
	}
}
