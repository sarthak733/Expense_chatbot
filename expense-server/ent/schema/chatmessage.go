package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ChatMessage holds the schema definition for the ChatMessage entity.
type ChatMessage struct {
	ent.Schema
}

// Fields of the ChatMessage.
func (ChatMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("user_id"),
		field.String("role"),
		field.String("message_text"),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the ChatMessage.
func (ChatMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("chat_messages").
			Unique().
			Field("user_id").
			Required(),
	}
}
