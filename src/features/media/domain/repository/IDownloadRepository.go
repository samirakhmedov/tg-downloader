package repository

import "tg-downloader/src/features/media/domain/entity"

type IDownloadRepository interface {
	ValidateURL(url string) (bool, string, error)
	Download(url string, outputDir string) (*entity.MediaProcessResult, error)
}
