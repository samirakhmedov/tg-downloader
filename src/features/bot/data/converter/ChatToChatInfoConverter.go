package converter

import (
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ChatToChatInfoConverter struct{}

func NewChatToChatInfoConverter() *ChatToChatInfoConverter {
	return &ChatToChatInfoConverter{}
}

func (c *ChatToChatInfoConverter) Convert() core.Codec[tgbotapi.Chat, entity.ChatInfo] {
	return &ChatToChatInfoCodec{}
}

type ChatToChatInfoCodec struct{}

func (c *ChatToChatInfoCodec) Convert(source tgbotapi.Chat) entity.ChatInfo {
	return entity.ChatInfo{
		ID:    source.ID,
		Title: source.Title,
		Type:  source.Type,
	}
}