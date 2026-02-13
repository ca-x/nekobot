package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
	Long:  `List, enable, disable, and manage agent skills.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available skills",
	Run:   runSkillsList,
}

var skillsSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List skill source paths",
	Run:   runSkillsSources,
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsSourcesCmd)
	rootCmd.AddCommand(skillsCmd)
}

func runSkillsList(cmd *cobra.Command, args []string) {
	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize logger: %v\n", err)
		return
	}

	// Create loader
	loader := skills.NewMultiPathLoader(log, "")

	// Load all skills
	allSkills, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load skills: %v\n", err)
		return
	}

	fmt.Printf("Found %d skills:\n\n", len(allSkills))
	for id, skill := range allSkills {
		status := "✓"
		if !skill.Enabled {
			status = "✗"
		}
		fmt.Printf("[%s] %-20s %s\n", status, id, skill.Name)
		if skill.Description != "" {
			fmt.Printf("    %s\n", skill.Description)
		}
		fmt.Printf("    Source: %s\n\n", skill.FilePath)
	}
}

func runSkillsSources(cmd *cobra.Command, args []string) {
	// Initialize logger
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelInfo,
		Development: true,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to initialize logger: %v\n", err)
		return
	}

	// Create loader
	loader := skills.NewMultiPathLoader(log, "")

	// Get sources
	sources := loader.GetSources()

	fmt.Println("Skill source paths (in priority order):\n")
	for _, source := range sources {
		fmt.Printf("[Priority %d] %s (%s)\n", source.Priority, source.Path, source.Type)
	}
}
