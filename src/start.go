package src

import (
	"context"
	"log"
	controller "tg-downloader/src/features/bot/interface"

	"go.uber.org/fx"
)

func StartBot(lc fx.Lifecycle, botController controller.IBotController) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("Starting bot...")
			botController.Initialize()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Bot stopped gracefully")
			botController.Dispose()
			return nil
		},
	})
}
