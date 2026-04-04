package repository

import (
	"context"

	mediaEntity "tg-downloader/src/features/media/domain/entity"
)

// IMediaUploadRepository defines Telegram media upload operations (video, photo, album).
type IMediaUploadRepository interface {
	UploadVideo(ctx context.Context, filePath string, groupID int64) error
	UploadPhoto(ctx context.Context, filePath string, groupID int64) error
	UploadAlbum(ctx context.Context, items []mediaEntity.MediaItem, groupID int64) error
}
