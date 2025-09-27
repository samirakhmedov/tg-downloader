package repository

import "tg-downloader/src/features/bot/domain/entity"

type IBotCacheRepository interface {
	GetGroup(id string) (*entity.Group, error)
	WriteGroup(group *entity.Group) error
	GetAllGroupsByUserName(username string) ([]*entity.Group, error)
	DeleteGroup(id string) error
}
