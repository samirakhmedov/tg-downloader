package converter

import (
	"tg-downloader/ent"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"
)

type TaskToDbTaskConverter struct{}

func NewTaskToDbTaskConverter() *TaskToDbTaskConverter {
	return &TaskToDbTaskConverter{}
}

func (c *TaskToDbTaskConverter) Convert() core.Codec[ent.Task, entity.Task] {
	return &taskToDbTaskCodec{}
}

func (c *TaskToDbTaskConverter) Parse() core.Codec[entity.Task, ent.Task] {
	return &dbTaskToTaskCodec{}
}

type taskToDbTaskCodec struct{}

func (c *taskToDbTaskCodec) Convert(source ent.Task) entity.Task {
	return entity.Task{
		ID:       source.ID,
		Link:     source.Link,
		GroupIDs: source.GroupIDs,
		Status:   entity.TaskStatus(source.Status),
	}
}

type dbTaskToTaskCodec struct{}

func (c *dbTaskToTaskCodec) Convert(source entity.Task) ent.Task {
	return ent.Task{
		ID:       source.ID,
		Link:     source.Link,
		GroupIDs: source.GroupIDs,
		Status:   string(source.Status),
	}
}