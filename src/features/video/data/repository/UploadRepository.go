package repository

import (
	"tg-downloader/src/features/video/domain/repository"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UploadRepository struct {
	botAPI *tgbotapi.BotAPI
}

func NewUploadRepository(botAPI *tgbotapi.BotAPI) repository.IUploadRepository {
	return &UploadRepository{
		botAPI: botAPI,
	}
}

func (r *UploadRepository) UploadVideo(filePath string, groupID int64) error {
	video := tgbotapi.NewVideo(groupID, tgbotapi.FilePath(filePath))
	video.SupportsStreaming = true

	_, err := r.botAPI.Send(video)
	return err
}
