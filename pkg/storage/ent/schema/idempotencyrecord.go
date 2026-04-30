package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// IdempotencyRecord stores deduplication state for mutating RPCs.
type IdempotencyRecord struct {
	ent.Schema
}

// Fields of the IdempotencyRecord.
func (IdempotencyRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string { return uuid.NewString() }).
			Immutable(),
		field.String("tenant_id").Default(""),
		field.String("caller_kind").Default("user"),
		field.String("caller_id").Default(""),
		field.String("method").Default(""),
		field.String("request_id").Default(""),
		field.String("request_hash").Default(""),
		field.String("status").Default("pending"),
		field.String("response_type").Default(""),
		field.String("response_json").Default(""),
		field.String("error_code").Default(""),
		field.String("error_message").Default(""),
		field.String("resource_kind").Default(""),
		field.String("resource_id").Default(""),
		field.String("event_id").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("expires_at").Default(defaultExpiry),
	}
}

// Edges of the IdempotencyRecord.
func (IdempotencyRecord) Edges() []ent.Edge {
	return nil
}

// Indexes of the IdempotencyRecord.
func (IdempotencyRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "caller_kind", "caller_id", "method", "request_id").
			Unique(),
		index.Fields("expires_at"),
		index.Fields("resource_kind", "resource_id"),
		index.Fields("status"),
	}
}

func defaultExpiry() time.Time {
	return time.Now().Add(30 * 24 * time.Hour)
}
