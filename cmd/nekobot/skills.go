package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"nekobot/pkg/config"
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

var skillsValidateCmd = &cobra.Command{
	Use:   "validate [skill-id]",
	Short: "Validate skill dependencies",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsValidate,
}

var skillsInstallDepsCmd = &cobra.Command{
	Use:   "install-deps [skill-id]",
	Short: "Install dependencies for a skill",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsInstallDeps,
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsSourcesCmd)
	skillsCmd.AddCommand(skillsValidateCmd)
	skillsCmd.AddCommand(skillsInstallDepsCmd)
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

	fmt.Println("Skill source paths (in priority order):")
	fmt.Println()
	for _, source := range sources {
		fmt.Printf("[Priority %d] %s (%s)\n", source.Priority, source.Path, source.Type)
	}
}

func runSkillsValidate(cmd *cobra.Command, args []string) {
	manager, err := loadSkillsManager()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load skills manager: %v\n", err)
		return
	}

	report, err := manager.CheckRequirementsReport(strings.TrimSpace(args[0]))
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to validate skill: %v\n", err)
		return
	}

	writeSkillValidationReport(cmd.OutOrStdout(), report)
}

func runSkillsInstallDeps(cmd *cobra.Command, args []string) {
	manager, err := loadSkillsManager()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to load skills manager: %v\n", err)
		return
	}

	skillID := strings.TrimSpace(args[0])
	fmt.Fprintf(cmd.OutOrStdout(), "Installing dependencies for skill %s\n", skillID)

	results, err := manager.InstallDependencies(context.Background(), skillID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to install dependencies: %v\n", err)
		return
	}
	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No dependencies declared.")
		return
	}

	for _, result := range results {
		status := "ok"
		if !result.Success {
			status = "failed"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s %s\n", status, result.Method, result.Package)
		if result.Error != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  error: %v\n", result.Error)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), skills.GetInstallSummary(results))
}

func loadSkillsManager() (*skills.Manager, error) {
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelError,
		Development: true,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	cfg, err := config.NewLoader().Load("")
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	manager, err := skills.ProvideManager(log, cfg)
	if err != nil {
		return nil, fmt.Errorf("provide skills manager: %w", err)
	}
	return manager, nil
}

func writeSkillValidationReport(w io.Writer, report *skills.SkillEntry) {
	if report == nil || report.Skill == nil {
		fmt.Fprintln(w, "No skill report available")
		return
	}

	fmt.Fprintf(w, "Skill: %s\n", report.Skill.Name)
	fmt.Fprintf(w, "ID: %s\n", report.Skill.ID)
	if report.Eligible {
		fmt.Fprintln(w, "Eligible: yes")
	} else {
		fmt.Fprintln(w, "Eligible: no")
	}

	if len(report.Reasons) == 0 {
		fmt.Fprintln(w, "All requirements satisfied.")
		return
	}

	fmt.Fprintln(w, "Missing requirements:")
	for _, reason := range report.Reasons {
		fmt.Fprintf(w, "- %s\n", reason)
	}
}
