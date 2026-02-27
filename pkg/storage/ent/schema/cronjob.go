package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CronJob stores scheduled jobs for cron/at/every execution.
type CronJob struct {
	ent.Schema
}

// Fields of the CronJob.
func (CronJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("name").NotEmpty(),
		field.String("schedule_kind").Default("cron"),
		field.String("schedule").Default(""),
		field.Time("at_time").Optional().Nillable(),
		field.String("every_duration").Default(""),
		field.String("prompt").NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Bool("delete_after_run").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_run").Optional().Nillable(),
		field.Time("next_run").Optional().Nillable(),
		field.Int("run_count").Default(0),
		field.String("last_error").Default(""),
		field.Bool("last_success").Default(false),
	}
}

// Edges of the CronJob.
func (CronJob) Edges() []ent.Edge {
	return nil
}

// Indexes of the CronJob.
func (CronJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "next_run"),
		index.Fields("schedule_kind"),
		index.Fields("created_at"),
	}
}
