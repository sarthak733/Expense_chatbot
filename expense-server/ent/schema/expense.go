package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Expense holds the schema definition for the Expense entity.
type Expense struct {
	ent.Schema
}

// Fields of the Expense.
func (Expense) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("user_id"),
		field.String("title").NotEmpty(),
		field.Float("amount"),
		field.String("category").Optional().Nillable(),
		field.Int("category_id").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the Expense.
func (Expense) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("expenses").
			Unique().
			Field("user_id").
			Required(),
		edge.From("category_ref", Category.Type).
			Ref("expenses").
			Unique().
			Field("category_id"),
	}
}
