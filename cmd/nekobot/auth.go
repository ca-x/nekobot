package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"nekobot/pkg/auth"
	"nekobot/pkg/logger"
)

var (
	authProvider   string
	authDeviceCode bool
	authPaste      bool
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials",
	Long:  `Manage authentication credentials for API providers.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to an API provider",
	Long: `Login to an API provider using OAuth or manual token entry.

Examples:
  # OAuth browser login
  nekobot auth login --provider openai

  # Device code flow (headless)
  nekobot auth login --provider openai --device-code

  # Manual token paste
  nekobot auth login --provider anthropic --paste`,
	Run: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from an API provider",
	Long: `Remove stored credentials for an API provider.

Examples:
  # Logout from specific provider
  nekobot auth logout --provider openai

  # Logout from all providers
  nekobot auth logout --all`,
	Run: runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Display currently authenticated providers and credential status.`,
	Run:   runAuthStatus,
}

var authLogoutAll bool

func init() {
	// Auth command flags
	authLoginCmd.Flags().StringVarP(&authProvider, "provider", "p", "", "API provider (openai, anthropic, google, etc.)")
	authLoginCmd.Flags().BoolVar(&authDeviceCode, "device-code", false, "Use device code flow (headless)")
	authLoginCmd.Flags().BoolVar(&authPaste, "paste", false, "Manually paste API key")
	authLoginCmd.MarkFlagRequired("provider")

	authLogoutCmd.Flags().StringVarP(&authProvider, "provider", "p", "", "API provider to logout from")
	authLogoutCmd.Flags().BoolVar(&authLogoutAll, "all", false, "Logout from all providers")

	// Add subcommands
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	// Add to root
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) {
	log, err := logger.New(logger.Config{
		Level:  "info",
		Format: "console",
	})
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		os.Exit(1)
	}

	store, err := auth.NewAuthStore()
	if err != nil {
		log.Error("Failed to initialize auth store", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("\n🔐 Authenticating with %s...\n\n", auth.ProviderDisplayName(authProvider))

	var cred *auth.AuthCredential

	if authPaste {
		// Manual token paste
		cred, err = auth.LoginPasteToken(authProvider, os.Stdin)
		if err != nil {
			log.Error("Failed to authenticate", zap.Error(err))
			os.Exit(1)
		}
	} else {
		// Check if provider supports OAuth
		if !auth.SupportsOAuth(authProvider) {
			fmt.Printf("Provider %s does not support OAuth. Use --paste to manually enter an API key.\n", authProvider)
			os.Exit(1)
		}

		oauthCfg, err := auth.GetOAuthConfig(authProvider)
		if err != nil {
			log.Error("Failed to get OAuth config", zap.Error(err))
			os.Exit(1)
		}

		if authDeviceCode {
			// Device code flow
			if !auth.SupportsDeviceCode(authProvider) {
				fmt.Printf("Provider %s does not support device code flow.\n", authProvider)
				os.Exit(1)
			}

			cred, err = auth.LoginDeviceCode(oauthCfg)
			if err != nil {
				log.Error("Failed to authenticate", zap.Error(err))
				os.Exit(1)
			}
		} else {
			// Browser OAuth flow
			cred, err = auth.LoginBrowser(oauthCfg)
			if err != nil {
				log.Error("Failed to authenticate", zap.Error(err))
				os.Exit(1)
			}
		}
	}

	// Store credential
	if err := store.Set(authProvider, cred); err != nil {
		log.Error("Failed to store credential", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("\n✅ Successfully authenticated with %s!\n", auth.ProviderDisplayName(authProvider))
	if cred.AccountID != "" {
		fmt.Printf("Account ID: %s\n", cred.AccountID)
	}
	if !cred.ExpiresAt.IsZero() {
		fmt.Printf("Token expires: %s\n", cred.ExpiresAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()
}

func runAuthLogout(cmd *cobra.Command, args []string) {
	log, err := logger.New(logger.Config{
		Level:  "info",
		Format: "console",
	})
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		os.Exit(1)
	}

	store, err := auth.NewAuthStore()
	if err != nil {
		log.Error("Failed to initialize auth store", zap.Error(err))
		os.Exit(1)
	}

	if authLogoutAll {
		if err := store.Clear(); err != nil {
			log.Error("Failed to clear credentials", zap.Error(err))
			os.Exit(1)
		}
		fmt.Println("✅ Logged out from all providers")
		return
	}

	if authProvider == "" {
		fmt.Println("Error: must specify --provider or --all")
		os.Exit(1)
	}

	if err := store.Delete(authProvider); err != nil {
		log.Error("Failed to delete credential", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("✅ Logged out from %s\n", auth.ProviderDisplayName(authProvider))
}

func runAuthStatus(cmd *cobra.Command, args []string) {
	log, err := logger.New(logger.Config{
		Level:  "info",
		Format: "console",
	})
	if err != nil {
		fmt.Printf("Error creating logger: %v\n", err)
		os.Exit(1)
	}

	store, err := auth.NewAuthStore()
	if err != nil {
		log.Error("Failed to initialize auth store", zap.Error(err))
		os.Exit(1)
	}

	creds := store.List()
	if len(creds) == 0 {
		fmt.Println("No authentication credentials stored.")
		fmt.Println("\nUse 'nekobot auth login --provider <name>' to authenticate.")
		return
	}

	fmt.Println("\n🔐 Authentication Status\n")
	fmt.Println("Provider      | Status    | Method      | Expires")
	fmt.Println("------------- | --------- | ----------- | --------------------")

	for _, cred := range creds {
		status := "✅ Active"
		if cred.IsExpired() {
			status = "❌ Expired"
		} else if cred.NeedsRefresh() {
			status = "⚠️  Expiring"
		}

		expiresStr := "-"
		if !cred.ExpiresAt.IsZero() {
			expiresStr = cred.ExpiresAt.Format("2006-01-02 15:04")
		}

		fmt.Printf("%-13s | %-9s | %-11s | %s\n",
			auth.ProviderDisplayName(cred.Provider),
			status,
			cred.AuthMethod,
			expiresStr,
		)
	}

	fmt.Println()
}
