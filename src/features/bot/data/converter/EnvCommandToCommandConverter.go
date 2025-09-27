package converter

import (
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/domain/entity"
)

type EnvCommandToCommandConverter struct{}

func NewEnvCommandToCommandConverter() *EnvCommandToCommandConverter {
	return &EnvCommandToCommandConverter{}
}

func (c *EnvCommandToCommandConverter) Convert() core.Codec[env.Command, entity.Command] {
	return &EnvCommandToCommandCodec{}
}

func (c *EnvCommandToCommandConverter) Parse() core.Codec[entity.Command, env.Command] {
	return &CommandToEnvCommandCodec{}
}

type EnvCommandToCommandCodec struct{}

func (c *EnvCommandToCommandCodec) Convert(source env.Command) entity.Command {
	return entity.Command{
		Command:     source.Command,
		Description: source.Description,
	}
}

type CommandToEnvCommandCodec struct{}

func (c *CommandToEnvCommandCodec) Convert(source entity.Command) env.Command {
	return env.Command{
		Command:     source.Command,
		Description: source.Description,
	}
}