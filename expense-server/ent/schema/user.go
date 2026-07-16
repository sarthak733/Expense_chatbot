package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id"),
		field.String("username").NotEmpty().Unique(),
		field.String("password_hash").NotEmpty(),
		field.String("email").Optional().Nillable(),
		field.String("first_name").Optional().Nillable(),
		field.String("last_name").Optional().Nillable(),
		field.String("currency").Default("USD"),
		field.String("theme").Default("system"),
		field.String("weekly_start").Default("monday"),
		field.Int("monthly_start_day").Default(1),
		field.Float("budget_alert_threshold").Default(80.0),
		field.Time("created_at").Default(time.Now).
			Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type),
		edge.To("categories", Category.Type),
		edge.To("expenses", Expense.Type),
		edge.To("budgets", Budget.Type),
		edge.To("recurring_expenses", RecurringExpense.Type),
		edge.To("chat_messages", ChatMessage.Type),
	}
}
