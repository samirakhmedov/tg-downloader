package repository

import (
	"strings"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/data/converter"
	"tg-downloader/src/features/bot/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotRepository struct {
	environment      env.TGDownloader
	botApi           *tgbotapi.BotAPI
	converter        *converter.UpdateToBotEventConverter
	commandConverter *converter.CommandToBotCommandConverter
	chatConverter    *converter.ChatToChatInfoConverter
}

func NewBotRepository(environment env.TGDownloader, botApi *tgbotapi.BotAPI) *BotRepository {
	return &BotRepository{
		environment:      environment,
		botApi:           botApi,
		converter:        converter.NewUpdateToBotEventConverter(environment),
		commandConverter: converter.NewCommandToBotCommandConverter(),
		chatConverter:    converter.NewChatToChatInfoConverter(),
	}
}

func (r *BotRepository) ReceiveEvents() entity.BotEvents {
	ch := make(chan entity.BotEvent, r.environment.TelegramConfiguration.UpdateLimit)

	go func() {
		u := tgbotapi.NewUpdate(core.BotBegginingUpdateOffset)
		u.Timeout = r.environment.TelegramConfiguration.UpdateTimeout
		u.AllowedUpdates = core.BotAllowedUpdates
		u.Limit = r.environment.TelegramConfiguration.UpdateLimit

		channel := r.botApi.GetUpdatesChan(u)

		for update := range channel {
			codec := r.converter.Convert()
			botEvent := codec.Convert(update)

			if botEvent != nil {
				ch <- botEvent
			}
		}
	}()

	return ch
}

func (r *BotRepository) IsAdmin(userName string) (bool, error) {
	formattedName := strings.ToLower(strings.TrimSpace(userName))
	for _, v := range r.environment.AuthConfiguration.Admininstrators {
		formattedAdminName := strings.ToLower(strings.TrimSpace(v.UserName))

		if formattedName == formattedAdminName {
			return true, nil
		}
	}

	return false, nil
}

func (r *BotRepository) SetCommandsForDirectMessages(userID int64, commands []entity.Command) error {
	botCommands := r.convertCommands(commands)

	setCommands := tgbotapi.NewSetMyCommandsWithScope(
		tgbotapi.NewBotCommandScopeChat(userID),
		botCommands...,
	)

	_, err := r.botApi.Request(setCommands)
	return err
}

func (r *BotRepository) SetCommandsForChatMember(chatID int64, userID int64, commands []entity.Command) error {
	botCommands := r.convertCommands(commands)

	setCommands := tgbotapi.NewSetMyCommandsWithScope(
		tgbotapi.NewBotCommandScopeChatMember(chatID, userID),
		botCommands...,
	)

	_, err := r.botApi.Request(setCommands)
	return err
}

func (r *BotRepository) convertCommands(commands []entity.Command) []tgbotapi.BotCommand {
	codec := r.commandConverter.Convert()
	botCommands := make([]tgbotapi.BotCommand, len(commands))

	for i, cmd := range commands {
		botCommands[i] = codec.Convert(cmd)
	}

	return botCommands
}

func (r *BotRepository) SendDirectMessage(userID int64, message string) error {
	msg := tgbotapi.NewMessage(userID, message)
	_, err := r.botApi.Send(msg)
	return err
}

func (r *BotRepository) SendGroupMessage(chatID int64, message string) error {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := r.botApi.Send(msg)
	return err
}

func (r *BotRepository) UpdateDirectMessage(userID int64, messageID int, newText string) error {
	edit := tgbotapi.NewEditMessageText(userID, messageID, newText)
	_, err := r.botApi.Send(edit)
	return err
}

func (r *BotRepository) UpdateGroupMessage(chatID int64, messageID int, newText string) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, newText)
	_, err := r.botApi.Send(edit)
	return err
}

func (r *BotRepository) GetChatInfo(chatID int64) (*entity.ChatInfo, error) {
	chatConfig := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	chat, err := r.botApi.GetChat(chatConfig)
	if err != nil {
		return nil, err
	}

	codec := r.chatConverter.Convert()
	chatInfo := codec.Convert(chat)
	return &chatInfo, nil
}
