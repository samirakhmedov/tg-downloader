package converter

import (
	"regexp"
	"strconv"
	"strings"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateToBotEventConverter struct {
	environment env.TGDownloader
}

func NewUpdateToBotEventConverter(environment env.TGDownloader) *UpdateToBotEventConverter {
	return &UpdateToBotEventConverter{
		environment: environment,
	}
}

func (c *UpdateToBotEventConverter) Convert() core.Codec[tgbotapi.Update, entity.BotEvent] {
	return &UpdateToBotEventCodec{
		environment: c.environment,
	}
}

func (c *UpdateToBotEventConverter) Parse() core.Codec[entity.BotEvent, tgbotapi.Update] {
	return &BotEventToUpdateCodec{}
}

type UpdateToBotEventCodec struct {
	environment env.TGDownloader
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
		return c.parseGroupCommand(message, groupID, userID, userName)
	} else {
		return c.parseDirectCommand(messageText, userID, userName)
	}
}

func (c *UpdateToBotEventCodec) parseGroupCommand(message *tgbotapi.Message, groupID, userID int64, userName string) entity.BotEvent {
	commands := c.environment.CommandConfiguration.Commands
	messageText := strings.TrimSpace(message.Text)

	// Parse the command (first word)
	parts := strings.Fields(messageText)
	if len(parts) == 0 {
		return entity.IgnoreCommand{
			UserID:   userID,
			UserName: userName,
			GroupID:  groupID,
			Command:  "",
		}
	}

	command := c.extractBotNameFromCommand(parts[0])

	switch command {
	case commands[core.ActivateCommandKey].Command:
		return entity.ActivateGroup{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	case commands[core.DeactivateCommandKey].Command:
		return entity.DeactivateGroup{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	case commands[core.LoadResourceKey].Command:
		// Handle load resource command with multiple scenarios:
		// 1. Reply to message containing link: /l (as reply)
		// 2. Direct command with link: /l {link}
		link := ""

		// Check if this is a reply to another message
		if message.ReplyToMessage != nil {
			// Extract link from the replied-to message using flexible extraction
			replyText := strings.TrimSpace(message.ReplyToMessage.Text)
			if hasLink, foundLink := c.extractLinkFromText(replyText); hasLink {
				link = foundLink
			}
		} else if len(parts) >= 2 {
			// Traditional format: /l {link}
			link = parts[1]
		}

		return entity.GetResource{
			GroupID: groupID,
			Link:    link,
		}
	case commands[core.GetBotCommandsKey].Command:
		return entity.GroupGetBotCommands{
			GroupID:  groupID,
			UserID:   userID,
			UserName: userName,
		}
	default:
		// Check if message contains a supported link
		if hasLink, link := c.containsSupportedLink(messageText); hasLink {
			return entity.GetResource{
				GroupID: groupID,
				Link:    link,
			}
		}

		return entity.IgnoreCommand{
			UserID:   userID,
			UserName: userName,
			GroupID:  groupID,
			Command:  command,
		}
	}
}

func (c *UpdateToBotEventCodec) parseDirectCommand(messageText string, userID int64, userName string) entity.BotEvent {
	commands := c.environment.CommandConfiguration.Commands

	switch {
	case messageText == commands[core.StartBotKey].Command:
		return entity.StartBot{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == commands[core.GetAllGroupsKey].Command:
		return entity.GetAllGroups{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == commands[core.GetServerLoadKey].Command:
		return entity.GetServerLoad{
			UserID:   userID,
			UserName: userName,
		}
	case messageText == commands[core.GetBotCommandsKey].Command:
		return entity.DirectGetBotCommands{
			UserID:   userID,
			UserName: userName,
		}
	case strings.HasPrefix(messageText, commands[core.DeleteGroupKey].Command+" "):
		groupNameStr := strings.TrimSpace(strings.TrimPrefix(messageText, commands[core.DeleteGroupKey].Command))
		if groupNameStr == "" {
			return entity.ErrorDirect{
				UserID:   userID,
				UserName: userName,
				Message:  "Group name is required for delete command: " + commands[core.DeleteGroupKey].Command + " {GROUP_NAME}",
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

// Helper functions for link detection and bot name parsing

// containsSupportedLink checks if the text contains any supported link pattern and returns the first match
func (c *UpdateToBotEventCodec) containsSupportedLink(text string) (bool, string) {
	for _, linkPattern := range c.environment.CommandConfiguration.SupportedLinks {
		re, err := regexp.Compile(linkPattern.Pattern)
		if err != nil {
			continue
		}
		if match := re.FindString(text); match != "" {
			return true, match
		}
	}
	return false, ""
}

// extractLinkFromText extracts links from text even when surrounded by other content (for reply scenarios)
func (c *UpdateToBotEventCodec) extractLinkFromText(text string) (bool, string) {
	for _, linkPattern := range c.environment.CommandConfiguration.SupportedLinks {
		// Convert anchored pattern to flexible pattern that stops at word boundaries
		flexiblePattern := strings.TrimPrefix(linkPattern.Pattern, "^")
		flexiblePattern = strings.TrimSuffix(flexiblePattern, "$")

		// Replace .* with [^\s]* to stop at whitespace boundaries for URL parameters
		flexiblePattern = strings.ReplaceAll(flexiblePattern, ".*", "[^\\s]*")

		re, err := regexp.Compile(flexiblePattern)
		if err != nil {
			continue
		}
		if match := re.FindString(text); match != "" {
			return true, match
		}
	}
	return false, ""
}

// extractBotNameFromCommand removes bot name suffix from command (e.g., "/l@botname" -> "/l")
func (c *UpdateToBotEventCodec) extractBotNameFromCommand(command string) string {
	botName := c.environment.TelegramConfiguration.BotName
	suffix := "@" + botName
	if strings.HasSuffix(command, suffix) {
		return strings.TrimSuffix(command, suffix)
	}
	return command
}
