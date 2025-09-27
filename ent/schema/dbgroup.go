package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// DbGroup holds the schema definition for the DbGroup entity.
type DbGroup struct {
	ent.Schema
}

// Fields of the DbGroup.
func (DbGroup) Fields() []ent.Field {
	return []ent.Field{
		field.String("identificator").Unique(),
		field.String("adminUserName").NotEmpty(),
	}
}

// Edges of the DbGroup.
func (DbGroup) Edges() []ent.Edge {
	return nil
}
