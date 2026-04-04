package repository

import "tg-downloader/src/features/media/domain/entity"

type IMediaDownloadRepository interface {
	ValidateURL(url string) (bool, entity.PlatformInfo, error)
	DownloadMedia(url string, outputDir string, platform entity.PlatformInfo) (*entity.DownloadResult, error)
}
