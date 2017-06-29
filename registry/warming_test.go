// +build integration

package registry

import (
	"encoding/json"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	"os"
	"sync"
	"testing"
	"time"
)

func TestWarmer_CacheNewRepo(t *testing.T) {
	mc := Setup(t)
	defer mc.Stop()

	dc := NewMockDockerClient(
		func(repository, reference string) ([]schema1.History, error) {
			return []schema1.History{{`{"test":"json"}`}}, nil
		},
		func(repository string) ([]string, error) {
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
	var manifests []schema1.History
	err = json.Unmarshal(item.Value, &manifests)
	if err != nil {
		t.Fatal(err)
	}

	if len(manifests) != 1 {
		t.Fatalf("Expected 1 history item, got %v", manifests)
	}
	expectedManifest := schema1.History{`{"test":"json"}`}
	if manifests[0] != expectedManifest {
		t.Fatalf("Expected  history item: %v, got %v", expectedManifest, manifests[0])
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
