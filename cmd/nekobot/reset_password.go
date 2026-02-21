package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"nekobot/pkg/config"
)

var (
	resetUsername string
	resetPassword string
)

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset WebUI admin password",
	Long: `Reset the admin password for the WebUI dashboard.
The JWT secret is rotated automatically, invalidating all existing sessions.

Examples:
  # Interactive password input
  nekobot reset-password

  # Non-interactive
  nekobot reset-password --password newpass

  # Change username and password
  nekobot reset-password --username myadmin --password newpass`,
	Run: runResetPassword,
}

func init() {
	resetPasswordCmd.Flags().StringVar(&resetUsername, "username", "", "new admin username (optional)")
	resetPasswordCmd.Flags().StringVar(&resetPassword, "password", "", "new admin password")
	rootCmd.AddCommand(resetPasswordCmd)
}

func runResetPassword(cmd *cobra.Command, args []string) {
	password := strings.TrimSpace(resetPassword)
	if password == "" {
		// Interactive input
		fmt.Print("Enter new password: ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(string(raw))
		if password == "" {
			fmt.Fprintln(os.Stderr, "Error: password cannot be empty")
			os.Exit(1)
		}

		fmt.Print("Confirm password: ")
		raw2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
		if string(raw) != string(raw2) {
			fmt.Fprintln(os.Stderr, "Error: passwords do not match")
			os.Exit(1)
		}
	}

	// Load config to find the database path.
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Load existing credential (if any).
	existing, err := config.LoadAdminCredentialFromConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading admin credential: %v\n", err)
		os.Exit(1)
	}

	username := strings.TrimSpace(resetUsername)
	if username == "" {
		if existing != nil && existing.Username != "" {
			username = existing.Username
		} else {
			username = "admin"
		}
	}

	hash, err := config.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
		os.Exit(1)
	}

	cred := &config.AdminCredential{
		Username:     username,
		PasswordHash: hash,
		JWTSecret:    config.GenerateJWTSecret(), // rotate to invalidate all sessions
	}

	if err := config.SaveAdminCredentialFromConfig(cfg, cred); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving admin credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Admin password reset successfully (username: %s)\n", username)
	fmt.Println("All existing sessions have been invalidated.")
}
