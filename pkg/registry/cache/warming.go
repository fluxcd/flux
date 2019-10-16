package cache

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
)

const askForNewImagesInterval = time.Minute

// start off assuming an image will change about an hour from first
// seeing it
const initialRefresh = 1 * time.Hour

// never try to refresh a tag faster than this
const minRefresh = 5 * time.Minute

// never set a refresh deadline longer than this
const maxRefresh = 7 * 24 * time.Hour

// excluded images get an constant, fairly long refresh deadline; we
// don't expect them to become usable e.g., change architecture.
const excludedRefresh = 24 * time.Hour

// the whole set of image manifests for a repo gets a long refresh; in
// general we write it back every time we go 'round the loop, so this
// is mainly for the effect of making garbage collection less likely.
const repoRefresh = maxRefresh

func clipRefresh(r time.Duration) time.Duration {
	if r > maxRefresh {
		return maxRefresh
	}
	if r < minRefresh {
		return minRefresh
	}
	return r
}

// Warmer refreshes the information kept in the cache from remote
// registries.
type Warmer struct {
	clientFactory registry.ClientFactory
	cache         Client
	burst         int
	Trace         bool
	Priority      chan image.Name
	Notify        func()
}

// NewWarmer creates cache warmer that (when Loop is invoked) will
// periodically refresh the values kept in the cache.
func NewWarmer(cf registry.ClientFactory, cacheClient Client, burst int) (*Warmer, error) {
	if cf == nil || cacheClient == nil || burst <= 0 {
		return nil, errors.New("arguments must be non-nil (or > 0 in the case of burst)")
	}
	return &Warmer{
		clientFactory: cf,
		cache:         cacheClient,
		burst:         burst,
	}, nil
}

// .. and this is what we keep in the backlog
type backlogItem struct {
	image.Name
	registry.Credentials
}

// Loop continuously gets the images to populate the cache with,
// and populate the cache with them.
func (w *Warmer) Loop(logger log.Logger, stop <-chan struct{}, wg *sync.WaitGroup, imagesToFetchFunc func() registry.ImageCreds) {
	defer wg.Done()

	refresh := time.Tick(askForNewImagesInterval)
	imageCreds := imagesToFetchFunc()
	backlog := imageCredsToBacklog(imageCreds)

	// We have some fine control over how long to spend on each fetch
	// operation, since they are given a `context`. For now though,
	// just rattle through them one by one, however long they take.
	ctx := context.Background()

	// NB the implicit contract here is that the prioritised
	// image has to have been running the last time we
	// requested the credentials.
	priorityWarm := func(name image.Name) {
		logger.Log("priority", name.String())
		if creds, ok := imageCreds[name]; ok {
			w.warm(ctx, time.Now(), logger, name, creds)
		} else {
			logger.Log("priority", name.String(), "err", "no creds available")
		}
	}

	// This loop acts keeps a kind of priority queue, whereby image
	// names coming in on the `Priority` channel are looked up first.
	// If there are none, images used in the cluster are refreshed;
	// but no more often than once every `askForNewImagesInterval`,
	// since there is no effective back-pressure on cache refreshes
	// and it would spin freely otherwise.
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case name := <-w.Priority:
			priorityWarm(name)
			continue
		default:
		}

		if len(backlog) > 0 {
			im := backlog[0]
			backlog = backlog[1:]
			w.warm(ctx, time.Now(), logger, im.Name, im.Credentials)
		} else {
			select {
			case <-stop:
				logger.Log("stopping", "true")
				return
			case <-refresh:
				imageCreds = imagesToFetchFunc()
				backlog = imageCredsToBacklog(imageCreds)
			case name := <-w.Priority:
				priorityWarm(name)
			}
		}
	}
}

func imageCredsToBacklog(imageCreds registry.ImageCreds) []backlogItem {
	backlog := make([]backlogItem, len(imageCreds))
	var i int
	for name, cred := range imageCreds {
		backlog[i] = backlogItem{name, cred}
		i++
	}
	return backlog
}

func (w *Warmer) warm(ctx context.Context, now time.Time, logger log.Logger, id image.Name, creds registry.Credentials) {
	errorLogger := log.With(logger, "canonical_name", id.CanonicalName(), "auth", creds)

	cacheManager, err := newRepoCacheManager(now, id, w.clientFactory, creds, time.Minute, w.burst, w.Trace, errorLogger, w.cache)
	if err != nil {
		errorLogger.Log("err", err.Error())
		return
	}

	// This is what we're going to write back to the cache
	var repo ImageRepository
	repo, err = cacheManager.fetchRepository()
	if err != nil && err != ErrNotCached {
		errorLogger.Log("err", errors.Wrap(err, "fetching previous result from cache"))
		return
	}
	// Save for comparison later
	oldImages := repo.Images

	// Now we have the previous result; everything after will be
	// attempting to refresh that value. Whatever happens, at the end
	// we'll write something back.
	defer func() {
		if err := cacheManager.storeRepository(repo); err != nil {
			errorLogger.Log("err", errors.Wrap(err, "writing result to cache"))
		}
	}()

	tags, err := cacheManager.getTags(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "net/http: request canceled") {
			errorLogger.Log("err", errors.Wrap(err, "requesting tags"))
			repo.LastError = err.Error()
		}
		return
	}

	fetchResult, err := cacheManager.fetchImages(tags)
	if err != nil {
		logger.Log("err", err, "tags", tags)
		repo.LastError = err.Error()
		return // abort and let the error be written
	}
	newImages := fetchResult.imagesFound

	var successCount int
	var manifestUnknownCount int

	if len(fetchResult.imagesToUpdate) > 0 {
		logger.Log("info", "refreshing image", "image", id, "tag_count", len(tags),
			"to_update", len(fetchResult.imagesToUpdate),
			"of_which_refresh", fetchResult.imagesToUpdateRefreshCount, "of_which_missing", fetchResult.imagesToUpdateMissingCount)
		var images map[string]image.Info
		images, successCount, manifestUnknownCount = cacheManager.updateImages(ctx, fetchResult.imagesToUpdate)
		for k, v := range images {
			newImages[k] = v
		}
		logger.Log("updated", id.String(), "successful", successCount, "attempted", len(fetchResult.imagesToUpdate))
	}

	// We managed to fetch new metadata for everything we needed.
	// Ratchet the result forward.
	if successCount+manifestUnknownCount == len(fetchResult.imagesToUpdate) {
		repo = ImageRepository{
			LastUpdate: time.Now(),
			RepositoryMetadata: image.RepositoryMetadata{
				Images: newImages,
				Tags:   tags,
			},
		}
		// If we got through all that without bumping into `HTTP 429
		// Too Many Requests` (or other problems), we can potentially
		// creep the rate limit up
		w.clientFactory.Succeed(id.CanonicalName())
	}

	if w.Notify != nil {
		cacheTags := StringSet{}
		for t := range oldImages {
			cacheTags[t] = struct{}{}
		}

		// If there's more tags than there used to be, there must be
		// at least one new tag.
		if len(cacheTags) < len(tags) {
			w.Notify()
			return
		}
		// Otherwise, check whether there are any entries in the
		// fetched tags that aren't in the cached tags.
		tagSet := NewStringSet(tags)
		if !tagSet.Subset(cacheTags) {
			w.Notify()
		}
	}
}

// StringSet is a set of strings.
type StringSet map[string]struct{}

// NewStringSet returns a StringSet containing exactly the strings
// given as arguments.
func NewStringSet(ss []string) StringSet {
	res := StringSet{}
	for _, s := range ss {
		res[s] = struct{}{}
	}
	return res
}

// Subset returns true if `s` is a subset of `t` (including the case
// of having the same members).
func (s StringSet) Subset(t StringSet) bool {
	for k := range s {
		if _, ok := t[k]; !ok {
			return false
		}
	}
	return true
}
