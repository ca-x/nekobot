package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// RuntimeDBName is the unified SQLite filename used by all runtime modules.
	RuntimeDBName = "nekobot.db"
)

// RuntimeDBPath returns the unified SQLite runtime database path.
func RuntimeDBPath(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is nil")
	}
	if cfg.DatabaseType() != "sqlite" {
		return "", fmt.Errorf("runtime database type %q does not use a local SQLite path", cfg.DatabaseType())
	}

	dbDir := strings.TrimSpace(cfg.DatabaseDir())
	if dbDir == "" {
		return "", fmt.Errorf("database directory is empty")
	}
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return "", fmt.Errorf("create database directory: %w", err)
	}

	return filepath.Join(dbDir, RuntimeDBName), nil
}

// RuntimeDBDisplayName returns a redacted runtime database identifier for logs and status APIs.
func RuntimeDBDisplayName(cfg *Config) (string, error) {
	dbType, dsn, err := RuntimeDBOpenConfig(cfg)
	if err != nil {
		return "", err
	}
	if dbType == "sqlite" {
		if path, ok := sqliteFilePathFromDSN(dsn); ok {
			return path, nil
		}
	}
	return dbType + ":" + RedactDatabaseDSN(dsn), nil
}

// RedactDatabaseDSN hides credentials in a database DSN for logs and status APIs.
func RedactDatabaseDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return trimmed
	}
	schemeIdx := strings.Index(trimmed, "://")
	if schemeIdx < 0 {
		authorityEnd := strings.IndexAny(trimmed, "/?")
		if authorityEnd < 0 {
			authorityEnd = len(trimmed)
		}
		authority := trimmed[:authorityEnd]
		if at := strings.LastIndex(authority, "@"); at >= 0 {
			return "****@" + trimmed[at+1:]
		}
		return trimmed
	}
	authorityStart := schemeIdx + len("://")
	authorityEnd := strings.IndexAny(trimmed[authorityStart:], "/?")
	if authorityEnd < 0 {
		authorityEnd = len(trimmed)
	} else {
		authorityEnd += authorityStart
	}
	authority := trimmed[authorityStart:authorityEnd]
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return trimmed
	}
	return trimmed[:authorityStart] + "****@" + authority[at+1:] + trimmed[authorityEnd:]
}

// EnsureRuntimeDBFile ensures the local SQLite runtime database file exists and returns its path.
// Non-SQLite runtime databases are remote DSNs and do not need local file creation.
func EnsureRuntimeDBFile(cfg *Config) (string, error) {
	dbType, dsn, err := RuntimeDBOpenConfig(cfg)
	if err != nil {
		return "", err
	}
	if dbType != "sqlite" {
		return RuntimeDBDisplayName(cfg)
	}

	if dbPath, ok := sqliteFilePathFromDSN(dsn); ok {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return "", fmt.Errorf("create database directory: %w", err)
		}
		f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return "", fmt.Errorf("create runtime database file: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("close runtime database file: %w", err)
		}
		return dbPath, nil
	}

	dbPath, err := RuntimeDBPath(cfg)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return "", fmt.Errorf("create runtime database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close runtime database file: %w", err)
	}
	return dbPath, nil
}

func sqliteFilePathFromDSN(dsn string) (string, bool) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" || trimmed == ":memory:" || strings.HasPrefix(trimmed, "file::memory:") {
		return "", false
	}
	value := trimmed
	if strings.HasPrefix(value, "file:") {
		value = strings.TrimPrefix(value, "file:")
	}
	if idx := strings.IndexByte(value, '?'); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value == "" || value == ":memory:" {
		return "", false
	}
	return value, true
}

// MigrateRuntimeDB copies the unified runtime database to a new location.
func MigrateRuntimeDB(oldPath, newPath string) error {
	oldRuntimePath := strings.TrimSpace(oldPath)
	newRuntimePath := strings.TrimSpace(newPath)
	if oldRuntimePath == "" {
		return fmt.Errorf("old runtime database path is empty")
	}
	if newRuntimePath == "" {
		return fmt.Errorf("new runtime database path is empty")
	}
	if oldRuntimePath == newRuntimePath {
		return nil
	}

	oldInfo, err := os.Stat(oldRuntimePath)
	if err != nil {
		if os.IsNotExist(err) {
			f, createErr := os.OpenFile(newRuntimePath, os.O_RDWR|os.O_CREATE, 0o644)
			if createErr != nil {
				return fmt.Errorf("create new runtime database file: %w", createErr)
			}
			if closeErr := f.Close(); closeErr != nil {
				return fmt.Errorf("close new runtime database file: %w", closeErr)
			}
			return nil
		}
		return fmt.Errorf("stat old runtime database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(newRuntimePath), 0o755); err != nil {
		return fmt.Errorf("create new runtime database directory: %w", err)
	}
	if newInfo, err := os.Stat(newRuntimePath); err == nil && newInfo.Size() > 0 {
		return fmt.Errorf("target runtime database already exists: %s", newRuntimePath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat new runtime database: %w", err)
	}

	src, err := os.Open(oldRuntimePath)
	if err != nil {
		return fmt.Errorf("open old runtime database: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	tmpPath := filepath.Join(
		filepath.Dir(newRuntimePath),
		fmt.Sprintf(".tmp-runtime-db-%d-%d", os.Getpid(), time.Now().UnixNano()),
	)
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, oldInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create temp runtime database: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = dst.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy runtime database: %w", err)
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync runtime database copy: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close temp runtime database: %w", err)
	}
	if err := os.Rename(tmpPath, newRuntimePath); err != nil {
		return fmt.Errorf("move migrated runtime database: %w", err)
	}
	if dirFile, err := os.Open(filepath.Dir(newRuntimePath)); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	cleanup = false
	return nil
}
