package converter

import (
	"strconv"
	"strings"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateToBotEventConverter struct {
	commandConfig env.CommandConfiguration
}

func NewUpdateToBotEventConverter(commandConfig env.CommandConfiguration) *UpdateToBotEventConverter {
	return &UpdateToBotEventConverter{
		commandConfig: commandConfig,
	}
}

func (c *UpdateToBotEventConverter) Convert() core.Codec[tgbotapi.Update, entity.BotEvent] {
	return &UpdateToBotEventCodec{
		commandConfig: c.commandConfig,
	}
}

func (c *UpdateToBotEventConverter) Parse() core.Codec[entity.BotEvent, tgbotapi.Update] {
	return &BotEventToUpdateCodec{}
}

type UpdateToBotEventCodec struct {
	commandConfig env.CommandConfiguration
}

func (c *UpdateToBotEventCodec) Convert(source tgbotapi.Update) entity.BotEvent {
	if source.Message == nil {
		return nil
	}

	message := source.Message
	messageText := strings.TrimSpace(message.Text)
	isGroup := message.Chat.IsGroup() || message.Chat.IsSuperGroup()

	userID := source.SentFrom().ID
	userName := source.SentFrom().UserName

	if isGroup {
		groupID := message.Chat.ID
		return c.parseGroupCommand(messageText, groupID, userID, userName)
	} else {
		return c.parseDirectCommand(messageText, userID, userName)
	}
}

func (c *UpdateToBotEventCodec) parseGroupCommand(messageText string, groupID, userID int64, userName string) entity.BotEvent {
	// Parse the command (first word)
	parts := strings.Fields(messageText)
	if len(parts) == 0 {
		return entity.ErrorGroup{
			GroupID: groupID,
			Message: "Empty command",
		}
	}

	command := parts[0]

	switch command {
	case c.commandConfig.Commands[core.ActivateCommandKey].Command:
		return entity.ActivateGroup{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	case c.commandConfig.Commands[core.DeactivateCommandKey].Command:
		return entity.DeactivateGroup{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	case c.commandConfig.Commands[core.LoadResourceKey].Command:
		// Parse link from message format: /l {link}
		link := ""
		if len(parts) >= 2 {
			link = parts[1]
		}
		return entity.GetResource{
			GroupID: groupID,
			Link:    link,
		}
	case c.commandConfig.Commands[core.GetBotCommandsKey].Command:
		return entity.GroupGetBotCommands{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	default:
		return entity.IgnoreCommand{
			UserID:   userID,
			UserName: userName,
			GroupID:  groupID,
			Command:  command,
		}
	}
}

func (c *UpdateToBotEventCodec) parseDirectCommand(messageText string, userID int64, userName string) entity.BotEvent {
	switch {
	case messageText == c.commandConfig.Commands[core.StartBotKey].Command:
		return entity.StartBot{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == c.commandConfig.Commands[core.GetAllGroupsKey].Command:
		return entity.GetAllGroups{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == c.commandConfig.Commands[core.GetServerLoadKey].Command:
		return entity.GetServerLoad{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == c.commandConfig.Commands[core.GetBotCommandsKey].Command:
		return entity.DirectGetBotCommands{
			UserID:   userID,
			UserName: userName,
		}
	case strings.HasPrefix(messageText, c.commandConfig.Commands[core.DeleteGroupKey].Command+" "):
		groupNameStr := strings.TrimSpace(strings.TrimPrefix(messageText, c.commandConfig.Commands[core.DeleteGroupKey].Command))
		if groupNameStr == "" {
			return entity.ErrorDirect{
				UserID:   userID,
				UserName: userName,
				Message:  "Group name is required for delete command: " + c.commandConfig.Commands[core.DeleteGroupKey].Command + " {GROUP_NAME}",
			}
		}
		groupID, err := strconv.ParseInt(groupNameStr, 10, 64)
		if err != nil {
			return entity.ErrorDirect{
				UserID:   userID,
				UserName: userName,
				Message:  "Invalid group ID: " + groupNameStr,
			}
		}
		return entity.DeleteGroup{
			UserID:   userID,
			UserName: userName,
			GroupID:  groupID,
		}
	default:
		return entity.IgnoreCommand{
			UserID:   userID,
			UserName: userName,
			GroupID:  0, // 0 for direct messages
			Command:  messageText,
		}
	}
}

type BotEventToUpdateCodec struct{}

func (c *BotEventToUpdateCodec) Convert(source entity.BotEvent) tgbotapi.Update {
	return tgbotapi.Update{}
}
