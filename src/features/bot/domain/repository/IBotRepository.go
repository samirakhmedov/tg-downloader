package repository

import "tg-downloader/src/features/bot/domain/entity"

type IBotRepository interface {
	ReceiveEvents() entity.BotEvents

	IsAdmin(userName string) (bool, error)

	SetCommandsForDirectMessages(userID int64, commands []entity.Command) error
	SetCommandsForChatMember(chatID int64, userID int64, commands []entity.Command) error

	SendDirectMessage(userID int64, message string) error
	SendGroupMessage(chatID int64, message string) error
	SendGroupMessageWithID(chatID int64, message string) (int, error)

	UpdateDirectMessage(userID int64, messageID int, newText string) error
	UpdateGroupMessage(chatID int64, messageID int, newText string) error

	DeleteGroupMessage(chatID int64, messageID int) error

	GetChatInfo(chatID int64) (*entity.ChatInfo, error)
}
