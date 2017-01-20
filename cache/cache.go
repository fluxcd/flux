package cache

import (
	"sync"
	"time"
)

type Cache struct {
	mtx     sync.RWMutex
	cache   map[string]*item
	pending map[string][]chan<- *item
}

type item struct {
	key  string
	load func() (interface{}, error)
	val  interface{}
	err  error
	ts   time.Time
}

func New() *Cache {
	return &Cache{
		cache:   map[string]*item{},
		pending: map[string][]chan<- *item{},
	}
}

func (c *Cache) Get(key string, load func() (interface{}, error)) (interface{}, error) {
	ch := make(chan *item)
	go c.get(key, load, ch)
	item := <-ch
	return item.val, item.err
}

func (c *Cache) get(key string, load func() (interface{}, error), ch chan *item) {
	// Get the lock and work quickly!!
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Best case: item is already cached.
	if item, ok := c.cache[key]; ok {
		ch <- item // bam
		return
	}

	// Next best: item is already being fetched.
	if _, ok := c.pending[key]; ok {
		c.pending[key] = append(c.pending[key], ch) // get in line
		return
	}

	// Worst case: we have to fetch the item.
	c.pending[key] = []chan<- *item{ch} // start the queue
	go c.load(key, load)                // load outside of the lock
}

func (c *Cache) load(key string, load func() (interface{}, error)) {
	// Do the actual load outside of the lock.
	val, err := load()
	item := &item{
		key:  key,
		load: load,
		val:  val,
		err:  err,
		ts:   time.Now(),
	}

	// Get the lock and work quickly!!
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Notify everyone waiting.
	for _, ch := range c.pending[key] {
		ch <- item
	}
	delete(c.pending, key)

	// Update the cache.
	delete(c.cache, key) // new beats old, even if it's bad
	if item.err == nil {
		c.cache[key] = item
	}
}
