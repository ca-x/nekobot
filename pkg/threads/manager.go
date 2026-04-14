package threads

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"nekobot/pkg/state"
)

const storeKey = "threads.records.v1"

type Record struct {
	ID        string    `json:"id"`
	RuntimeID string    `json:"runtime_id"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Manager struct {
	kv state.KV
}

func NewManager(kv state.KV) *Manager {
	return &Manager{kv: kv}
}

func (m *Manager) Get(ctx context.Context, id string) (Record, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Record{}, false, nil
	}
	records, err := m.load(ctx)
	if err != nil {
		return Record{}, false, err
	}
	record, ok := records[id]
	return record, ok, nil
}

func (m *Manager) List(ctx context.Context) ([]Record, error) {
	records, err := m.load(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Record, 0, len(records))
	for _, record := range records {
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (m *Manager) Upsert(ctx context.Context, id, runtimeID, topic string) error {
	if m == nil || m.kv == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return m.kv.UpdateFunc(ctx, storeKey, func(current interface{}) interface{} {
		records := decodeRecords(current)
		now := time.Now()
		record, ok := records[id]
		if !ok {
			record = Record{ID: id, CreatedAt: now}
		}
		record.RuntimeID = strings.TrimSpace(runtimeID)
		record.Topic = strings.TrimSpace(topic)
		record.UpdatedAt = now
		records[id] = record
		return encodeRecords(records)
	})
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	if m == nil || m.kv == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return m.kv.UpdateFunc(ctx, storeKey, func(current interface{}) interface{} {
		records := decodeRecords(current)
		delete(records, id)
		return encodeRecords(records)
	})
}

func (m *Manager) load(ctx context.Context) (map[string]Record, error) {
	if m == nil || m.kv == nil {
		return map[string]Record{}, nil
	}
	value, ok, err := m.kv.Get(ctx, storeKey)
	if err != nil {
		return nil, fmt.Errorf("load thread records: %w", err)
	}
	if !ok {
		return map[string]Record{}, nil
	}
	return decodeRecords(value), nil
}

func decodeRecords(current interface{}) map[string]Record {
	result := map[string]Record{}
	raw, ok := current.(map[string]interface{})
	if !ok {
		return result
	}
	for id, value := range raw {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		result[strings.TrimSpace(id)] = Record{
			ID:        strings.TrimSpace(stringValue(item["id"], id)),
			RuntimeID: strings.TrimSpace(stringValue(item["runtime_id"], "")),
			Topic:     strings.TrimSpace(stringValue(item["topic"], "")),
			CreatedAt: timeValue(item["created_at"]),
			UpdatedAt: timeValue(item["updated_at"]),
		}
	}
	return result
}

func encodeRecords(records map[string]Record) map[string]interface{} {
	result := map[string]interface{}{}
	for id, record := range records {
		result[id] = map[string]interface{}{
			"id":         record.ID,
			"runtime_id": record.RuntimeID,
			"topic":      record.Topic,
			"created_at": record.CreatedAt.Format(time.RFC3339Nano),
			"updated_at": record.UpdatedAt.Format(time.RFC3339Nano),
		}
	}
	return result
}

func stringValue(value interface{}, fallback string) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func timeValue(value interface{}) time.Time {
	if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
