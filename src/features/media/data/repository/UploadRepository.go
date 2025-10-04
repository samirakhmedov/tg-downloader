package repository

import (
	"tg-downloader/src/features/media/domain/entity"
	"tg-downloader/src/features/media/domain/repository"

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

func (r *UploadRepository) UploadMedia(files []entity.MediaFile, groupID int64) error {
	if len(files) == 0 {
		return nil
	}

	// Separate files by type
	var videos []entity.MediaFile
	var images []entity.MediaFile
	var audios []entity.MediaFile

	for _, file := range files {
		switch file.MediaType {
		case entity.MediaTypeVideo:
			videos = append(videos, file)
		case entity.MediaTypeImage:
			images = append(images, file)
		case entity.MediaTypeAudio:
			audios = append(audios, file)
		}
	}

	// Upload videos individually
	for _, video := range videos {
		videoMsg := tgbotapi.NewVideo(groupID, tgbotapi.FilePath(video.FilePath))
		videoMsg.SupportsStreaming = true
		_, err := r.botAPI.Send(videoMsg)
		if err != nil {
			return err
		}
	}

	// Upload images as media group if multiple, or single photo
	if len(images) > 0 {
		if len(images) == 1 {
			// Single image - send as photo
			photo := tgbotapi.NewPhoto(groupID, tgbotapi.FilePath(images[0].FilePath))
			_, err := r.botAPI.Send(photo)
			if err != nil {
				return err
			}
		} else {
			// Multiple images - send as media group
			var mediaGroup []interface{}
			for _, img := range images {
				photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(img.FilePath))
				mediaGroup = append(mediaGroup, photo)
			}

			mediaGroupConfig := tgbotapi.NewMediaGroup(groupID, mediaGroup)
			_, err := r.botAPI.SendMediaGroup(mediaGroupConfig)
			if err != nil {
				return err
			}
		}
	}

	// Upload audio files individually
	for _, audio := range audios {
		audioMsg := tgbotapi.NewAudio(groupID, tgbotapi.FilePath(audio.FilePath))
		_, err := r.botAPI.Send(audioMsg)
		if err != nil {
			return err
		}
	}

	return nil
}
