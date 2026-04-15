package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

var daemonAddr string
var daemonMachineName string
var daemonServerURL string
var daemonToken string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run Nekobot host daemon",
}

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Nekobot host daemon HTTP service",
	Run:   runDaemon,
}

func init() {
	daemonRunCmd.Flags().StringVar(&daemonAddr, "addr", daemonhost.DefaultAddr, "daemon listen address")
	daemonRunCmd.Flags().StringVar(&daemonMachineName, "machine-name", "", "machine display name")
	daemonRunCmd.Flags().StringVar(&daemonServerURL, "server-url", "", "optional server URL for machine registration/heartbeat")
	daemonRunCmd.Flags().StringVar(&daemonToken, "token", "", "daemon bearer token for server registration/heartbeat")
	daemonCmd.AddCommand(daemonRunCmd)
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) {
	cfg, err := config.NewLoader().Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{
		FilePath:     filepath.Join(cfg.Storage.DBDir, "daemon-state.json"),
		AutoSave:     true,
		SaveInterval: 5,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create daemon state store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if daemonServerURL != "" {
		daemonURL := "http://" + daemonAddr
		client := daemonhost.NewAuthedClient(daemonServerURL, daemonToken)
		go func() {
			if err := daemonhost.RegisterAndPoll(ctx, client, daemonhost.PollOptions{
				MachineName: daemonMachineName,
				DaemonURL:   daemonURL,
			}); err != nil && ctx.Err() == nil {
				log.Warn("Daemon remote polling stopped", zap.Error(err))
			}
		}()
	}
	server := daemonhost.NewServer(daemonAddr, daemonMachineName, store)
	log.Info("Starting daemon host", zap.String("addr", daemonAddr))
	if err := server.Serve(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "serve daemon host: %v\n", err)
		os.Exit(1)
	}
}
