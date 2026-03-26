package memory

import (
	"container/list"
	"sync"
)

const defaultEmbeddingCacheSize = 1024

type embeddingCache struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*list.Element
	lruList *list.List
}

type embeddingCacheEntry struct {
	key   string
	value []float32
}

func newEmbeddingCache(maxSize int) *embeddingCache {
	if maxSize <= 0 {
		maxSize = defaultEmbeddingCacheSize
	}

	return &embeddingCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

func (c *embeddingCache) Get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	c.lruList.MoveToFront(elem)
	entry := elem.Value.(*embeddingCacheEntry)
	return cloneVector(entry.value), true
}

func (c *embeddingCache) Put(key string, value []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*embeddingCacheEntry).value = cloneVector(value)
		return
	}

	elem := c.lruList.PushFront(&embeddingCacheEntry{
		key:   key,
		value: cloneVector(value),
	})
	c.items[key] = elem

	if c.lruList.Len() > c.maxSize {
		c.evictOldest()
	}
}

func (c *embeddingCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lruList.Len()
}

func (c *embeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.lruList.Init()
}

func (c *embeddingCache) evictOldest() {
	elem := c.lruList.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*embeddingCacheEntry)
	delete(c.items, entry.key)
	c.lruList.Remove(elem)
}

func cloneVector(value []float32) []float32 {
	if len(value) == 0 {
		return nil
	}

	out := make([]float32, len(value))
	copy(out, value)
	return out
}
