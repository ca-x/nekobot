package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChannelAccount stores channel-agnostic transport endpoints.
type ChannelAccount struct {
	ent.Schema
}

// Fields of the ChannelAccount.
func (ChannelAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("channel_type").NotEmpty(),
		field.String("account_key").NotEmpty(),
		field.String("display_name").Default(""),
		field.String("description").Default(""),
		field.Bool("enabled").Default(true),
		field.String("config_json").Default("{}"),
		field.String("metadata_json").Default("{}"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the ChannelAccount.
func (ChannelAccount) Edges() []ent.Edge {
	return nil
}

// Indexes of the ChannelAccount.
func (ChannelAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_type", "account_key").Unique(),
		index.Fields("channel_type"),
		index.Fields("enabled"),
		index.Fields("updated_at"),
	}
}
