package converter

import (
	"tg-downloader/ent"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"
)

type DbGroupToGroupConverter struct{}

func NewDbGroupToGroupConverter() *DbGroupToGroupConverter {
	return &DbGroupToGroupConverter{}
}

func (c *DbGroupToGroupConverter) Convert() core.Codec[ent.DbGroup, entity.Group] {
	return &DbGroupToGroupCodec{}
}

func (c *DbGroupToGroupConverter) Parse() core.Codec[entity.Group, ent.DbGroup] {
	return &GroupToDbGroupCodec{}
}

type DbGroupToGroupCodec struct{}

func (c *DbGroupToGroupCodec) Convert(source ent.DbGroup) entity.Group {
	return entity.Group{
		GroupID:       source.Identificator,
		AdminUserName: source.AdminUserName,
	}
}

type GroupToDbGroupCodec struct{}

func (c *GroupToDbGroupCodec) Convert(source entity.Group) ent.DbGroup {
	return ent.DbGroup{
		ID:            0, // Will be set by database on insert
		Identificator: source.GroupID,
		AdminUserName: source.AdminUserName,
	}
}