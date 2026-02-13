// Package logger provides structured logging with rotation support.
// It uses zap for high-performance structured logging and lumberjack for log rotation.
package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Level represents the log level.
type Level string

const (
	// LevelDebug for debug messages.
	LevelDebug Level = "debug"
	// LevelInfo for informational messages.
	LevelInfo Level = "info"
	// LevelWarn for warning messages.
	LevelWarn Level = "warn"
	// LevelError for error messages.
	LevelError Level = "error"
	// LevelFatal for fatal messages (will call os.Exit(1)).
	LevelFatal Level = "fatal"
)

// Config represents logger configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error, fatal).
	Level Level

	// OutputPath is the log file path. Empty means stdout only.
	OutputPath string

	// MaxSize is the maximum size in megabytes before rotation (default: 100).
	MaxSize int

	// MaxBackups is the maximum number of old log files to retain (default: 3).
	MaxBackups int

	// MaxAge is the maximum number of days to retain old log files (default: 7).
	MaxAge int

	// Compress determines if rotated log files should be compressed (default: true).
	Compress bool

	// Development enables development mode (more verbose, human-readable).
	Development bool

	// EnableCaller adds caller information (file:line) to logs.
	EnableCaller bool

	// EnableStacktrace adds stacktrace for Error and above.
	EnableStacktrace bool
}

// DefaultConfig returns a default logger configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	logPath := filepath.Join(homeDir, ".nanobot", "logs", "nekobot.log")

	return &Config{
		Level:            LevelInfo,
		OutputPath:       logPath,
		MaxSize:          100,
		MaxBackups:       3,
		MaxAge:           7,
		Compress:         true,
		Development:      false,
		EnableCaller:     true,
		EnableStacktrace: true,
	}
}

// Logger wraps zap.Logger with additional functionality.
type Logger struct {
	*zap.Logger
	config *Config
	sugar  *zap.SugaredLogger
}

// New creates a new logger with the given configuration.
func New(cfg *Config) (*Logger, error) {
	// Convert level
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Use colored output in development mode for console
	if cfg.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Development {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create cores
	var cores []zapcore.Core

	// Console output (always enabled)
	consoleEncoder := encoder
	if cfg.Development {
		consoleEncoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	cores = append(cores, zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	))

	// File output (if path specified)
	if cfg.OutputPath != "" {
		// Ensure log directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0755); err != nil {
			return nil, fmt.Errorf("creating log directory: %w", err)
		}

		// Create lumberjack logger for rotation
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.OutputPath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}

		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			level,
		))
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Create logger options
	options := []zap.Option{
		zap.AddCaller(),
	}

	if cfg.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	if cfg.Development {
		options = append(options, zap.Development())
	}

	// Create logger
	zapLogger := zap.New(core, options...)

	return &Logger{
		Logger: zapLogger,
		config: cfg,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// Sugar returns a sugared logger for easier use.
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// WithFields creates a new logger with the given fields.
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		config: l.config,
		sugar:  l.Logger.With(fields...).Sugar(),
	}
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// parseLevel converts string level to zapcore.Level.
func parseLevel(level Level) (zapcore.Level, error) {
	switch level {
	case LevelDebug:
		return zapcore.DebugLevel, nil
	case LevelInfo:
		return zapcore.InfoLevel, nil
	case LevelWarn:
		return zapcore.WarnLevel, nil
	case LevelError:
		return zapcore.ErrorLevel, nil
	case LevelFatal:
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// Global logger instance.
var global *Logger

// InitGlobal initializes the global logger.
func InitGlobal(cfg *Config) error {
	logger, err := New(cfg)
	if err != nil {
		return err
	}
	global = logger
	return nil
}

// Global returns the global logger instance.
func Global() *Logger {
	if global == nil {
		// Create default logger if not initialized
		cfg := DefaultConfig()
		cfg.Development = true
		global, _ = New(cfg)
	}
	return global
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	Global().Debug(msg, fields...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	Global().Info(msg, fields...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	Global().Warn(msg, fields...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	Global().Error(msg, fields...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	Global().Fatal(msg, fields...)
}

// Sync syncs the global logger.
func Sync() error {
	return Global().Sync()
}
