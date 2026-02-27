package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic atomically writes data to a file using a temp file + rename
// pattern. The target file is either completely written with the new data or
// unchanged if any step fails before the rename.
//
// Steps:
//  1. Creates a temp file in the same directory as target
//  2. Writes data to temp file
//  3. Syncs data to physical storage
//  4. Sets file permissions
//  5. Atomically renames temp → target
//  6. Syncs directory entry
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpFile, err := os.OpenFile(
		filepath.Join(dir, fmt.Sprintf(".tmp-%d-%d", os.Getpid(), time.Now().UnixNano())),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		perm,
	)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("set permissions: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		dirFile.Close()
	}

	cleanup = false
	return nil
}
