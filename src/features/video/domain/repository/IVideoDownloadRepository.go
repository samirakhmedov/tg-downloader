package repository

import "tg-downloader/src/features/video/domain/entity"

type IVideoDownloadRepository interface {
	ValidateURL(url string) (bool, string, error)
	DownloadVideo(url string, outputDir string) (*entity.VideoProcessResult, error)
}