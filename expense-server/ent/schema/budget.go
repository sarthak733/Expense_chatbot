package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Budget holds the schema definition for the Budget entity.
type Budget struct {
	ent.Schema
}

// Fields of the Budget.
func (Budget) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.Int("user_id"),
		field.Int("category_id").Optional().Nillable(),
		field.Float("amount"),
		field.String("period").Default("monthly"),
		field.Time("start_date"),
		field.Time("end_date"),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the Budget.
func (Budget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("budgets").
			Unique().
			Field("user_id").
			Required(),
		edge.From("category", Category.Type).
			Ref("budgets").
			Unique().
			Field("category_id"),
	}
}
