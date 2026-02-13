package config

import (
	"nekobot/pkg/logger"
)

// ToLoggerConfig converts LoggerConfig to logger.Config.
func (lc *LoggerConfig) ToLoggerConfig() *logger.Config {
	level := logger.LevelInfo
	switch lc.Level {
	case "debug":
		level = logger.LevelDebug
	case "info":
		level = logger.LevelInfo
	case "warn":
		level = logger.LevelWarn
	case "error":
		level = logger.LevelError
	case "fatal":
		level = logger.LevelFatal
	}

	return &logger.Config{
		Level:      level,
		OutputPath: lc.OutputPath,
		MaxSize:    lc.MaxSize,
		MaxBackups: lc.MaxBackups,
		MaxAge:     lc.MaxAge,
		Compress:   lc.Compress,
	}
}
