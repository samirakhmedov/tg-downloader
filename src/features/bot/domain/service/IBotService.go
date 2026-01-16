package service

import "tg-downloader/src/features/bot/domain/entity"

type IBotService interface {
	UpdateCommandsForUser(userID int64, userName string) error
	UpdateCommandsForGroupUser(chatID int64, userID int64, userName string) error
	GetBotEvents() entity.BotEvents
	ActivateGroup(groupID int64, userID int64, userName string) error
	DeactivateGroup(groupID int64, userID int64, userName string) error
	DeleteGroup(groupID int64, userID int64, userName string) error
	GetAllGroups(userID int64, userName string) error
	GetServerLoad(userID int64, userName string) error
	GetDirectCommands(userID int64, userName string) error
	GetGroupCommands(groupID int64, userID int64, userName string) error
	HandleDirectError(userID int64, userName string, message string) error
	HandleGroupError(groupID int64, message string) error
	LoadResource(groupID int64, link string) (messageID int, canProcess bool, err error)
	HandleVideoUploadStarted(groupID int64, messageID int) error
	HandleVideoProcessSuccess(groupID int64, messageID int) error
	HandleVideoProcessFailure(groupID int64, messageID int, errorMessage string) error
}
