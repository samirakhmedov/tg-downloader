package repository

import (
	"context"
	"net/http"
	"time"

	iRepo "tg-downloader/src/features/media/domain/repository"
	mediaEntity "tg-downloader/src/features/media/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const telegramAlbumLimit = 10

// BotAPISender abstracts the Telegram API send methods for testability.
type BotAPISender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	SendMediaGroup(config tgbotapi.MediaGroupConfig) ([]tgbotapi.Message, error)
}

// Compile-time interface check.
var _ iRepo.IMediaUploadRepository = (*MediaUploadRepository)(nil)

type MediaUploadRepository struct {
	sender BotAPISender
}

func NewMediaUploadRepository(botAPI *tgbotapi.BotAPI, uploadTimeoutSeconds int) iRepo.IMediaUploadRepository {
	uploadClient := *botAPI
	uploadClient.Client = &http.Client{
		Timeout: time.Duration(uploadTimeoutSeconds) * time.Second,
	}
	return &MediaUploadRepository{sender: &uploadClient}
}

// NewMediaUploadRepositoryWithSender creates a repository with an injected sender (for testing).
func NewMediaUploadRepositoryWithSender(sender BotAPISender) *MediaUploadRepository {
	return &MediaUploadRepository{sender: sender}
}

func (r *MediaUploadRepository) UploadVideo(ctx context.Context, filePath string, groupID int64) error {
	video := tgbotapi.NewVideo(groupID, tgbotapi.FilePath(filePath))
	video.SupportsStreaming = true
	_, err := r.sender.Send(video)
	return err
}

func (r *MediaUploadRepository) UploadPhoto(ctx context.Context, filePath string, groupID int64) error {
	photo := tgbotapi.NewPhoto(groupID, tgbotapi.FilePath(filePath))
	_, err := r.sender.Send(photo)
	return err
}

func (r *MediaUploadRepository) UploadAlbum(ctx context.Context, items []mediaEntity.MediaItem, groupID int64) error {
	if len(items) == 0 {
		return nil
	}

	if len(items) == 1 {
		item := items[0]
		if item.Type == mediaEntity.MediaTypeVideo {
			video := tgbotapi.NewVideo(groupID, tgbotapi.FilePath(item.FilePath))
			video.SupportsStreaming = true
			_, err := r.sender.Send(video)
			return err
		}
		photo := tgbotapi.NewPhoto(groupID, tgbotapi.FilePath(item.FilePath))
		_, err := r.sender.Send(photo)
		return err
	}

	// Chunk into batches of telegramAlbumLimit.
	for start := 0; start < len(items); start += telegramAlbumLimit {
		end := start + telegramAlbumLimit
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		media := make([]interface{}, 0, len(batch))
		for _, item := range batch {
			if item.Type == mediaEntity.MediaTypeVideo {
				v := tgbotapi.NewInputMediaVideo(tgbotapi.FilePath(item.FilePath))
				v.SupportsStreaming = true
				media = append(media, v)
			} else {
				media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(item.FilePath)))
			}
		}

		_, err := r.sender.SendMediaGroup(tgbotapi.NewMediaGroup(groupID, media))
		if err != nil {
			return err
		}
	}

	return nil
}
