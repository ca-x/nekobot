package main

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func main() {
	app := fx.New(
		// Core modules
		logger.Module,
		config.Module,
		agent.Module,

		// Application logic
		fx.Invoke(run),
	)

	app.Run()
}

func run(
	lc fx.Lifecycle,
	log *logger.Logger,
	cfg *config.Config,
	ag *agent.Agent,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Starting NekoBot demo")

			// Example: simple chat
			response, err := ag.Chat(ctx, "Hello! Please list the files in the current directory using the list_dir tool.")
			if err != nil {
				log.Error("Chat failed", zap.Error(err))
				return err
			}

			fmt.Println("\n=== Agent Response ===")
			fmt.Println(response)
			fmt.Println("===================")

			// Example: write and read file
			response2, err := ag.Chat(ctx, "Please create a file called 'test.txt' with the content 'Hello from NekoBot!' using write_file tool.")
			if err != nil {
				log.Error("Chat failed", zap.Error(err))
				return err
			}

			fmt.Println("\n=== Agent Response 2 ===")
			fmt.Println(response2)
			fmt.Println("========================")

			return nil
		},
	})
}
