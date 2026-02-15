package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ToolEvent stores timeline events for tool sessions.
type ToolEvent struct {
	ent.Schema
}

// Fields of the ToolEvent.
func (ToolEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("session_id"),
		field.String("event_type"),
		field.String("payload_json").Optional().Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the ToolEvent.
func (ToolEvent) Edges() []ent.Edge {
	return nil
}

// Indexes of the ToolEvent.
func (ToolEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "created_at"),
		index.Fields("event_type"),
	}
}
