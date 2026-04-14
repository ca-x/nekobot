package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"nekobot/pkg/externalagent"
)

var externalAgentsCmd = &cobra.Command{
	Use:   "external-agents",
	Short: "Inspect supported external coding agents",
}

var externalAgentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported external agent adapters and install hints",
	Run:   runExternalAgentsList,
}

var externalAgentsDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect locally installed external agents by config directory",
	Run:   runExternalAgentsDetect,
}

var externalAgentsInstallHintCmd = &cobra.Command{
	Use:   "install-hint [kind]",
	Short: "Show the best-effort install command for a supported external agent",
	Args:  cobra.ExactArgs(1),
	Run:   runExternalAgentsInstallHint,
}

func init() {
	externalAgentsCmd.AddCommand(externalAgentsListCmd)
	externalAgentsCmd.AddCommand(externalAgentsDetectCmd)
	externalAgentsCmd.AddCommand(externalAgentsInstallHintCmd)
	rootCmd.AddCommand(externalAgentsCmd)
}

func runExternalAgentsList(cmd *cobra.Command, args []string) {
	registry := externalagent.NewRegistry()
	items := registry.List()
	fmt.Fprintf(cmd.OutOrStdout(), "Supported external agents (%d):\n\n", len(items))
	for _, item := range items {
		installHint := "manual"
		if item.SupportsAutoInstall() {
			installHint = strings.Join(item.InstallCommand(runtime.GOOS), " ")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\n  tool: %s\n  command: %s\n  auto-install: %t\n  install-hint: %s\n\n", item.Kind(), item.Tool(), item.Command(), item.SupportsAutoInstall(), installHint)
	}
}

func runExternalAgentsDetect(cmd *cobra.Command, args []string) {
	items, err := externalagent.DetectInstalled("")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to detect installed external agents: %v\n", err)
		return
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No supported external agents detected.")
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Kind < items[j].Kind })
	fmt.Fprintf(cmd.OutOrStdout(), "Detected external agents (%d):\n\n", len(items))
	for _, item := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\n  tool: %s\n  config: %s\n\n", item.Kind, item.Tool, item.ConfigDir)
	}
}

func runExternalAgentsInstallHint(cmd *cobra.Command, args []string) {
	hint, err := externalagent.InstallHint(args[0], runtime.GOOS)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to resolve install hint: %v\n", err)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.Join(hint, " "))
}
