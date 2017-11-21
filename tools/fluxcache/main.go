package main

import (
	"os"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/image"
)


func bail(e error) {
	fmt.Fprintln(os.Stderr, "ERROR " + e.Error())
	os.Exit(1)
}

type exact struct {
	k string
}
func (x exact) Key() string {
	return x.k
}

func main() {
	memcachedAddr := os.Getenv("MEMCACHED")
	if memcachedAddr == "" {
		memcachedAddr = "localhost:11211"
	}
	client := cache.NewFixedServerMemcacheClient(cache.MemcacheConfig{
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		Timeout: 5 * time.Second,
	}, memcachedAddr)

	var (
		bytes []byte
		expiry time.Time
		err error
	)

	switch os.Args[1] {
	case "exact":
		fmt.Println("Exact key: "+ os.Args[2])
		k := exact{os.Args[2]}
		bytes, expiry, err = client.GetKey(k)
	case "repo":
		ref, parseErr := image.ParseRef(os.Args[2])
		if parseErr != nil {
			bail(parseErr)
		}

		tagsKey := cache.NewTagKey(ref.CanonicalName())
		fmt.Printf("Key: %s\n", tagsKey.Key())
		client.GetKey(tagsKey)
		bytes, expiry, err = client.GetKey(tagsKey)

		//// repoKey := cache.NewRepositoryKey(ref.CanonicalName())
		//// _, expiry, err = client.GetKey(repoKey)
		//// if err != nil {
		//// 	bail(err)
		//// }
		//// fmt.Printf("%s %s\n", expiry, ref.Name)
	case "tag":
		ref, parseErr := image.ParseRef(os.Args[2])
		if parseErr != nil {
			bail(parseErr)
		}
		k := cache.NewManifestKey(ref.CanonicalRef())
		fmt.Printf("Key: %s\n", k.Key())

		bytes, expiry, err = client.GetKey(k)
	}

	if err != nil {
		bail(err)
	}
	fmt.Printf("%s %s\n", expiry, string(bytes))
}
