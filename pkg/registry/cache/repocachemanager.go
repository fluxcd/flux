package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
)

type imageToUpdate struct {
	ref             image.Ref
	previousDigest  string
	previousRefresh time.Duration
}

// repoCacheManager handles cache operations for a container image repository
type repoCacheManager struct {
	now           time.Time
	repoID        image.Name
	client        registry.Client
	clientTimeout time.Duration
	burst         int
	trace         bool
	logger        log.Logger
	cacheClient   Client
	sync.Mutex
}

func newRepoCacheManager(now time.Time,
	repoID image.Name, clientFactory registry.ClientFactory, creds registry.Credentials, repoClientTimeout time.Duration,
	burst int, trace bool, logger log.Logger, cacheClient Client) (*repoCacheManager, error) {
	client, err := clientFactory.ClientFor(repoID.CanonicalName(), creds)
	if err != nil {
		return nil, err
	}
	manager := &repoCacheManager{
		now:           now,
		repoID:        repoID,
		client:        client,
		clientTimeout: repoClientTimeout,
		burst:         burst,
		trace:         trace,
		logger:        logger,
		cacheClient:   cacheClient,
	}
	return manager, nil
}

// fetchRepository fetches the repository from the cache
func (c *repoCacheManager) fetchRepository() (ImageRepository, error) {
	var result ImageRepository
	repoKey := NewRepositoryKey(c.repoID.CanonicalName())
	bytes, _, err := c.cacheClient.GetKey(repoKey)
	if err != nil {
		return ImageRepository{}, err
	}
	if err = json.Unmarshal(bytes, &result); err != nil {
		return ImageRepository{}, err
	}
	return result, nil
}

// getTags gets the tags from the repository
func (c *repoCacheManager) getTags(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.clientTimeout)
	defer cancel()
	tags, err := c.client.Tags(ctx)
	if ctx.Err() == context.DeadlineExceeded {
		return nil, c.clientTimeoutError()
	}
	return tags, err
}

// storeRepository stores the repository from the cache
func (c *repoCacheManager) storeRepository(repo ImageRepository) error {
	repoKey := NewRepositoryKey(c.repoID.CanonicalName())
	bytes, err := json.Marshal(repo)
	if err != nil {
		return err
	}
	return c.cacheClient.SetKey(repoKey, c.now.Add(repoRefresh), bytes)
}

// fetchImagesResult is the result of fetching images from the cache
// invariant: len(imagesToUpdate) == imagesToUpdateRefreshCount + imagesToUpdateMissingCount
type fetchImagesResult struct {
	imagesFound                map[string]image.Info // images found in the cache
	imagesToUpdate             []imageToUpdate       // images which need to be updated
	imagesToUpdateRefreshCount int                   // number of imagesToUpdate which need updating due to their cache entry expiring
	imagesToUpdateMissingCount int                   // number of imagesToUpdate which need updating due to being missing
}

// fetchImages attemps to fetch the images with the provided tags from the cache.
// It returns the images found, those which require updating and details about
// why they need to be updated.
func (c *repoCacheManager) fetchImages(tags []string) (fetchImagesResult, error) {
	images := map[string]image.Info{}

	// Create a list of images that need updating
	var toUpdate []imageToUpdate

	// Counters for reporting what happened
	var missing, refresh int
	for _, tag := range tags {
		if tag == "" {
			return fetchImagesResult{}, fmt.Errorf("empty tag in fetched tags")
		}

		// See if we have the manifest already cached
		newID := c.repoID.ToRef(tag)
		key := NewManifestKey(newID.CanonicalRef())
		bytes, deadline, err := c.cacheClient.GetKey(key)
		// If err, then we don't have it yet. Update.
		switch {
		case err != nil: // by and large these are cache misses, but any error shall count as "not found"
			if err != ErrNotCached {
				c.logger.Log("warning", "error from cache", "err", err, "ref", newID)
			}
			missing++
			toUpdate = append(toUpdate, imageToUpdate{ref: newID, previousRefresh: initialRefresh})
		case len(bytes) == 0:
			c.logger.Log("warning", "empty result from cache", "ref", newID)
			missing++
			toUpdate = append(toUpdate, imageToUpdate{ref: newID, previousRefresh: initialRefresh})
		default:
			var entry registry.ImageEntry
			if err := json.Unmarshal(bytes, &entry); err == nil {
				if c.trace {
					c.logger.Log("trace", "found cached manifest", "ref", newID, "last_fetched", entry.LastFetched.Format(time.RFC3339), "deadline", deadline.Format(time.RFC3339))
				}

				if entry.ExcludedReason == "" {
					images[tag] = entry.Info
					if c.now.After(deadline) {
						previousRefresh := minRefresh
						lastFetched := entry.Info.LastFetched
						if !lastFetched.IsZero() {
							previousRefresh = deadline.Sub(lastFetched)
						}
						toUpdate = append(toUpdate, imageToUpdate{ref: newID, previousRefresh: previousRefresh, previousDigest: entry.Info.Digest})
						refresh++
					}
				} else {
					if c.trace {
						c.logger.Log("trace", "excluded in cache", "ref", newID, "reason", entry.ExcludedReason)
					}
					if c.now.After(deadline) {
						toUpdate = append(toUpdate, imageToUpdate{ref: newID, previousRefresh: excludedRefresh})
						refresh++
					}
				}
			}
		}
	}

	result := fetchImagesResult{
		imagesFound:                images,
		imagesToUpdate:             toUpdate,
		imagesToUpdateRefreshCount: refresh,
		imagesToUpdateMissingCount: missing,
	}

	return result, nil
}

// updateImages, refreshes the cache entries for the images passed. It may not succeed for all images.
// It returns the values stored in cache, the number of images it succeeded for and the number
// of images whose manifest wasn't found in the registry.
func (c *repoCacheManager) updateImages(ctx context.Context, images []imageToUpdate) (map[string]image.Info, int, int) {
	// The upper bound for concurrent fetches against a single host is
	// w.Burst, so limit the number of fetching goroutines to that.
	fetchers := make(chan struct{}, c.burst)
	awaitFetchers := &sync.WaitGroup{}

	ctxc, cancel := context.WithCancel(ctx)
	defer cancel()

	var successCount int
	var manifestUnknownCount int
	var result = map[string]image.Info{}
	var warnAboutRateLimit sync.Once
updates:
	for _, up := range images {
		// to avoid race condition, when accessing it in the go routine
		upCopy := up
		select {
		case <-ctxc.Done():
			break updates
		case fetchers <- struct{}{}:
		}
		awaitFetchers.Add(1)
		go func() {
			defer func() { awaitFetchers.Done(); <-fetchers }()
			ctxcc, cancel := context.WithTimeout(ctxc, c.clientTimeout)
			defer cancel()
			entry, err := c.updateImage(ctxcc, upCopy)
			if err != nil {
				if err, ok := errors.Cause(err).(net.Error); (ok && err.Timeout()) || ctxcc.Err() == context.DeadlineExceeded {
					// This was due to a context timeout, don't bother logging
					return
				}
				switch {
				case strings.Contains(err.Error(), "429"):
					// abort the image tags fetching if we've been rate limited
					warnAboutRateLimit.Do(func() {
						c.logger.Log("warn", "aborting image tag fetching due to rate limiting, will try again later")
						cancel()
					})
				case strings.Contains(err.Error(), "manifest unknown"):
					// Registry is corrupted, keep going, this manifest may not be relevant for automatic updates
					c.Lock()
					manifestUnknownCount++
					c.Unlock()
					c.logger.Log("warn", fmt.Sprintf("manifest for tag %s missing in repository %s", up.ref.Tag, up.ref.Name),
						"impact", "flux will fail to auto-release workloads with matching images, ask the repository administrator to fix the inconsistency")
				default:
					c.logger.Log("err", err, "ref", up.ref)
				}
				return
			}
			c.Lock()
			successCount++
			if entry.ExcludedReason == "" {
				result[upCopy.ref.Tag] = entry.Info
			}
			c.Unlock()
		}()
	}
	awaitFetchers.Wait()
	return result, successCount, manifestUnknownCount
}

func (c *repoCacheManager) updateImage(ctx context.Context, update imageToUpdate) (registry.ImageEntry, error) {
	imageID := update.ref

	if c.trace {
		c.logger.Log("trace", "refreshing manifest", "ref", imageID, "previous_refresh", update.previousRefresh.String())
	}

	ctx, cancel := context.WithTimeout(ctx, c.clientTimeout)
	defer cancel()
	// Get the image from the remote
	entry, err := c.client.Manifest(ctx, imageID.Tag)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return registry.ImageEntry{}, c.clientTimeoutError()
		}
		if _, ok := err.(*image.LabelTimestampFormatError); !ok {
			return registry.ImageEntry{}, err
		}
		c.logger.Log("err", err, "ref", imageID)
	}

	refresh := update.previousRefresh
	reason := ""
	switch {
	case entry.ExcludedReason != "":
		c.logger.Log("excluded", entry.ExcludedReason, "ref", imageID)
		refresh = excludedRefresh
		reason = "image is excluded"
	case update.previousDigest == "":
		entry.Info.LastFetched = c.now
		refresh = update.previousRefresh
		reason = "no prior cache entry for image"
	case entry.Info.Digest == update.previousDigest:
		entry.Info.LastFetched = c.now
		refresh = clipRefresh(refresh * 2)
		reason = "image digest is same"
	default: // i.e., not excluded, but the digests differ -> the tag was moved
		entry.Info.LastFetched = c.now
		refresh = clipRefresh(refresh / 2)
		reason = "image digest is different"
	}

	if c.trace {
		c.logger.Log("trace", "caching manifest", "ref", imageID, "last_fetched", c.now.Format(time.RFC3339), "refresh", refresh.String(), "reason", reason)
	}

	key := NewManifestKey(imageID.CanonicalRef())
	// Write back to memcached
	val, err := json.Marshal(entry)
	if err != nil {
		return registry.ImageEntry{}, err
	}
	err = c.cacheClient.SetKey(key, c.now.Add(refresh), val)
	if err != nil {
		return registry.ImageEntry{}, err
	}
	return entry, nil
}

func (r *repoCacheManager) clientTimeoutError() error {
	return fmt.Errorf("client timeout (%s) exceeded", r.clientTimeout)
}
