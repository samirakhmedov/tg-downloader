package service

import "tg-downloader/src/features/media/domain/entity"

type MediaProcessCallback func(groupID int64, success bool, result string)

type IMediaService interface {
	StartWorkers()
	StopWorkers()
	ProcessMedia(link string, groupID int64, messageID int) error
	IsTaskExistsForGroup(link string, groupID int64) bool
	GetMediaEvents() entity.MediaEvents
}
