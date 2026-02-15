package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/workspace"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize nekobot workspace and configuration",
	Long: `Interactive setup wizard for first-time nekobot users.

This command will:
- Create workspace directory structure
- Generate configuration files
- Initialize memory and skills directories
- Create bootstrap files with templates

Run this once when setting up nekobot for the first time.`,
	Run: runOnboard,
}

func onboardDefaultConfig(workspaceDir string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspaceDir
	return cfg
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

func runOnboard(cmd *cobra.Command, args []string) {
	fmt.Println("🐾 Welcome to Nekobot!")
	fmt.Println("")
	fmt.Println("Let's set up your workspace and configuration.")
	fmt.Println("")

	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Get default workspace
	defaultWorkspace, err := workspace.GetDefaultWorkspaceDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get default workspace: %v\n", err)
		os.Exit(1)
	}

	// Ask for workspace location
	workspaceDir := defaultWorkspace
	fmt.Printf("Workspace location: [%s] ", defaultWorkspace)
	var input string
	fmt.Scanln(&input)
	if input != "" {
		workspaceDir = input
	}
	fmt.Println("")

	// Expand ~ to home directory
	if len(workspaceDir) > 0 && workspaceDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			workspaceDir = filepath.Join(home, workspaceDir[1:])
		}
	}

	// Create workspace manager
	wm := workspace.NewManager(workspaceDir, log)

	// Initialize workspace
	fmt.Println("📁 Creating workspace structure...")
	if err := wm.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create workspace: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Workspace created at: %s\n", workspaceDir)
	fmt.Println("")

	targetConfigPath := strings.TrimSpace(configPath)
	if targetConfigPath == "" {
		configHome, err := config.GetConfigHome()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get config home: %v\n", err)
			os.Exit(1)
		}
		targetConfigPath = filepath.Join(configHome, "config.json")
	}
	targetConfigPath, err = filepath.Abs(targetConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve config path: %v\n", err)
		os.Exit(1)
	}
	configPath = targetConfigPath

	cfg := onboardDefaultConfig(workspaceDir)
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
		loader := config.NewLoader()
		loaded, loadErr := loader.LoadFromFile(configPath)
		if loadErr == nil {
			cfg = loaded
		}
	}
	cfg.Agents.Defaults.Workspace = workspaceDir
	cfg.Storage.DBDir = filepath.Dir(configPath)

	if err := config.SaveToFile(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	dbPath, err := config.EnsureRuntimeDBFile(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize runtime database: %v\n", err)
		os.Exit(1)
	}

	if configExists {
		fmt.Printf("📝 Configuration updated at: %s\n", configPath)
	} else {
		fmt.Printf("✅ Configuration created at: %s\n", configPath)
		fmt.Println("")
		fmt.Println("⚠️  Important: Add your API key to the config file:")
		fmt.Printf("   Edit: %s\n", configPath)
		fmt.Println("   Set: providers.<provider>.api_key")
	}
	fmt.Printf("🗄️ Runtime database ready: %s\n", dbPath)

	fmt.Println("")
	fmt.Println("🎉 Onboarding complete!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("")
	fmt.Println("1. Configure API keys:")
	fmt.Printf("   Edit %s\n", configPath)
	fmt.Println("   Add your Anthropic/OpenAI API key")
	fmt.Println("")
	fmt.Println("2. Personalize your workspace:")
	fmt.Printf("   cd %s\n", workspaceDir)
	fmt.Println("   Edit SOUL.md, IDENTITY.md, USER.md")
	fmt.Println("")
	fmt.Println("3. Start using nekobot:")
	fmt.Println("   nekobot agent -m \"Hello!\"")
	fmt.Println("")
	fmt.Println("4. Explore documentation:")
	fmt.Printf("   cat %s/BOOTSTRAP.md\n", workspaceDir)
	fmt.Println("")
	fmt.Println("Happy hacking! 🚀")
}
