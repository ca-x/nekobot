package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/cron"
	"nekobot/pkg/logger"
	"nekobot/pkg/providers"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/tools"
	"nekobot/pkg/workspace"
)

var (
	cronJobID  string
	cronName   string
	cronPrompt string
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage cron jobs",
	Long:  `Manage cron jobs for scheduled agent tasks.`,
}

var cronAddCmd = &cobra.Command{
	Use:   "add <schedule>",
	Short: "Add a new cron job",
	Long: `Add a new cron job with the specified schedule.

Schedule format: Standard cron expression (5 fields)
  - Minute (0-59)
  - Hour (0-23)
  - Day of month (1-31)
  - Month (1-12)
  - Day of week (0-6, Sunday=0)

Examples:
  # Every hour at minute 0
  nekobot cron add "0 * * * *" --name "Hourly Check" --prompt "Check system status"

  # Every day at 9:00 AM
  nekobot cron add "0 9 * * *" --name "Morning Report" --prompt "Generate daily report"

  # Every Monday at 10:00 AM
  nekobot cron add "0 10 * * 1" --name "Weekly Summary" --prompt "Create weekly summary"`,
	Args: cobra.ExactArgs(1),
	Run:  runCronAdd,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron jobs",
	Long:  `Display all configured cron jobs with their status.`,
	Run:   runCronList,
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove <job-id>",
	Short: "Remove a cron job",
	Long:  `Remove a cron job by its ID.`,
	Args:  cobra.ExactArgs(1),
	Run:   runCronRemove,
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable <job-id>",
	Short: "Enable a cron job",
	Long:  `Enable a disabled cron job.`,
	Args:  cobra.ExactArgs(1),
	Run:   runCronEnable,
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable <job-id>",
	Short: "Disable a cron job",
	Long:  `Disable an enabled cron job.`,
	Args:  cobra.ExactArgs(1),
	Run:   runCronDisable,
}

func init() {
	// Add command flags
	cronAddCmd.Flags().StringVar(&cronName, "name", "", "Job name (required)")
	cronAddCmd.Flags().StringVar(&cronPrompt, "prompt", "", "Task prompt for agent (required)")
	cronAddCmd.MarkFlagRequired("name")
	cronAddCmd.MarkFlagRequired("prompt")

	// Add subcommands
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)

	// Add to root
	rootCmd.AddCommand(cronCmd)
}

func runCronAdd(cmd *cobra.Command, args []string) {
	schedule := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cronManager *cron.Manager

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}
	defer app.Stop(context.Background())

	// Add job
	job, err := cronManager.AddJob(cronName, schedule, cronPrompt)
	if err != nil {
		fmt.Printf("Error adding cron job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job added successfully!\n\n")
	fmt.Printf("Job ID:    %s\n", job.ID)
	fmt.Printf("Name:      %s\n", job.Name)
	fmt.Printf("Schedule:  %s\n", job.Schedule)
	fmt.Printf("Next run:  %s\n", job.NextRun.Format("2006-01-02 15:04:05"))
}

func runCronList(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cronManager *cron.Manager

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}
	defer app.Stop(context.Background())

	jobs := cronManager.ListJobs()
	if len(jobs) == 0 {
		fmt.Println("No cron jobs configured.")
		fmt.Println("\nUse 'nekobot cron add' to create a new job.")
		return
	}

	fmt.Printf("\n📅 Cron Jobs (%d)\n\n", len(jobs))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSCHEDULE\tENABLED\tNEXT RUN\tLAST RUN\tRUNS\tSTATUS")
	fmt.Fprintln(w, "--\t----\t--------\t-------\t--------\t--------\t----\t------")

	for _, job := range jobs {
		enabled := "✅"
		if !job.Enabled {
			enabled = "❌"
		}

		status := "Never"
		if !job.LastRun.IsZero() {
			if job.LastSuccess {
				status = "✅ OK"
			} else {
				status = "❌ Failed"
			}
		}

		nextRun := "-"
		if job.Enabled && !job.NextRun.IsZero() {
			nextRun = job.NextRun.Format("15:04:05")
		}

		lastRun := "-"
		if !job.LastRun.IsZero() {
			lastRun = job.LastRun.Format("15:04:05")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			job.ID,
			truncateString(job.Name, 20),
			job.Schedule,
			enabled,
			nextRun,
			lastRun,
			job.RunCount,
			status,
		)
	}

	w.Flush()
	fmt.Println()
}

func runCronRemove(cmd *cobra.Command, args []string) {
	jobID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cronManager *cron.Manager

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}
	defer app.Stop(context.Background())

	if err := cronManager.RemoveJob(jobID); err != nil {
		fmt.Printf("Error removing job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job removed: %s\n", jobID)
}

func runCronEnable(cmd *cobra.Command, args []string) {
	jobID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cronManager *cron.Manager

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}
	defer app.Stop(context.Background())

	if err := cronManager.EnableJob(jobID); err != nil {
		fmt.Printf("Error enabling job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job enabled: %s\n", jobID)
}

func runCronDisable(cmd *cobra.Command, args []string) {
	jobID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cronManager *cron.Manager

	app := fx.New(
		config.Module,
		logger.Module,
		bus.Module,
		session.Module,
		providers.Module,
		tools.Module,
		commands.Module,
		workspace.Module,
		skills.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}
	defer app.Stop(context.Background())

	if err := cronManager.DisableJob(jobID); err != nil {
		fmt.Printf("Error disabling job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job disabled: %s\n", jobID)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
