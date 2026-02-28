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
	"nekobot/pkg/providerstore"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/state"
	"nekobot/pkg/tools"
	"nekobot/pkg/workspace"
)

var (
	cronName           string
	cronPrompt         string
	cronDeleteAfterRun bool
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

var cronAtCmd = &cobra.Command{
	Use:   "at <time>",
	Short: "Add a one-time job",
	Long: `Add a one-time job that runs at a specific time.

Time format:
  RFC3339 timestamp (e.g. 2026-03-01T09:30:00+08:00)

Example:
  nekobot cron at "2026-03-01T09:30:00+08:00" --name "One Shot" --prompt "Run once"`,
	Args: cobra.ExactArgs(1),
	Run:  runCronAt,
}

var cronEveryCmd = &cobra.Command{
	Use:   "every <duration>",
	Short: "Add an interval job",
	Long: `Add a recurring job that runs at a fixed interval.

Duration format examples:
  30s, 5m, 1h, 2h30m

Example:
  nekobot cron every "15m" --name "Heartbeat" --prompt "Check service health"`,
	Args: cobra.ExactArgs(1),
	Run:  runCronEvery,
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

var cronRunCmd = &cobra.Command{
	Use:   "run <job-id>",
	Short: "Run a cron job immediately",
	Long:  `Run a cron job once immediately.`,
	Args:  cobra.ExactArgs(1),
	Run:   runCronRun,
}

func init() {
	// Add command flags.
	cronAddCmd.Flags().StringVar(&cronName, "name", "", "Job name (required)")
	cronAddCmd.Flags().StringVar(&cronPrompt, "prompt", "", "Task prompt for agent (required)")
	_ = cronAddCmd.MarkFlagRequired("name")
	_ = cronAddCmd.MarkFlagRequired("prompt")

	cronAtCmd.Flags().StringVar(&cronName, "name", "", "Job name (required)")
	cronAtCmd.Flags().StringVar(&cronPrompt, "prompt", "", "Task prompt for agent (required)")
	cronAtCmd.Flags().BoolVar(
		&cronDeleteAfterRun,
		"delete-after-run",
		true,
		"Delete one-time job after successful execution",
	)
	_ = cronAtCmd.MarkFlagRequired("name")
	_ = cronAtCmd.MarkFlagRequired("prompt")

	cronEveryCmd.Flags().StringVar(&cronName, "name", "", "Job name (required)")
	cronEveryCmd.Flags().StringVar(&cronPrompt, "prompt", "", "Task prompt for agent (required)")
	_ = cronEveryCmd.MarkFlagRequired("name")
	_ = cronEveryCmd.MarkFlagRequired("prompt")

	// Add subcommands.
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronAtCmd)
	cronCmd.AddCommand(cronEveryCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronRunCmd)

	// Add to root.
	rootCmd.AddCommand(cronCmd)
}

func runCronAdd(cmd *cobra.Command, args []string) {
	schedule := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()
	job, err := manager.AddCronJob(cronName, schedule, cronPrompt)
	if err != nil {
		fmt.Printf("Error adding cron job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job added successfully!\n\n")
	fmt.Printf("Job ID:    %s\n", job.ID)
	fmt.Printf("Name:      %s\n", job.Name)
	fmt.Printf("Kind:      %s\n", job.ScheduleKind)
	fmt.Printf("Schedule:  %s\n", job.Schedule)
	if !job.NextRun.IsZero() {
		fmt.Printf("Next run:  %s\n", job.NextRun.Format("2006-01-02 15:04:05"))
	}
}

func runCronAt(cmd *cobra.Command, args []string) {
	at, err := time.Parse(time.RFC3339, args[0])
	if err != nil {
		fmt.Printf("Error parsing time %q: %v\n", args[0], err)
		os.Exit(1)
	}

	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()
	job, err := manager.AddAtJob(cronName, at, cronPrompt, cronDeleteAfterRun)
	if err != nil {
		fmt.Printf("Error adding one-time job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ One-time job added successfully!\n\n")
	fmt.Printf("Job ID:            %s\n", job.ID)
	fmt.Printf("Name:              %s\n", job.Name)
	fmt.Printf("Kind:              %s\n", job.ScheduleKind)
	if job.AtTime != nil {
		fmt.Printf("Run at:            %s\n", job.AtTime.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Delete after run:  %t\n", job.DeleteAfterRun)
}

func runCronEvery(cmd *cobra.Command, args []string) {
	every := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()
	job, err := manager.AddEveryJob(cronName, every, cronPrompt)
	if err != nil {
		fmt.Printf("Error adding interval job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Interval job added successfully!\n\n")
	fmt.Printf("Job ID:    %s\n", job.ID)
	fmt.Printf("Name:      %s\n", job.Name)
	fmt.Printf("Kind:      %s\n", job.ScheduleKind)
	fmt.Printf("Every:     %s\n", job.EveryDuration)
	if !job.NextRun.IsZero() {
		fmt.Printf("Next run:  %s\n", job.NextRun.Format("2006-01-02 15:04:05"))
	}
}

func runCronList(cmd *cobra.Command, args []string) {
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()
	jobs := manager.ListJobs()
	if len(jobs) == 0 {
		fmt.Println("No cron jobs configured.")
		fmt.Println("\nUse 'nekobot cron add', 'nekobot cron at', or 'nekobot cron every' to create jobs.")
		return
	}

	fmt.Printf("\nCron Jobs (%d)\n\n", len(jobs))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tKIND\tSCHEDULE\tENABLED\tNEXT RUN\tLAST RUN\tRUNS\tSTATUS")
	fmt.Fprintln(w, "--\t----\t----\t--------\t-------\t--------\t--------\t----\t------")

	for _, job := range jobs {
		enabled := "yes"
		if !job.Enabled {
			enabled = "no"
		}

		status := "never"
		if !job.LastRun.IsZero() {
			if job.LastSuccess {
				status = "ok"
			} else {
				status = "failed"
			}
		}

		nextRun := "-"
		if job.Enabled && !job.NextRun.IsZero() {
			nextRun = job.NextRun.Format("2006-01-02 15:04:05")
		}

		lastRun := "-"
		if !job.LastRun.IsZero() {
			lastRun = job.LastRun.Format("2006-01-02 15:04:05")
		}

		schedule := scheduleSummary(job)
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			job.ID,
			truncateString(job.Name, 20),
			job.ScheduleKind,
			schedule,
			enabled,
			nextRun,
			lastRun,
			job.RunCount,
			status,
		)
	}

	_ = w.Flush()
	fmt.Println()
}

func runCronRemove(cmd *cobra.Command, args []string) {
	jobID := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()

	if err := manager.RemoveJob(jobID); err != nil {
		fmt.Printf("Error removing job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job removed: %s\n", jobID)
}

func runCronEnable(cmd *cobra.Command, args []string) {
	jobID := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()

	if err := manager.EnableJob(jobID); err != nil {
		fmt.Printf("Error enabling job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job enabled: %s\n", jobID)
}

func runCronDisable(cmd *cobra.Command, args []string) {
	jobID := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()

	if err := manager.DisableJob(jobID); err != nil {
		fmt.Printf("Error disabling job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job disabled: %s\n", jobID)
}

func runCronRun(cmd *cobra.Command, args []string) {
	jobID := args[0]
	manager, cleanup := buildCronManagerOrExit()
	defer cleanup()

	if err := manager.RunJob(jobID); err != nil {
		fmt.Printf("Error running job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Cron job started: %s\n", jobID)
}

func buildCronManagerOrExit() (*cron.Manager, func()) {
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
		state.Module,
		providerstore.Module,
		agent.Module,
		cron.Module,

		fx.Populate(&cronManager),
		fx.NopLogger,
	)

	if err := app.Start(ctx); err != nil {
		fmt.Printf("Error starting app: %v\n", err)
		os.Exit(1)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	cleanup := func() {
		defer stopCancel()
		if err := app.Stop(stopCtx); err != nil {
			fmt.Printf("Error stopping app: %v\n", err)
		}
	}

	return cronManager, cleanup
}

func scheduleSummary(job *cron.Job) string {
	switch job.ScheduleKind {
	case cron.ScheduleAt:
		if job.AtTime != nil {
			return "at " + job.AtTime.Format(time.RFC3339)
		}
		return "at"
	case cron.ScheduleEvery:
		return "every " + job.EveryDuration
	default:
		return job.Schedule
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
