package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ToolSession stores persisted metadata for interactive tool sessions.
type ToolSession struct {
	ent.Schema
}

// Fields of the ToolSession.
func (ToolSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("owner").Default(""),
		field.String("source").Default("webui"),
		field.String("channel").Optional().Default(""),
		field.String("conversation_key").Optional().Default(""),
		field.String("tool").Default(""),
		field.String("title").Optional().Default(""),
		field.String("command").Optional().Default(""),
		field.String("workdir").Optional().Default(""),
		field.String("state").Default("running"),
		field.String("access_mode").Default("none"),
		field.String("access_secret_hash").Optional().Default(""),
		field.Time("access_once_used_at").Optional().Nillable(),
		field.Bool("pinned").Default(false),
		field.Time("last_active_at").Default(time.Now),
		field.Time("detached_at").Optional().Nillable(),
		field.Time("terminated_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.String("metadata_json").Optional().Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the ToolSession.
func (ToolSession) Edges() []ent.Edge {
	return nil
}

// Indexes of the ToolSession.
func (ToolSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner", "state"),
		index.Fields("owner", "access_mode"),
		index.Fields("source", "conversation_key"),
		index.Fields("last_active_at"),
		index.Fields("created_at"),
		index.Fields("updated_at"),
	}
}

// Annotations of the ToolSession.
func (ToolSession) Annotations() []schema.Annotation {
	return nil
}
