package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Task holds the schema definition for the Task entity.
type Task struct {
	ent.Schema
}

// Fields of the Task.
func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.String("link").NotEmpty().Unique(),
		field.JSON("groupIDs", []int64{}),
		field.String("status").Default("pending"),
	}
}

// Edges of the Task.
func (Task) Edges() []ent.Edge {
	return nil
}