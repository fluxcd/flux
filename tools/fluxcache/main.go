package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/registry/cache"
)

func bail(e error) {
	fmt.Fprintln(os.Stderr, "ERROR "+e.Error())
	os.Exit(1)
}

type exact struct {
	k string
}

func (x exact) Key() string {
	return x.k
}

func main() {
	var memcachedAddr = flag.String("memcached", "localhost:11211", "address for connecting to memcached")
	var raw = flag.Bool("raw", false, "show raw memcached entry")
	var key = flag.Bool("key", false, "argument is an exact key (implies -raw)")

	flag.Parse()
	if *key {
		*raw = true
	}

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	client := cache.NewFixedServerMemcacheClient(cache.MemcacheConfig{
		Logger:  log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		Timeout: 5 * time.Second,
	}, *memcachedAddr)

	var (
		bytes  []byte
		expiry time.Time
		err    error
		entry  interface{}
		ref    image.Ref
	)

	if *key {
		k := exact{args[0]}
		bytes, expiry, err = client.GetKey(k)
		goto display
	}

	ref, err = image.ParseRef(args[0])
	if err != nil {
		bail(err)
	}

	if ref.Tag != "" {
		k := cache.NewManifestKey(ref.CanonicalRef())
		fmt.Fprintf(os.Stderr, "Key: %s\n", k.Key())
		bytes, expiry, err = client.GetKey(k)
		if !*raw && err == nil {
			var im image.Info
			if err = json.Unmarshal(bytes, &im); err != nil {
				bail(err)
			}
			entry = im
		}
	} else {
		k := cache.NewRepositoryKey(ref.CanonicalName())
		bytes, expiry, err = client.GetKey(k)
		if !*raw && err == nil {
			var repo registry.ImageRepository
			if err = json.Unmarshal(bytes, &repo); err != nil {
				bail(err)
			}
			entry = repo
		}
	}

display:
	if *raw {
		fmt.Printf("%s %s\n", expiry, string(bytes))
	} else {
		fmt.Printf("%s %#v\n", expiry, entry)
	}
	return
}
