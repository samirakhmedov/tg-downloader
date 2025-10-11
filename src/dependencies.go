package src

import (
	"context"
	"fmt"
	"log"
	"tg-downloader/ent"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/core/logger"
	"tg-downloader/src/features/bot/data/repository"
	i "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/bot/domain/service"
	controller "tg-downloader/src/features/bot/interface"
	systemRepo "tg-downloader/src/features/system/data/repository"
	iSystemRepo "tg-downloader/src/features/system/domain/repository"
	videoRepo "tg-downloader/src/features/video/data/repository"
	iVideoRepo "tg-downloader/src/features/video/domain/repository"
	videoService "tg-downloader/src/features/video/domain/service"

	"entgo.io/ent/dialect/sql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func NewBotConfiguration() env.TGDownloader {
	cfg, err := env.LoadFromPath(context.Background(), core.DownloaderConfigPath)

	if err != nil {
		log.Fatal("Failed to load configuration of bot. Error: ", err)
	}

	return cfg
}

func NewDatabase(cfg env.TGDownloader, logger *logger.Logger, lc fx.Lifecycle) *ent.Client {
	drv, err := sql.Open(core.DatabaseDriver, core.DatabaseSource)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed opening connection to %s: %v", core.DatabaseDriver, err))
	}

	var client = ent.NewClient(ent.Driver(drv))

	if cfg.Debug {
		client = client.Debug()
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Running migrations...")

			return client.Schema.Create(context.Background())
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing database connection...")
			return client.Close()
		},
	})

	return client
}

func NewBotAPI(cfg env.TGDownloader, lc fx.Lifecycle, logger *logger.Logger) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramConfiguration.TgBotApiKey)

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create bot instance. Error: %s", err))
	}

	bot.Debug = cfg.Debug

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping bot updates...")
			bot.StopReceivingUpdates()
			return nil
		},
	})

	return bot
}

func NewBotRepository(cfg env.TGDownloader, botApi *tgbotapi.BotAPI) i.IBotRepository {
	repo := repository.NewBotRepository(cfg, botApi)

	return repo
}

func NewBotCacheRepository(database *ent.Client) i.IBotCacheRepository {
	repo := repository.NewBotCacheRepository(database)

	return repo
}

func NewSystemRepository() iSystemRepo.ISystemRepository {
	return systemRepo.NewSystemRepository()
}

func NewBotService(botRepo i.IBotRepository, cacheRepo i.IBotCacheRepository, systemRepo iSystemRepo.ISystemRepository, cfg env.TGDownloader, logger *logger.Logger) service.IBotService {
	return service.NewBotService(botRepo, cacheRepo, systemRepo, cfg, logger)
}

func NewTaskRepository(database *ent.Client) i.ITaskRepository {
	return repository.NewTaskRepository(database)
}

func NewVideoDownloadRepository(cfg env.TGDownloader) iVideoRepo.IVideoDownloadRepository {
	return videoRepo.NewVideoDownloadRepository(cfg)
}

func NewUploadRepository(botApi *tgbotapi.BotAPI) iVideoRepo.IUploadRepository {
	return videoRepo.NewUploadRepository(botApi)
}

func NewVideoService(cfg env.TGDownloader, taskRepo i.ITaskRepository, downloadRepo iVideoRepo.IVideoDownloadRepository, uploadRepo iVideoRepo.IUploadRepository, logger *logger.Logger) videoService.IVideoService {
	return videoService.NewVideoService(cfg, taskRepo, downloadRepo, uploadRepo, logger)
}

func NewBotController(botService service.IBotService, videoService videoService.IVideoService, logger *logger.Logger) controller.IBotController {
	return controller.NewBotController(botService, videoService, logger)
}

// NewLoggerStrategies creates a list of logger strategies.
// Currently includes only the debug logger strategy, but can be extended with additional strategies.
func NewLoggerStrategies(cfg env.TGDownloader) []logger.ILoggerStrategy {
	return []logger.ILoggerStrategy{logger.NewDebugLoggerStrategy(cfg.Debug)}
}

// NewLogger creates a new Logger with the provided list of strategies.
// The logger will delegate all log calls to each strategy in the list.
func NewLogger(strategies []logger.ILoggerStrategy) *logger.Logger {
	return logger.NewLogger(strategies)
}

// NewFxLogger creates a new FX event logger that uses our custom Logger
func NewFxLogger(logger *logger.Logger) fxevent.Logger {
	return logger.NewFxLogger(logger)
}
