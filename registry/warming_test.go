// +build integration

package registry

import (
	"github.com/go-kit/kit/log"
	"os"
	"sync"
	"testing"
	"time"
)

func TestQueue_Usage(t *testing.T) {

	queue := NewQueue(
		func() []Repository {
			r, _ := ParseRepository("test/image")
			return []Repository{r}
		},
		log.NewLogfmtLogger(os.Stderr),
		1*time.Millisecond,
	)

	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	shutdownWg.Add(1)
	go queue.Loop(shutdown, shutdownWg)
	defer func() {
		shutdown <- struct{}{}
		shutdownWg.Wait()
	}()

	time.Sleep(10 * time.Millisecond)
	if len(queue.Queue()) == 0 {
		t.Fatal("Should have randomly added containers to queue")
	}
}

func TestQueue_NoContainers(t *testing.T) {
	queue := NewQueue(
		func() []Repository {
			return []Repository{}
		},
		log.NewLogfmtLogger(os.Stderr),
		1*time.Millisecond,
	)

	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	shutdownWg.Add(1)
	go queue.Loop(shutdown, shutdownWg)
	defer func() {
		shutdown <- struct{}{}
		shutdownWg.Wait()
	}()

	time.Sleep(10 * time.Millisecond)
	if len(queue.Queue()) != 0 {
		t.Fatal("There were no containers, so there should be no repositories in the queue")
	}
}
