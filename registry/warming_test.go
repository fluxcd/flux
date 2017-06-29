// +build integration

package registry

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"os"
	"sync"
	"testing"
	"time"
)

func TestWarmer_CacheNewRepo(t *testing.T) {
	mc := Setup(t)
	defer mc.Stop()

	dc := NewMockClient(
		func(repository Repository, tag string) (flux.Image, error) {
			return img, nil
		},
		func(repository Repository) ([]string, error) {
			return []string{"tag1"}, nil
		},
	)

	w := Warmer{
		Logger:        log.NewLogfmtLogger(os.Stderr),
		ClientFactory: &mockRemoteFactory{c: dc},
		Creds:         NoCredentials(),
		Expiry:        time.Hour,
		Client:        mc,
	}

	shutdown := make(chan struct{})
	repo := make(chan Repository)
	shutdownWg := &sync.WaitGroup{}
	shutdownWg.Add(1)
	go w.Loop(shutdown, shutdownWg, repo)

	r, _ := ParseRepository("test/repo")
	repo <- r

	shutdown <- struct{}{}
	shutdownWg.Wait()

	// Test that tags were written
	key := tagKey("", r.String())
	item, err := mc.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	err = json.Unmarshal(item.Value, &tags)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("Expected 1 history item, got %v", tags)
	}
	expectedTag := "tag1"
	if tags[0] != expectedTag {
		t.Fatalf("Expected  history item: %v, got %v", expectedTag, tags[0])
	}

	// Test that manifest was written
	key = manifestKey("", r.String(), "tag1")
	item, err = mc.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	var i flux.Image
	err = json.Unmarshal(item.Value, &i)
	if err != nil {
		t.Fatal(err)
	}

	if i.ID.String() != img.ID.String() {
		t.Fatalf("Expected %s, got %s", img.ID.String(), i.ID.String())
	}
}

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
