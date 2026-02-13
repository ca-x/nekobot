package logger_test

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Example_dualOutput demonstrates logging to both console and file simultaneously.
func Example_dualOutput() {
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = "/tmp/nanobot-dual-output.log" // File output
	cfg.MaxSize = 10
	cfg.MaxBackups = 3

	log, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// This log will appear in BOTH console AND file
	log.Info("This message appears in both console and log file",
		zap.String("target", "dual"),
	)

	log.Warn("Warning message",
		zap.String("severity", "medium"),
	)

	fmt.Println("Check /tmp/nanobot-dual-output.log for file output")
	// Output appears on console too
}

// Example_consoleOnly demonstrates console-only logging.
func Example_consoleOnly() {
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = "" // Empty path = console only

	log, _ := logger.New(cfg)
	defer log.Sync()

	log.Info("This only appears on console")
}

// Example_productionLogging demonstrates production configuration.
func Example_productionLogging() {
	cfg := &logger.Config{
		Level:            logger.LevelInfo,
		OutputPath:       "/var/log/nekobot/production.log",
		MaxSize:          100,  // 100MB
		MaxBackups:       10,   // Keep 10 old files
		MaxAge:           30,   // 30 days
		Compress:         true, // Compress rotated logs
		Development:      false,
		EnableCaller:     true,
		EnableStacktrace: true,
	}

	log, err := logger.New(cfg)
	if err != nil {
		// Fall back to stdout if file creation fails
		cfg.OutputPath = ""
		log, _ = logger.New(cfg)
	}
	defer log.Sync()

	// Logs to both console (JSON format) and file
	log.Info("Production log entry",
		zap.String("environment", "production"),
		zap.Int("workers", 10),
	)
}

// Example_differentFormats demonstrates different formats for console and file.
func Example_differentFormats() {
	cfg := logger.DefaultConfig()
	cfg.Development = true                        // Console: colored, human-readable
	cfg.OutputPath = "/tmp/nanobot-formatted.log" // File: JSON format

	log, _ := logger.New(cfg)
	defer log.Sync()

	// Console will show: INFO [timestamp] msg="User action" user=john action=login
	// File will show: {"level":"info","time":"...","msg":"User action","user":"john","action":"login"}
	log.Info("User action",
		zap.String("user", "john"),
		zap.String("action", "login"),
	)

	fmt.Println("\nConsole output is human-readable (colored)")
	fmt.Println("File output is JSON for machine parsing")
}

// Example_structuredLogging demonstrates the power of structured logging.
func Example_structuredLogging() {
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = "/tmp/nanobot-structured.log"

	log, _ := logger.New(cfg)
	defer log.Sync()

	// Structured fields make logs searchable and analyzable
	log.Info("HTTP request",
		zap.String("method", "POST"),
		zap.String("path", "/api/users"),
		zap.Int("status_code", 200),
		zap.Duration("duration_ms", 42),
		zap.String("user_agent", "Mozilla/5.0"),
	)

	// You can grep logs by field:
	// grep '"method":"POST"' /tmp/nanobot-structured.log
	// grep '"status_code":200' /tmp/nanobot-structured.log

	fmt.Println("\nStructured logs are easy to search and analyze")
}

// Example_cleanup demonstrates proper logger cleanup.
func Example_cleanup() {
	cfg := logger.DefaultConfig()
	cfg.OutputPath = "/tmp/nanobot-cleanup.log"

	log, _ := logger.New(cfg)

	// Always sync before program exits to flush buffered logs
	defer func() {
		if err := log.Sync(); err != nil {
			// On Linux, Sync may fail for stdout/stderr, which is expected
			// Only log actual file sync errors
			if cfg.OutputPath != "" {
				fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
			}
		}
	}()

	log.Info("Application exiting gracefully")
	// The defer ensures all logs are written to disk
}
