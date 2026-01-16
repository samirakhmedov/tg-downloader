package service

import "tg-downloader/src/features/video/domain/entity"

type VideoProcessCallback func(groupID int64, success bool, result string)

type IVideoService interface {
	StartWorkers()
	StopWorkers()
	ProcessVideo(link string, groupID int64, messageID int) error
	GetVideoEvents() entity.VideoEvents
}
