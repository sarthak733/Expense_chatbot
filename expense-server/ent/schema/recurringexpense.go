package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// RecurringExpense holds the schema definition for the RecurringExpense entity.
type RecurringExpense struct {
	ent.Schema
}

// Fields of the RecurringExpense.
func (RecurringExpense) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("user_id"),
		field.String("title").NotEmpty(),
		field.Float("amount"),
		field.Int("category_id").Optional().Nillable(),
		field.String("interval"),
		field.Time("next_run_date"),
		field.Time("last_run_date").Optional().Nillable(),
		field.Bool("is_active").Default(true),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the RecurringExpense.
func (RecurringExpense) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("recurring_expenses").
			Unique().
			Field("user_id").
			Required(),
		edge.From("category", Category.Type).
			Ref("recurring_expenses").
			Unique().
			Field("category_id"),
	}
}
