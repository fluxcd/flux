package git

import (
	"errors"
	"io/ioutil"
	"os"
	"sync"

	"context"
	"time"
)

const (
	defaultInterval = 5 * time.Minute
	opTimeout       = 20 * time.Second

	DefaultCloneTimeout = 2 * time.Minute
	CheckPushTag        = "flux-write-check"
)

var (
	ErrNoChanges  = errors.New("no changes made in repo")
	ErrNoConfig   = errors.New("git repo does not have valid config")
	ErrNotCloned  = errors.New("git repo has not been cloned yet")
	ErrClonedOnly = errors.New("git repo has been cloned but not yet checked for write access")
)

type NotReadyError struct {
	underlying error
}

func (err NotReadyError) Error() string {
	return "git repo not ready: " + err.underlying.Error()
}

// GitRepoStatus represents the progress made synchronising with a git
// repo. These are given below in expected order, but the status may
// go backwards if e.g., a deploy key is deleted.
type GitRepoStatus string

const (
	RepoNoConfig GitRepoStatus = "unconfigured" // configuration is empty
	RepoNew      GitRepoStatus = "new"          // no attempt made to clone it yet
	RepoCloned   GitRepoStatus = "cloned"       // has been read (cloned); no attempt made to write
	RepoReady    GitRepoStatus = "ready"        // has been written to, so ready to sync
)

// Remote points at a git repo somewhere.
type Remote struct {
	URL string // clone from here
}

type Repo struct {
	// As supplied to constructor
	origin   Remote
	interval time.Duration

	// State
	mu     sync.RWMutex
	status GitRepoStatus
	err    error
	dir    string

	notify chan struct{}
	C      chan struct{}
}

type Option interface {
	apply(*Repo)
}

type PollInterval time.Duration

func (p PollInterval) apply(r *Repo) {
	r.interval = time.Duration(p)
}

// NewRepo constructs a repo mirror which will sync itself.
func NewRepo(origin Remote, opts ...Option) *Repo {
	status := RepoNew
	if origin.URL == "" {
		status = RepoNoConfig
	}
	r := &Repo{
		origin:   origin,
		status:   status,
		interval: defaultInterval,
		err:      ErrNotCloned,
		notify:   make(chan struct{}, 1), // `1` so that Notify doesn't block
		C:        make(chan struct{}, 1), // `1` so we don't block on completing a refresh
	}
	for _, opt := range opts {
		opt.apply(r)
	}
	return r
}

// Origin returns the Remote with which the Repo was constructed.
func (r *Repo) Origin() Remote {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.origin
}

// Dir returns the local directory into which the repo has been
// cloned, if it has been cloned.
func (r *Repo) Dir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dir
}

// Clean removes the mirrored repo. Syncing may continue with a new
// directory, so you may need to stop that first.
func (r *Repo) Clean() {
	r.mu.Lock()
	if r.dir != "" {
		os.RemoveAll(r.dir)
	}
	r.dir = ""
	r.status = RepoNew
	r.mu.Unlock()
}

// Status reports that readiness status of this Git repo: whether it
// has been cloned and is writable, and if not, the error stopping it
// getting to the next state.
func (r *Repo) Status() (GitRepoStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status, r.err
}

func (r *Repo) setUnready(s GitRepoStatus, err error) {
	r.mu.Lock()
	r.status = s
	r.err = err
	r.mu.Unlock()
}

func (r *Repo) setReady() {
	r.mu.Lock()
	r.status = RepoReady
	r.err = nil
	r.mu.Unlock()
}

// Notify tells the repo that it should fetch from the origin as soon
// as possible. It does not block.
func (r *Repo) Notify() {
	select {
	case r.notify <- struct{}{}:
		// duly notified
	default:
		// notification already pending
	}
}

// refreshed indicates that the repo has successfully fetched from upstream.
func (r *Repo) refreshed() {
	select {
	case r.C <- struct{}{}:
	default:
	}
}

// errorIfNotReady returns the appropriate error if the repo is not
// ready, and `nil` otherwise.
func (r *Repo) errorIfNotReady() error {
	switch r.status {
	case RepoReady:
		return nil
	case RepoNoConfig:
		return ErrNoConfig
	default:
		return NotReadyError{r.err}
	}
}

// Revision returns the revision (SHA1) of the ref passed in
func (r *Repo) Revision(ctx context.Context, ref string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return "", err
	}
	return refRevision(ctx, r.dir, ref)
}

func (r *Repo) CommitsBefore(ctx context.Context, ref, path string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return nil, err
	}
	return onelinelog(ctx, r.dir, ref, path)
}

func (r *Repo) CommitsBetween(ctx context.Context, ref1, ref2, path string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return nil, err
	}
	return onelinelog(ctx, r.dir, ref1+".."+ref2, path)
}

// Start begins synchronising the repo by cloning it, then fetching
// the required tags and so on.
func (r *Repo) Start(shutdown <-chan struct{}, done *sync.WaitGroup) error {
	defer done.Done()

	for {

		r.mu.RLock()
		url := r.origin.URL
		dir := r.dir
		status := r.status
		r.mu.RUnlock()

		bg := context.Background()

		switch status {

		case RepoNoConfig:
			// this is not going to change in the lifetime of this
			// process, so just exit.
			return nil
		case RepoNew:

			rootdir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(bg, opTimeout)
			dir, err = mirror(ctx, rootdir, url)
			cancel()
			if err == nil {
				r.mu.Lock()
				r.dir = dir
				ctx, cancel := context.WithTimeout(bg, opTimeout)
				err = r.fetch(ctx)
				cancel()
				r.mu.Unlock()
			}
			if err == nil {
				r.setUnready(RepoCloned, ErrClonedOnly)
				continue // with new status, skipping timer
			}
			dir = ""
			os.RemoveAll(rootdir)
			r.setUnready(RepoNew, err)

		case RepoCloned:
			ctx, cancel := context.WithTimeout(bg, opTimeout)
			err := checkPush(ctx, dir, url)
			cancel()
			if err == nil {
				r.setReady()
				// Treat every transition to ready as a refresh, so
				// that any listeners can respond in the same way.
				r.refreshed()
				continue // with new status, skipping timer
			}
			r.setUnready(RepoCloned, err)

		case RepoReady:
			if err := r.refreshLoop(shutdown); err != nil {
				r.setUnready(RepoNew, err)
				continue // with new status, skipping timer
			}
		}

		tryAgain := time.NewTimer(10 * time.Second)
		select {
		case <-shutdown:
			if !tryAgain.Stop() {
				<-tryAgain.C
			}
			return nil
		case <-tryAgain.C:
			continue
		}
	}
}

func (r *Repo) Refresh(ctx context.Context) error {
	// the lock here and below is difficult to avoid; possibly we
	// could clone to another repo and pull there, then swap when complete.
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.errorIfNotReady(); err != nil {
		return err
	}
	if err := r.fetch(ctx); err != nil {
		return err
	}
	r.refreshed()
	return nil
}

func (r *Repo) refreshLoop(shutdown <-chan struct{}) error {
	gitPoll := time.NewTimer(r.interval)
	for {
		select {
		case <-shutdown:
			if !gitPoll.Stop() {
				<-gitPoll.C
			}
			return nil
		case <-gitPoll.C:
			r.Notify()
		case <-r.notify:
			if !gitPoll.Stop() {
				select {
				case <-gitPoll.C:
				default:
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
			err := r.Refresh(ctx)
			cancel()
			if err != nil {
				return err
			}
			gitPoll.Reset(r.interval)
		}
	}
}

// fetch gets updated refs, and associated objects, from the upstream.
func (r *Repo) fetch(ctx context.Context) error {
	if err := fetch(ctx, r.dir, "origin"); err != nil {
		return err
	}
	return nil
}

// workingClone makes a non-bare clone, at `ref` (probably a branch),
// and returns the filesystem path to it.
func (r *Repo) workingClone(ctx context.Context, ref string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return "", err
	}
	working, err := ioutil.TempDir(os.TempDir(), "flux-working")
	if err != nil {
		return "", err
	}
	return clone(ctx, working, r.dir, ref)
}
