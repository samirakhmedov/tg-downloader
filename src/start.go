package src

import (
	"context"
	"tg-downloader/src/core/logger"
	controller "tg-downloader/src/features/bot/interface"

	"go.uber.org/fx"
)

func StartBot(lc fx.Lifecycle, botController controller.IBotController, logger *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting bot...")
			botController.Initialize()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Bot stopped gracefully")
			botController.Dispose()
			return nil
		},
	})
}
