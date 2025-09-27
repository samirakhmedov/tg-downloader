package src

import (
	"context"
	"log"
	"tg-downloader/ent"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/data/repository"
	controller "tg-downloader/src/features/bot/interface"
	i "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/bot/domain/service"
	systemRepo "tg-downloader/src/features/system/data/repository"
	iSystemRepo "tg-downloader/src/features/system/domain/repository"
	videoRepo "tg-downloader/src/features/video/data/repository"
	iVideoRepo "tg-downloader/src/features/video/domain/repository"
	videoService "tg-downloader/src/features/video/domain/service"

	"entgo.io/ent/dialect/sql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/fx"
)

func NewBotConfiguration() env.TGDownloader {
	cfg, err := env.LoadFromPath(context.Background(), core.DownloaderConfigPath)

	if err != nil {
		log.Fatal("Failed to load configuration of bot. Error: ", err)
	}

	return cfg
}

func NewDatabase(cfg env.TGDownloader, lc fx.Lifecycle) *ent.Client {
	drv, err := sql.Open(core.DatabaseDriver, core.DatabaseSource)

	if err != nil {
		log.Fatalf("Failed opening connection to %s: %v", core.DatabaseDriver, err)
	}

	var client = ent.NewClient(ent.Driver(drv))

	if cfg.Debug {
		client = client.Debug()
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("Running migrations...")

			return client.Schema.Create(context.Background())
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Closing database connection...")
			return client.Close()
		},
	})

	return client
}

func NewBotAPI(cfg env.TGDownloader, lc fx.Lifecycle) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(cfg.BotConfiguration.TgBotApiKey)

	if err != nil {
		log.Fatal("Failed to create bot instance. Error: ", err)
	}

	bot.Debug = cfg.Debug

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Println("Stopping bot updates...")
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

func NewBotService(botRepo i.IBotRepository, cacheRepo i.IBotCacheRepository, systemRepo iSystemRepo.ISystemRepository, cfg env.TGDownloader) service.IBotService {
	return service.NewBotService(botRepo, cacheRepo, systemRepo, cfg)
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

func NewVideoService(cfg env.TGDownloader, taskRepo i.ITaskRepository, downloadRepo iVideoRepo.IVideoDownloadRepository, uploadRepo iVideoRepo.IUploadRepository) videoService.IVideoService {
	return videoService.NewVideoService(cfg, taskRepo, downloadRepo, uploadRepo)
}

func NewBotController(botService service.IBotService, videoService videoService.IVideoService) controller.IBotController {
	return controller.NewBotController(botService, videoService)
}
