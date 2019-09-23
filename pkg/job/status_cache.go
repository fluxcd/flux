package job

import (
	"sync"
)

type StatusCache struct {
	// Size is the number of statuses to store. When full, jobs are evicted in FIFO ordering.
	// oldest ones will be evicted to make room.
	Size int

	// Store cache entries in an array to make fifo eviction easier. Efficiency
	// doesn't matter because the cache is small and computers are fast.
	cache []cacheEntry
	sync.RWMutex
}

type cacheEntry struct {
	ID     ID
	Status Status
}

func (c *StatusCache) SetStatus(id ID, status Status) {
	if c.Size <= 0 {
		return
	}
	c.Lock()
	defer c.Unlock()
	if i := c.statusIndex(id); i >= 0 {
		// already exists, update
		c.cache[i].Status = status
	}
	// Evict, if we need to. Eviction is done first, so that append can only copy
	// the things we care about keeping. Micro-optimize to the max.
	if c.Size <= len(c.cache) {
		c.cache = c.cache[len(c.cache)-(c.Size-1):]
	}
	c.cache = append(c.cache, cacheEntry{
		ID:     id,
		Status: status,
	})
}

func (c *StatusCache) Status(id ID) (Status, bool) {
	c.RLock()
	defer c.RUnlock()
	i := c.statusIndex(id)
	if i < 0 {
		return Status{}, false
	}
	return c.cache[i].Status, true
}

func (c *StatusCache) statusIndex(id ID) int {
	// entries are sorted by arrival time, not id, so we can't use binary search.
	for i := range c.cache {
		if c.cache[i].ID == id {
			return i
		}
	}
	return -1
}
