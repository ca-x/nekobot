package logger_test

import (
	"fmt"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Example_basicUsage demonstrates basic logger usage.
func Example_basicUsage() {
	// Create a simple development logger
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = "" // stdout only

	log, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Use structured logging
	log.Info("Server starting",
		zap.String("host", "localhost"),
		zap.Int("port", 8080),
	)

	log.Debug("Debug message", zap.String("detail", "some debug info"))

	log.Warn("Warning message", zap.String("reason", "something unusual"))
}

// Example_withFields demonstrates logger with fields.
func Example_withFields() {
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = ""

	log, _ := logger.New(cfg)
	defer log.Sync()

	// Create logger with default fields
	requestLogger := log.WithFields(
		zap.String("request_id", "req-123"),
		zap.String("user_id", "user-456"),
	)

	// All logs from this logger will include the fields
	requestLogger.Info("Processing request")
	requestLogger.Info("Request completed")
}

// Example_sugarLogger demonstrates sugared logger for easier use.
func Example_sugarLogger() {
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = ""

	log, _ := logger.New(cfg)
	defer log.Sync()

	// Get sugared logger
	sugar := log.Sugar()

	// Printf-style logging
	sugar.Infof("User %s logged in from %s", "john", "192.168.1.1")

	// Key-value pairs
	sugar.Infow("Request processed",
		"method", "GET",
		"path", "/api/users",
		"duration_ms", 42,
	)
}

// Example_globalLogger demonstrates using the global logger.
func Example_globalLogger() {
	// Initialize global logger
	cfg := logger.DefaultConfig()
	cfg.Development = true
	cfg.OutputPath = ""

	if err := logger.InitGlobal(cfg); err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Use global logger functions
	logger.Info("Application started")
	logger.Debug("Debug info")
	logger.Warn("Warning message")
}

// Example_fileRotation demonstrates log file rotation configuration.
func Example_fileRotation() {
	cfg := logger.DefaultConfig()
	cfg.OutputPath = "/tmp/nanobot.log"
	cfg.MaxSize = 10      // 10 MB per file
	cfg.MaxBackups = 5    // Keep 5 old files
	cfg.MaxAge = 30       // Keep logs for 30 days
	cfg.Compress = true   // Compress rotated files
	cfg.Development = false

	log, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	fmt.Println("Logging to file with rotation enabled")
	log.Info("This will be written to file")
}
