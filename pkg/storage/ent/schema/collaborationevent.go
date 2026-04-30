package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CollaborationEvent stores the append-only daemon collaboration event log.
type CollaborationEvent struct {
	ent.Schema
}

// Fields of the CollaborationEvent.
func (CollaborationEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("tenant_id").Default("default"),
		field.String("server_id").Default(""),
		field.String("stream").Default("tenant:default"),
		field.Int64("sequence").Default(0),
		field.String("event_id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("event_type").NotEmpty(),
		field.String("target").Default(""),
		field.String("thread_id").Default(""),
		field.String("actor_kind").Default("system"),
		field.String("actor_id").Default(""),
		field.String("subject_kind").Default(""),
		field.String("subject_id").Default(""),
		field.String("parent_subject_kind").Default(""),
		field.String("parent_subject_id").Default(""),
		field.String("assignee_id").Default(""),
		field.String("mentioned_agent_ids_json").Default("[]"),
		field.String("capability_keys_json").Default("[]"),
		field.Int64("graph_version").Default(0),
		field.String("idempotency_key").Default(""),
		field.String("payload_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the CollaborationEvent.
func (CollaborationEvent) Edges() []ent.Edge {
	return nil
}

// Indexes of the CollaborationEvent.
func (CollaborationEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "stream", "sequence").Unique(),
		index.Fields("tenant_id", "event_id").Unique(),
		index.Fields("tenant_id", "target", "sequence"),
		index.Fields("tenant_id", "assignee_id", "sequence"),
		index.Fields("tenant_id", "actor_id", "sequence"),
		index.Fields("tenant_id", "subject_kind", "subject_id"),
		index.Fields("event_type"),
		index.Fields("created_at"),
	}
}
