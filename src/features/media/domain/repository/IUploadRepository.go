package repository

import "tg-downloader/src/features/media/domain/entity"

type IUploadRepository interface {
	UploadMedia(files []entity.MediaFile, groupID int64) error
}
