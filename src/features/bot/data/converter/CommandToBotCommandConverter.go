package converter

import (
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CommandToBotCommandConverter struct{}

func NewCommandToBotCommandConverter() *CommandToBotCommandConverter {
	return &CommandToBotCommandConverter{}
}

func (c *CommandToBotCommandConverter) Convert() core.Codec[entity.Command, tgbotapi.BotCommand] {
	return &CommandToBotCommandCodec{}
}

func (c *CommandToBotCommandConverter) Parse() core.Codec[tgbotapi.BotCommand, entity.Command] {
	return &BotCommandToCommandCodec{}
}

type CommandToBotCommandCodec struct{}

func (c *CommandToBotCommandCodec) Convert(source entity.Command) tgbotapi.BotCommand {
	return tgbotapi.BotCommand{
		Command:     source.Command,
		Description: source.Description,
	}
}

type BotCommandToCommandCodec struct{}

func (c *BotCommandToCommandCodec) Convert(source tgbotapi.BotCommand) entity.Command {
	return entity.Command{
		Command:     source.Command,
		Description: source.Description,
	}
}