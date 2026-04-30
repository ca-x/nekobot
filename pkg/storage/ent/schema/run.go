package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Run stores a durable execution attempt for a task or direct agent action.
type Run struct {
	ent.Schema
}

// Fields of the Run.
func (Run) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("task_id").Default(""),
		field.String("target").Default(""),
		field.String("agent_id").Default(""),
		field.String("computer_id").Default(""),
		field.String("runtime_profile_id").Default(""),
		field.String("status").Default("queued"),
		field.String("lease_id").Default(""),
		field.String("request_id").Default(""),
		field.String("input_message_id").Default(""),
		field.String("last_seen_event_id").Default(""),
		field.Time("started_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("completed_at").Optional().Nillable(),
		field.String("error").Default(""),
		field.String("summary").Default(""),
		field.String("state").Default(""),
		field.String("tenant_id").Default(""),
		field.String("owner_user_id").Default(""),
		field.Enum("visibility").Values("private", "shared", "system").Default("shared"),
	}
}

// Edges of the Run.
func (Run) Edges() []ent.Edge {
	return nil
}

// Indexes of the Run.
func (Run) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id"),
		index.Fields("target"),
		index.Fields("agent_id"),
		index.Fields("computer_id"),
		index.Fields("status"),
		index.Fields("tenant_id", "visibility"),
		index.Fields("owner_user_id", "visibility"),
		index.Fields("started_at"),
		index.Fields("updated_at"),
	}
}
