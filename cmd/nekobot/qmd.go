package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/memory/qmd"
)

var qmdCmd = &cobra.Command{
	Use:   "qmd",
	Short: "Manage QMD semantic search",
	Long:  `Manage QMD (Query Markdown) semantic search collections and indexing.`,
}

var qmdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show QMD status",
	Run:   runQMDStatus,
}

var qmdUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update QMD collections",
	Run:   runQMDUpdate,
}

var qmdSearchCmd = &cobra.Command{
	Use:   "search [collection] [query]",
	Short: "Search in a collection",
	Args:  cobra.ExactArgs(2),
	Run:   runQMDSearch,
}

var searchLimit int

func init() {
	qmdSearchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "limit number of results")

	qmdCmd.AddCommand(qmdStatusCmd)
	qmdCmd.AddCommand(qmdUpdateCmd)
	qmdCmd.AddCommand(qmdSearchCmd)
	rootCmd.AddCommand(qmdCmd)
}

func runQMDStatus(cmd *cobra.Command, args []string) {
	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize logger: %v\n", err)
		return
	}

	// Load config
	cfg, err := loadConfigForQMD()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load config: %v\n", err)
		return
	}

	// Create QMD manager
	qmdConfig := qmd.ConfigFromConfig(cfg.Memory.QMD)
	manager := qmd.NewManager(log, qmdConfig)

	// Get status
	status := manager.GetStatus()

	// Print status
	fmt.Println("QMD Status:")
	fmt.Printf("  Available: %v\n", status.Available)
	if status.Available {
		fmt.Printf("  Version: %s\n", status.Version)
		fmt.Printf("  Collections: %d\n", len(status.Collections))
		if !status.LastUpdate.IsZero() {
			fmt.Printf("  Last Update: %s\n", status.LastUpdate.Format(time.RFC3339))
		}

		if len(status.Collections) > 0 {
			fmt.Println("\nCollections:")
			for _, coll := range status.Collections {
				fmt.Printf("  - %s\n", coll.Name)
				fmt.Printf("    Path: %s\n", coll.Path)
				fmt.Printf("    Pattern: %s\n", coll.Pattern)
				if !coll.LastUpdated.IsZero() {
					fmt.Printf("    Last Updated: %s\n", coll.LastUpdated.Format(time.RFC3339))
				}
			}
		}
	} else {
		fmt.Printf("  Error: %s\n", status.Error)
		fmt.Println("\nTo use QMD, install it from: https://github.com/username/qmd")
	}
}

func runQMDUpdate(cmd *cobra.Command, args []string) {
	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize logger: %v\n", err)
		return
	}

	// Load config
	cfg, err := loadConfigForQMD()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load config: %v\n", err)
		return
	}

	// Create QMD manager
	qmdConfig := qmd.ConfigFromConfig(cfg.Memory.QMD)
	manager := qmd.NewManager(log, qmdConfig)

	if !manager.IsAvailable() {
		fmt.Println("QMD is not available. Please install it first.")
		return
	}

	// Initialize collections
	workspaceDir := cfg.WorkspacePath()
	ctx := context.Background()
	if err := manager.Initialize(ctx, workspaceDir); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize: %v\n", err)
		return
	}

	// Update all collections
	fmt.Println("Updating QMD collections...")
	if err := manager.UpdateAll(ctx); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to update: %v\n", err)
		return
	}

	fmt.Println("QMD collections updated successfully")
}

func runQMDSearch(cmd *cobra.Command, args []string) {
	collectionName := args[0]
	query := args[1]

	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize logger: %v\n", err)
		return
	}

	// Load config
	cfg, err := loadConfigForQMD()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load config: %v\n", err)
		return
	}

	// Create QMD manager
	qmdConfig := qmd.ConfigFromConfig(cfg.Memory.QMD)
	manager := qmd.NewManager(log, qmdConfig)

	if !manager.IsAvailable() {
		fmt.Println("QMD is not available. Please install it first.")
		return
	}

	// Search
	ctx := context.Background()
	results, err := manager.Search(ctx, collectionName, query, searchLimit)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Search failed: %v\n", err)
		return
	}

	// Print results
	fmt.Printf("Found %d results for '%s' in collection '%s':\n\n", len(results), query, collectionName)

	for i, result := range results {
		fmt.Printf("%d. %s (score: %.2f)\n", i+1, result.Path, result.Score)
		if result.Snippet != "" {
			fmt.Printf("   %s\n", result.Snippet)
		}
		fmt.Println()
	}
}

func loadConfigForQMD() (*config.Config, error) {
	loader := config.NewLoader()
	return loader.Load(configPath)
}
