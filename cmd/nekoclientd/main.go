package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"nekobot/pkg/config"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/servicecontrol"
)

var (
	configPath       string
	serverTarget     string
	grpcToken        string
	machineName      string
	inventoryHomeDir string
)

type clientProgram struct{ cancel context.CancelFunc }

func main() {
	root := &cobra.Command{Use: "nekoclientd", Short: "Nekobot remote agent daemon client"}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if configPath == "" {
			return nil
		}
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("resolve config path: %w", err)
		}
		return os.Setenv(config.ConfigPathEnv, abs)
	}
	runCmd := &cobra.Command{Use: "run", Short: "Run nekoclientd", Run: runClient}
	runCmd.Flags().StringVar(&serverTarget, "server", daemonhost.DefaultAddr, "grpc server target host:port")
	runCmd.Flags().StringVar(&grpcToken, "token", "", "daemon auth token")
	runCmd.Flags().StringVar(&machineName, "machine-name", "", "machine display name")
	runCmd.Flags().StringVar(&inventoryHomeDir, "home", "", "inventory home dir override")
	root.AddCommand(runCmd)
	root.AddCommand(&cobra.Command{Use: "install", RunE: func(cmd *cobra.Command, args []string) error { return installClientService() }})
	root.AddCommand(&cobra.Command{Use: "uninstall", RunE: func(cmd *cobra.Command, args []string) error { return uninstallClientService() }})
	root.AddCommand(&cobra.Command{Use: "start", RunE: func(cmd *cobra.Command, args []string) error { return startClientService() }})
	root.AddCommand(&cobra.Command{Use: "stop", RunE: func(cmd *cobra.Command, args []string) error { return stopClientService() }})
	root.AddCommand(&cobra.Command{Use: "restart", RunE: func(cmd *cobra.Command, args []string) error { return restartClientService() }})
	root.AddCommand(&cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error { return statusClientService() }})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runClient(cmd *cobra.Command, args []string) {
	isService := os.Getenv("INVOCATION_ID") != "" || os.Getenv("_") == "/bin/launchd" || os.Getenv("SERVICE_NAME") != ""
	if isService {
		if err := runClientService(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runClientLoop(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runClientLoop(ctx context.Context) error {
	client, err := daemonhost.NewGRPCClient(serverTarget, grpcToken)
	if err != nil {
		return err
	}
	defer client.Close()
	return daemonhost.RegisterAndPoll(ctx, client, daemonhost.PollOptions{MachineName: machineName, InventoryHomeDir: inventoryHomeDir, PollInterval: 15 * time.Second})
}

func runClientService() error {
	prg := &clientProgram{}
	svc, err := service.New(prg, servicecontrol.NekoClientdConfig(configPath))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	return svc.Run()
}
func (p *clientProgram) Start(s service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func() { _ = runClientLoop(ctx) }()
	return nil
}
func (p *clientProgram) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}
func installClientService() error {
	svc, err := service.New(&clientProgram{}, servicecontrol.NekoClientdConfig(configPath))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	if err := svc.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}
	return nil
}
func uninstallClientService() error {
	svc, err := service.New(&clientProgram{}, servicecontrol.NekoClientdConfig(configPath))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling service: %w", err)
	}
	return nil
}
func startClientService() error {
	svc, err := service.New(&clientProgram{}, servicecontrol.NekoClientdConfig(configPath))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	if err := svc.Start(); err != nil {
		return fmt.Errorf("starting service: %w", err)
	}
	return nil
}
func stopClientService() error {
	svc, err := service.New(&clientProgram{}, servicecontrol.NekoClientdConfig(configPath))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	if err := svc.Stop(); err != nil {
		return fmt.Errorf("stopping service: %w", err)
	}
	return nil
}
func restartClientService() error { return servicecontrol.RestartNekoClientdService(configPath) }
func statusClientService() error {
	_, err := servicecontrol.InspectNekoClientdService(configPath)
	return err
}
