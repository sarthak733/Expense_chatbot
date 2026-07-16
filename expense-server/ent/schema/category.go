package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Category holds the schema definition for the Category entity.
type Category struct {
	ent.Schema
}

// Fields of the Category.
func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("user_id").Optional().Nillable(),
		field.String("name").NotEmpty(),
		field.String("color").Default("#808080"),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the Category.
func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("categories").
			Unique().
			Field("user_id"),
		edge.To("expenses", Expense.Type),
		edge.To("budgets", Budget.Type),
		edge.To("recurring_expenses", RecurringExpense.Type),
	}
}

// Indexes of the Category.
func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "name").Unique(),
	}
}
