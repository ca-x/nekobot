package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"nekobot/pkg/config"
	"nekobot/pkg/licensing"
	"nekobot/pkg/storage/ent"
)

var (
	licenseInstallID      string
	licenseSubject        string
	licenseID             string
	licenseMaxUsers       int
	licenseExpiresAt      string
	licensePrivateKeyFile string
	licensePublicKeyFile  string
	licenseOutputFile     string
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Manage server license files",
}

var licenseInstallIDCmd = &cobra.Command{
	Use:   "install-id",
	Short: "Print this server install id",
	Run: func(cmd *cobra.Command, args []string) {
		client, closeClient := openLicenseEntClient()
		defer closeClient()
		installID, err := licensing.EnsureInstallID(context.Background(), client)
		if err != nil {
			exitLicenseError("load install id", err)
		}
		fmt.Println(installID)
	},
}

var licenseStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print current license status",
	Run: func(cmd *cobra.Command, args []string) {
		client, closeClient := openLicenseEntClient()
		defer closeClient()
		status, err := licensing.StatusForClient(context.Background(), client)
		if err != nil {
			exitLicenseError("load license status", err)
		}
		fmt.Printf("install_id: %s\n", status.InstallID)
		fmt.Printf("state: %s\n", status.State)
		fmt.Printf("licensed: %t\n", status.Licensed)
		fmt.Printf("max_users: %d\n", status.MaxUsers)
		fmt.Printf("enabled_users: %d\n", status.EnabledUserCount)
		if status.LicenseID != "" {
			fmt.Printf("license_id: %s\n", status.LicenseID)
		}
		if status.ExpiresAt != "" {
			fmt.Printf("expires_at: %s\n", status.ExpiresAt)
		}
		if status.Error != "" {
			fmt.Printf("error: %s\n", status.Error)
		}
	},
}

var licenseKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate an Ed25519 license signing keypair",
	Run: func(cmd *cobra.Command, args []string) {
		pub, priv, err := licensing.GenerateKeyPair()
		if err != nil {
			exitLicenseError("generate keypair", err)
		}
		privatePEM, err := licensing.EncodePrivateKeyPEM(priv)
		if err != nil {
			exitLicenseError("encode private key", err)
		}
		publicPEM, err := licensing.EncodePublicKeyPEM(pub)
		if err != nil {
			exitLicenseError("encode public key", err)
		}
		if strings.TrimSpace(licensePrivateKeyFile) != "" {
			if err := os.WriteFile(licensePrivateKeyFile, privatePEM, 0o600); err != nil {
				exitLicenseError("write private key", err)
			}
		} else {
			fmt.Print(string(privatePEM))
		}
		if strings.TrimSpace(licensePublicKeyFile) != "" {
			if err := os.WriteFile(licensePublicKeyFile, publicPEM, 0o644); err != nil {
				exitLicenseError("write public key", err)
			}
		} else {
			fmt.Print(string(publicPEM))
		}
		fmt.Fprintf(os.Stderr, "public_key_base64=%s\n", base64.StdEncoding.EncodeToString(pub))
	},
}

var licenseGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a signed license file",
	Run: func(cmd *cobra.Command, args []string) {
		privateKeyPath := strings.TrimSpace(licensePrivateKeyFile)
		if privateKeyPath == "" {
			exitLicenseError("generate license", fmt.Errorf("--private-key-file is required"))
		}
		rawPrivate, err := os.ReadFile(privateKeyPath)
		if err != nil {
			exitLicenseError("read private key", err)
		}
		privateKey, err := licensing.ParsePrivateKey(string(rawPrivate))
		if err != nil {
			exitLicenseError("parse private key", err)
		}
		var expiresAt *time.Time
		if strings.TrimSpace(licenseExpiresAt) != "" {
			parsed, err := parseLicenseTime(licenseExpiresAt)
			if err != nil {
				exitLicenseError("parse expires-at", err)
			}
			expiresAt = &parsed
		}
		file, err := licensing.GenerateLicense(licensing.GenerateOptions{
			LicenseID: licenseID,
			Subject:   licenseSubject,
			InstallID: licenseInstallID,
			MaxUsers:  licenseMaxUsers,
			ExpiresAt: expiresAt,
			Features:  []string{"multi_user"},
		}, privateKey)
		if err != nil {
			exitLicenseError("generate license", err)
		}
		raw, err := licensing.MarshalLicense(file)
		if err != nil {
			exitLicenseError("marshal license", err)
		}
		if strings.TrimSpace(licenseOutputFile) == "" || licenseOutputFile == "-" {
			fmt.Println(raw)
			return
		}
		if err := os.WriteFile(licenseOutputFile, []byte(raw+"\n"), 0o600); err != nil {
			exitLicenseError("write license", err)
		}
		fmt.Printf("License written to %s\n", licenseOutputFile)
	},
}

func init() {
	licenseKeygenCmd.Flags().StringVar(&licensePrivateKeyFile, "private-key-file", "", "path for generated private key PEM")
	licenseKeygenCmd.Flags().StringVar(&licensePublicKeyFile, "public-key-file", "", "path for generated public key PEM")

	licenseGenerateCmd.Flags().StringVar(&licenseInstallID, "install-id", "", "server install id to bind")
	licenseGenerateCmd.Flags().StringVar(&licenseSubject, "subject", "", "license subject/customer label")
	licenseGenerateCmd.Flags().StringVar(&licenseID, "license-id", "", "license id (default: generated UUID)")
	licenseGenerateCmd.Flags().IntVar(&licenseMaxUsers, "max-users", licensing.FreeUserLimit+1, "maximum enabled users")
	licenseGenerateCmd.Flags().StringVar(&licenseExpiresAt, "expires-at", "", "expiration date/time, e.g. 2027-05-01 or RFC3339")
	licenseGenerateCmd.Flags().StringVar(&licensePrivateKeyFile, "private-key-file", "", "private key PEM file")
	licenseGenerateCmd.Flags().StringVarP(&licenseOutputFile, "output", "o", "-", "output license file, or - for stdout")
	_ = licenseGenerateCmd.MarkFlagRequired("install-id")
	_ = licenseGenerateCmd.MarkFlagRequired("private-key-file")

	licenseCmd.AddCommand(licenseInstallIDCmd)
	licenseCmd.AddCommand(licenseStatusCmd)
	licenseCmd.AddCommand(licenseKeygenCmd)
	licenseCmd.AddCommand(licenseGenerateCmd)
	rootCmd.AddCommand(licenseCmd)
}

func openLicenseEntClient() (*ent.Client, func()) {
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		exitLicenseError("load config", err)
	}
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		exitLicenseError("open runtime database", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		exitLicenseError("ensure runtime schema", err)
	}
	return client, func() {
		if err := client.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: close runtime database: %v\n", err)
		}
	}
}

func parseLicenseTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		return t, nil
	}
	if unix, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func exitLicenseError(action string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", action, err)
	os.Exit(1)
}
