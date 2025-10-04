package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"tg-downloader/src"

	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.Provide(
			src.NewBotConfiguration,
		),
		fx.Provide(
			src.NewDebugLoggerStrategy,
		),
		fx.Provide(
			src.NewLoggerStrategies,
		),
		fx.Provide(
			src.NewLogger,
		),
		fx.WithLogger(src.NewFxLogger),
		fx.Provide(
			src.NewBotAPI,
		),
		fx.Provide(
			src.NewDatabase,
		),
		fx.Provide(
			src.NewBotRepository,
		),
		fx.Provide(
			src.NewSystemRepository,
		),
		fx.Provide(
			src.NewBotCacheRepository,
		),
		fx.Provide(
			src.NewTaskRepository,
		),
		fx.Provide(
			src.NewDownloadRepository,
		),
		fx.Provide(
			src.NewUploadRepository,
		),
		fx.Provide(
			src.NewBotService,
		),
		fx.Provide(
			src.NewMediaService,
		),
		fx.Provide(
			src.NewBotController,
		),
		fx.Invoke(src.StartBot),
	)

	// Create a context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the application
	if err := app.Start(ctx); err != nil {
		panic(err)
	}

	// Wait for interrupt signal to gracefully shutdown
	<-ctx.Done()

	// Stop the application gracefully
	if err := app.Stop(context.Background()); err != nil {
		panic(err)
	}
}
