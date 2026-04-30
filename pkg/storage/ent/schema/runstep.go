package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// RunStep stores a durable activity item inside a Run.
type RunStep struct {
	ent.Schema
}

// Fields of the RunStep.
func (RunStep) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("run_id").NotEmpty(),
		field.Uint32("sequence").Default(0),
		field.String("kind").Default("message"),
		field.String("status").Default("ok"),
		field.String("summary").Default(""),
		field.String("detail").Default(""),
		field.String("artifact_ids_json").Default("[]"),
		field.Time("started_at").Default(time.Now),
		field.Time("completed_at").Optional().Nillable(),
		field.String("request_id").Default(""),
	}
}

// Edges of the RunStep.
func (RunStep) Edges() []ent.Edge {
	return nil
}

// Indexes of the RunStep.
func (RunStep) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id"),
		index.Fields("run_id", "sequence"),
		index.Fields("kind"),
		index.Fields("started_at"),
	}
}
