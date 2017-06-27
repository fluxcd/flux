package registry

import (
	"github.com/go-kit/kit/log"
	"math/rand"
	"sync"
	"time"
)

// Queue provides an updating repository queue for the warmer.
// If no items are added to the queue this will randomly add a new
// registry to warm
type Queue struct {
	RunningContainers    func() []Repository
	Logger               log.Logger
	RegistryPollInterval time.Duration
	warmQueue            chan Repository
	queueLock            sync.Mutex
}

// Queue loop to maintain the queue and periodically add a random
// repository that is running in the cluster.
func (w *Queue) Loop(stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if w.RunningContainers == nil || w.Logger == nil || w.RegistryPollInterval == 0 {
		panic("registry.Queue fields are nil")
	}

	w.queueLock.Lock()
	w.warmQueue = make(chan Repository, 100)
	w.queueLock.Unlock()

	pollImages := time.Tick(w.RegistryPollInterval)
	w.Logger.Log("tick", w.RegistryPollInterval)

	for {
		select {
		case <-stop:
			w.Logger.Log("stopping", "true")
			return
		case <-pollImages:
			c := w.RunningContainers()
			if len(c) > 0 { // Only add random registry if there are running containers
				i := rand.Intn(len(c)) // Pick random registry
				w.queueLock.Lock()
				w.warmQueue <- c[i] // Add registry to queue
				w.queueLock.Unlock()
			}
		}
	}
}

func (w *Queue) Queue() chan Repository {
	w.queueLock.Lock()
	defer w.queueLock.Unlock()
	return w.warmQueue
}
