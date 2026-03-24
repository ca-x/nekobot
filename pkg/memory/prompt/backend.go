package prompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nekobot/pkg/fileutil"
	"nekobot/pkg/state"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/configsection"
)

const (
	defaultDBPrefix = "memory:"
	defaultKVPrefix = "memory:"
	longTermSuffix  = "long_term"
)

type fileBackend struct {
	baseDir string
}

// NewFileBackend creates a file-backed prompt-memory backend.
func NewFileBackend(baseDir string) (Backend, error) {
	trimmedBaseDir := strings.TrimSpace(baseDir)
	if trimmedBaseDir == "" {
		return nil, fmt.Errorf("prompt memory file backend base dir is required")
	}
	if err := os.MkdirAll(trimmedBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure prompt memory directory %s: %w", trimmedBaseDir, err)
	}
	return &fileBackend{baseDir: trimmedBaseDir}, nil
}

func (b *fileBackend) ReadLongTerm(ctx context.Context) (string, error) {
	_ = ctx
	memoryFile := filepath.Join(b.baseDir, "MEMORY.md")
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read long-term memory %s: %w", memoryFile, err)
	}
	return string(data), nil
}

func (b *fileBackend) WriteLongTerm(ctx context.Context, content string) error {
	_ = ctx
	memoryFile := filepath.Join(b.baseDir, "MEMORY.md")
	if err := fileutil.WriteFileAtomic(memoryFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write long-term memory %s: %w", memoryFile, err)
	}
	return nil
}

func (b *fileBackend) ReadDaily(ctx context.Context, day time.Time) (string, error) {
	_ = ctx
	dailyFile := b.dailyFile(day)
	data, err := os.ReadFile(dailyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read daily memory %s: %w", dailyFile, err)
	}
	return string(data), nil
}

func (b *fileBackend) WriteDaily(ctx context.Context, day time.Time, content string) error {
	_ = ctx
	dailyFile := b.dailyFile(day)
	if err := os.MkdirAll(filepath.Dir(dailyFile), 0o755); err != nil {
		return fmt.Errorf("ensure daily memory directory %s: %w", filepath.Dir(dailyFile), err)
	}
	if err := fileutil.WriteFileAtomic(dailyFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write daily memory %s: %w", dailyFile, err)
	}
	return nil
}

func (b *fileBackend) dailyFile(day time.Time) string {
	dayID := day.Format("20060102")
	monthID := dayID[:6]
	return filepath.Join(b.baseDir, monthID, dayID+".md")
}

type dbBackend struct {
	client     *ent.Client
	longTermID string
	dailyPref  string
}

// NewDBBackend creates a database-backed prompt-memory backend.
func NewDBBackend(client *ent.Client, prefix string) (Backend, error) {
	if client == nil {
		return nil, fmt.Errorf("prompt memory db backend ent client is nil")
	}
	resolvedPrefix := normalizePrefix(prefix, defaultDBPrefix)
	return &dbBackend{
		client:     client,
		longTermID: resolvedPrefix + longTermSuffix,
		dailyPref:  resolvedPrefix + "daily:",
	}, nil
}

func (b *dbBackend) ReadLongTerm(ctx context.Context) (string, error) {
	payload, exists, err := loadSectionPayload(ctx, b.client, b.longTermID)
	if err != nil {
		return "", fmt.Errorf("read long-term memory from db: %w", err)
	}
	if !exists {
		return "", nil
	}
	return string(payload), nil
}

func (b *dbBackend) WriteLongTerm(ctx context.Context, content string) error {
	if err := upsertSectionPayload(ctx, b.client, b.longTermID, []byte(content)); err != nil {
		return fmt.Errorf("write long-term memory to db: %w", err)
	}
	return nil
}

func (b *dbBackend) ReadDaily(ctx context.Context, day time.Time) (string, error) {
	sectionName := b.dailySection(day)
	payload, exists, err := loadSectionPayload(ctx, b.client, sectionName)
	if err != nil {
		return "", fmt.Errorf("read daily memory %s from db: %w", sectionName, err)
	}
	if !exists {
		return "", nil
	}
	return string(payload), nil
}

func (b *dbBackend) WriteDaily(ctx context.Context, day time.Time, content string) error {
	sectionName := b.dailySection(day)
	if err := upsertSectionPayload(ctx, b.client, sectionName, []byte(content)); err != nil {
		return fmt.Errorf("write daily memory %s to db: %w", sectionName, err)
	}
	return nil
}

func (b *dbBackend) dailySection(day time.Time) string {
	return b.dailyPref + day.Format("20060102")
}

type noopBackend struct{}

// NewNoopBackend creates a no-op prompt-memory backend.
func NewNoopBackend() Backend {
	return &noopBackend{}
}

func (b *noopBackend) ReadLongTerm(ctx context.Context) (string, error) {
	_ = ctx
	return "", nil
}

func (b *noopBackend) WriteLongTerm(ctx context.Context, content string) error {
	_ = ctx
	_ = content
	return nil
}

func (b *noopBackend) ReadDaily(ctx context.Context, day time.Time) (string, error) {
	_ = ctx
	_ = day
	return "", nil
}

func (b *noopBackend) WriteDaily(ctx context.Context, day time.Time, content string) error {
	_ = ctx
	_ = day
	_ = content
	return nil
}

type kvBackend struct {
	store      state.KV
	longTermID string
	dailyPref  string
}

// NewKVBackend creates a KV-backed prompt-memory backend.
func NewKVBackend(store state.KV, prefix string) (Backend, error) {
	if store == nil {
		return nil, fmt.Errorf("prompt memory kv backend store is nil")
	}
	resolvedPrefix := normalizePrefix(prefix, defaultKVPrefix)
	return &kvBackend{
		store:      store,
		longTermID: resolvedPrefix + longTermSuffix,
		dailyPref:  resolvedPrefix + "daily:",
	}, nil
}

func (b *kvBackend) ReadLongTerm(ctx context.Context) (string, error) {
	content, exists, err := b.store.GetString(ctx, b.longTermID)
	if err != nil {
		return "", fmt.Errorf("read long-term memory from kv: %w", err)
	}
	if !exists {
		return "", nil
	}
	return content, nil
}

func (b *kvBackend) WriteLongTerm(ctx context.Context, content string) error {
	if err := b.store.Set(ctx, b.longTermID, content); err != nil {
		return fmt.Errorf("write long-term memory to kv: %w", err)
	}
	return nil
}

func (b *kvBackend) ReadDaily(ctx context.Context, day time.Time) (string, error) {
	key := b.dailyKey(day)
	content, exists, err := b.store.GetString(ctx, key)
	if err != nil {
		return "", fmt.Errorf("read daily memory %s from kv: %w", key, err)
	}
	if !exists {
		return "", nil
	}
	return content, nil
}

func (b *kvBackend) WriteDaily(ctx context.Context, day time.Time, content string) error {
	key := b.dailyKey(day)
	if err := b.store.Set(ctx, key, content); err != nil {
		return fmt.Errorf("write daily memory %s to kv: %w", key, err)
	}
	return nil
}

func (b *kvBackend) dailyKey(day time.Time) string {
	return b.dailyPref + day.Format("20060102")
}

func normalizePrefix(prefix, fallback string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		trimmedPrefix = fallback
	}
	if strings.HasSuffix(trimmedPrefix, ":") {
		return trimmedPrefix
	}
	return trimmedPrefix + ":"
}

func loadSectionPayload(ctx context.Context, client *ent.Client, section string) ([]byte, bool, error) {
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(section)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load memory section %s: %w", section, err)
	}
	return []byte(rec.PayloadJSON), true, nil
}

func upsertSectionPayload(ctx context.Context, client *ent.Client, section string, payload []byte) error {
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(section)).Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return fmt.Errorf("load memory section %s: %w", section, err)
		}
		_, err = client.ConfigSection.Create().
			SetSection(section).
			SetPayloadJSON(string(payload)).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				affectedRows, updateErr := client.ConfigSection.Update().
					Where(configsection.SectionEQ(section)).
					SetPayloadJSON(string(payload)).
					Save(ctx)
				if updateErr == nil && affectedRows > 0 {
					return nil
				}
			}
			return fmt.Errorf("save memory section %s: %w", section, err)
		}
		return nil
	}

	_, err = client.ConfigSection.UpdateOneID(rec.ID).SetPayloadJSON(string(payload)).Save(ctx)
	if err != nil {
		return fmt.Errorf("save memory section %s: %w", section, err)
	}
	return nil
}
