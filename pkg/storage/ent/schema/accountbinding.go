package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AccountBinding stores account-to-runtime routing contracts.
type AccountBinding struct {
	ent.Schema
}

// Fields of the AccountBinding.
func (AccountBinding) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("channel_account_id").NotEmpty(),
		field.String("agent_runtime_id").NotEmpty(),
		field.Enum("binding_mode").Values("single_agent", "multi_agent").Default("single_agent"),
		field.Bool("enabled").Default(true),
		field.Bool("allow_public_reply").Default(true),
		field.String("reply_label").Default(""),
		field.Int("priority").Default(100),
		field.String("metadata_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the AccountBinding.
func (AccountBinding) Edges() []ent.Edge {
	return nil
}

// Indexes of the AccountBinding.
func (AccountBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_account_id", "agent_runtime_id").Unique(),
		index.Fields("channel_account_id"),
		index.Fields("agent_runtime_id"),
		index.Fields("binding_mode"),
		index.Fields("enabled"),
	}
}
