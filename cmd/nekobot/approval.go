package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"nekobot/pkg/config"
)

var (
	approvalDenyReason string
)

var approvalCmd = &cobra.Command{
	Use:   "approval",
	Short: "Manage tool approval requests",
	Long: `View and manage pending tool execution approval requests.

The approval system queues tool calls for review when running in 'manual' mode.
Use these commands to list, approve, or deny pending requests.

Examples:
  nekobot approval list
  nekobot approval approve approval-1
  nekobot approval deny approval-2 --reason "unsafe operation"`,
}

var approvalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending approval requests",
	Run:   runApprovalList,
}

var approvalApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a pending request",
	Args:  cobra.ExactArgs(1),
	Run:   runApprovalApprove,
}

var approvalDenyCmd = &cobra.Command{
	Use:   "deny <id>",
	Short: "Deny a pending request",
	Args:  cobra.ExactArgs(1),
	Run:   runApprovalDeny,
}

func init() {
	approvalDenyCmd.Flags().StringVar(&approvalDenyReason, "reason", "", "Reason for denial")

	approvalCmd.AddCommand(approvalListCmd)
	approvalCmd.AddCommand(approvalApproveCmd)
	approvalCmd.AddCommand(approvalDenyCmd)

	rootCmd.AddCommand(approvalCmd)
}

func getWebUIBase() string {
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		// Fallback defaults
		return "http://localhost:8081"
	}
	port := cfg.WebUI.Port
	if port == 0 {
		port = cfg.Gateway.Port + 1
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

func runApprovalList(cmd *cobra.Command, args []string) {
	base := getWebUIBase()
	resp, err := http.Get(base + "/api/approvals")
	if err != nil {
		fmt.Printf("Error connecting to gateway: %v\n", err)
		fmt.Println("Make sure the gateway is running with WebUI enabled.")
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Println("Authentication required. The approval API requires a valid JWT token.")
		fmt.Println("This will be improved in a future release with API key support.")
		os.Exit(1)
	}

	var requests []struct {
		ID        string                 `json:"id"`
		ToolName  string                 `json:"tool_name"`
		Arguments map[string]interface{} `json:"arguments"`
		SessionID string                 `json:"session_id"`
		Decision  string                 `json:"decision"`
	}

	if err := json.Unmarshal(body, &requests); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if len(requests) == 0 {
		fmt.Println("No pending approval requests.")
		return
	}

	fmt.Printf("\nPending Approvals (%d)\n\n", len(requests))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTOOL\tSESSION\tSTATUS")
	fmt.Fprintln(w, "--\t----\t-------\t------")

	for _, req := range requests {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			req.ID,
			req.ToolName,
			truncateStr(req.SessionID, 20),
			req.Decision,
		)
	}
	w.Flush()
	fmt.Println()
}

func runApprovalApprove(cmd *cobra.Command, args []string) {
	id := args[0]
	base := getWebUIBase()

	resp, err := http.Post(base+"/api/approvals/"+id+"/approve", "application/json", nil)
	if err != nil {
		fmt.Printf("Error connecting to gateway: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Request not found: %s\n", id)
		os.Exit(1)
	}

	fmt.Printf("Approved: %s\n", id)
}

func runApprovalDeny(cmd *cobra.Command, args []string) {
	id := args[0]
	base := getWebUIBase()

	body := "{}"
	if approvalDenyReason != "" {
		body = fmt.Sprintf(`{"reason":%q}`, approvalDenyReason)
	}

	resp, err := http.Post(base+"/api/approvals/"+id+"/deny", "application/json", strings.NewReader(body))
	if err != nil {
		fmt.Printf("Error connecting to gateway: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Request not found: %s\n", id)
		os.Exit(1)
	}

	msg := fmt.Sprintf("Denied: %s", id)
	if approvalDenyReason != "" {
		msg += fmt.Sprintf(" (reason: %s)", approvalDenyReason)
	}
	fmt.Println(msg)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
