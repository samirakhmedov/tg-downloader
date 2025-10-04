package core

const (
	DownloaderConfigPath = "config/Config.pkl"
	DatabaseDriver       = "sqlite3"
	DatabaseSource       = "file:database.db?_fk=1&_journal_mode=WAL"
	ActivateCommandKey   = "activateGroup"
	DeactivateCommandKey = "deactivateGroup"
	GetBotCommandsKey    = "getBotCommands"
	GetServerLoadKey     = "getServerLoad"
	LoadResourceKey      = "loadResource"
	GetAllGroupsKey      = "getAllGroups"
	DeleteGroupKey       = "deleteGroup"
	StartBotKey          = "start"

	// Media processing constants
	MediaTempDirectory   = "temp/media"
	MediaOutputDirectory = "output/media"
	URLRegexPattern      = `^https?://[^\s/$.?#].[^\s]*$`
)

var (
	BotAllowedUpdates = []string{
		"message",
	}
)
