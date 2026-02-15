package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Provider stores provider profiles configured via WebUI.
type Provider struct {
	ent.Schema
}

// Fields of the Provider.
func (Provider) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("name").NotEmpty(),
		field.String("provider_kind").NotEmpty(),
		field.String("api_key").Default(""),
		field.String("api_base").Default(""),
		field.String("proxy").Default(""),
		field.String("models_json").Default("[]"),
		field.String("default_model").Default(""),
		field.Int("timeout").Default(60),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Provider.
func (Provider) Edges() []ent.Edge {
	return nil
}

// Indexes of the Provider.
func (Provider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("provider_kind"),
	}
}
