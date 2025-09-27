package repository

import "tg-downloader/src/features/bot/domain/entity"

type ITaskRepository interface {
	CreateTask(link string, groupID int64) (*entity.Task, error)
	GetNextTask() (*entity.Task, error)
	MarkTaskInProgress(id int) error
	DeleteTask(id int) error
	FindTaskByLink(link string) (*entity.Task, error)
	AddGroupToTask(taskID int, groupID int64) error
}