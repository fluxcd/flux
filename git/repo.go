package git

import (
	"errors"
	"io/ioutil"
	"os"
	"sync"

	"context"
	"time"

	"github.com/weaveworks/flux"
)

const (
	interval  = 5 * time.Minute
	opTimeout = 20 * time.Second

	DefaultCloneTimeout = 2 * time.Minute
	CheckPushTag        = "flux-write-check"
)

var (
	ErrNoChanges = errors.New("no changes made in repo")
	ErrNotReady  = errors.New("git repo not ready")
	ErrNoConfig  = errors.New("git repo has not valid config")
)

// Remote points at a git repo somewhere.
type Remote struct {
	URL string // clone from here
}

type Repo struct {
	// As supplied to constructor
	origin Remote

	// State
	mu     sync.RWMutex
	status flux.GitRepoStatus
	err    error
	dir    string

	notify chan struct{}
	C      chan struct{}
}

// NewRepo constructs a repo mirror which will sync itself.
func NewRepo(origin Remote) *Repo {
	r := &Repo{
		origin: origin,
		status: flux.RepoNew,
		err:    nil,
		notify: make(chan struct{}, 1), // `1` so that Notify doesn't block
		C:      make(chan struct{}, 1), // `1` so we don't block on completing a refresh
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

// Status reports that readiness status of this Git repo: whether it
// has been cloned, whether it is writable, and if not, the error
// stopping it getting to the next state.
func (r *Repo) Status() (flux.GitRepoStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status, r.err
}

func (r *Repo) setStatus(s flux.GitRepoStatus, err error) {
	r.mu.Lock()
	r.status = s
	r.err = err
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

// Revision returns the revision (SHA1) of the ref passed in
func (r *Repo) Revision(ctx context.Context, ref string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.dir == "" {
		return "", errors.New("git repo not initialised")
	}
	return refRevision(ctx, r.dir, ref)
}

func (r *Repo) CommitsBefore(ctx context.Context, ref, path string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return onelinelog(ctx, r.dir, ref, path)
}

func (r *Repo) CommitsBetween(ctx context.Context, ref1, ref2, path string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
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

		// TODO(michael): I don't think this is a real status; perhaps
		// have a no-op repo instead.
		case flux.RepoNoConfig:
			// this is not going to change in the lifetime of this
			// process
			return ErrNoConfig
		case flux.RepoNew:

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
				r.setStatus(flux.RepoCloned, nil)
				continue // with new status, skipping timer
			}
			dir = ""
			os.RemoveAll(rootdir)
			r.setStatus(flux.RepoNew, err)

		case flux.RepoCloned:
			ctx, cancel := context.WithTimeout(bg, opTimeout)
			err := checkPush(ctx, dir, url)
			cancel()
			if err == nil {
				r.setStatus(flux.RepoReady, nil)
				continue // with new status, skipping timer
			}
			r.setStatus(flux.RepoCloned, err)

		case flux.RepoReady:
			if err := r.refreshLoop(shutdown); err != nil {
				r.setStatus(flux.RepoNew, err)
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
	if r.status != flux.RepoReady {
		return ErrNotReady
	}
	if err := r.fetch(ctx); err != nil {
		return err
	}
	select {
	case r.C <- struct{}{}:
	default:
	}
	return nil
}

func (r *Repo) refreshLoop(shutdown <-chan struct{}) error {
	gitPoll := time.NewTimer(interval)
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
			ctx, cancel := context.WithTimeout(context.Background(), interval)
			err := r.Refresh(ctx)
			cancel()
			if err != nil {
				return err
			}
			gitPoll.Reset(interval)
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
	working, err := ioutil.TempDir(os.TempDir(), "flux-working")
	if err != nil {
		return "", err
	}
	return clone(ctx, working, r.dir, ref)
}
