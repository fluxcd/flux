package git

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"context"
	"time"
)

const (
	defaultInterval = 5 * time.Minute
	defaultTimeout  = 20 * time.Second

	CheckPushTagPrefix = "flux-write-check"
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

func (err NotReadyError) Unwrap() error { return err.underlying }

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

type Repo struct {
	// As supplied to constructor
	origin   Remote
	branch   string
	interval time.Duration
	timeout  time.Duration
	readonly bool

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

type optionFunc func(*Repo)

func (f optionFunc) apply(r *Repo) {
	f(r)
}

type PollInterval time.Duration

func (p PollInterval) apply(r *Repo) {
	r.interval = time.Duration(p)
}

type Timeout time.Duration

func (t Timeout) apply(r *Repo) {
	r.timeout = time.Duration(t)
}

type Branch string

func (b Branch) apply(r *Repo) {
	r.branch = string(b)
}

type IsReadOnly bool

func (ro IsReadOnly) apply(r *Repo) {
	r.readonly = bool(ro)
}

var ReadOnly IsReadOnly = true

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
		timeout:  defaultTimeout,
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

// Readonly returns `true` if the repo was marked as readonly, `false`
// otherwise
func (r *Repo) Readonly() bool {
	return r.readonly
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

// BranchHead returns the HEAD revision (SHA1) of the configured branch
func (r *Repo) BranchHead(ctx context.Context) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return "", err
	}
	return refRevision(ctx, r.dir, "heads/"+r.branch)
}

func (r *Repo) CommitsBefore(ctx context.Context, ref string, firstParent bool, paths ...string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return nil, err
	}
	return onelinelog(ctx, r.dir, ref, paths, firstParent)
}

func (r *Repo) CommitsBetween(ctx context.Context, ref1, ref2 string, firstParent bool, paths ...string) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return nil, err
	}
	return onelinelog(ctx, r.dir, ref1+".."+ref2, paths, firstParent)
}

func (r *Repo) VerifyTag(ctx context.Context, tag string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return "", err
	}
	return verifyTag(ctx, r.dir, tag)
}

func (r *Repo) VerifyCommit(ctx context.Context, commit string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return err
	}
	return verifyCommit(ctx, r.dir, commit)
}

func (r *Repo) DeleteTag(ctx context.Context, tag string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := r.errorIfNotReady(); err != nil {
		return err
	}
	return deleteTag(ctx, r.dir, tag, r.origin.URL)
}

func (r *Repo) NoteRevList(ctx context.Context, notesRef string) (map[string]struct{}, error) {
	return noteRevList(ctx, r.Dir(), notesRef)
}

// GetNote gets a note for the revision specified, or nil if there is no such note.
func (r *Repo) GetNote(ctx context.Context, rev, notesRef string, note interface{}) (bool, error) {
	return getNote(ctx, r.Dir(), notesRef, rev, note)
}

// step attempts to advance the repo state machine, and returns `true`
// if it has made progress, `false` otherwise.
func (r *Repo) step(bg context.Context) bool {
	r.mu.RLock()
	url := r.origin.URL
	dir := r.dir
	status := r.status
	r.mu.RUnlock()

	switch status {

	case RepoNoConfig:
		// this is not going to change in the lifetime of this
		// process, so just exit.
		return false

	case RepoNew:
		rootdir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
		if err != nil {
			panic(err)
		}

		ctx, cancel := context.WithTimeout(bg, r.timeout)
		dir, err = mirror(ctx, rootdir, url)
		cancel()
		if err == nil {
			r.mu.Lock()
			r.dir = dir
			ctx, cancel := context.WithTimeout(bg, r.timeout)
			err = r.fetch(ctx)
			cancel()
			r.mu.Unlock()
		}
		if err == nil {
			r.setUnready(RepoCloned, ErrClonedOnly)
			return true
		}
		dir = ""
		os.RemoveAll(rootdir)
		r.setUnready(RepoNew, err)
		return false

	case RepoCloned:
		ctx, cancel := context.WithTimeout(bg, r.timeout)
		defer cancel()

		if r.branch != "" {
			r.mu.Lock()
			// The remote may have changed between `RepoNew` and this
			// iteration of `RepoCloned`. Fetch again to pick-up any
			// changes that may have been made.
			err := r.fetch(ctx)
			r.mu.Unlock()
			if err != nil {
				r.setUnready(RepoCloned, err)
				return false
			}

			ok, err := refExists(ctx, dir, "refs/heads/"+r.branch)
			if err != nil {
				r.setUnready(RepoCloned, err)
				return false
			}
			if !ok {
				r.setUnready(RepoCloned, fmt.Errorf("configured branch '%s' does not exist", r.branch))
				return false
			}
		}

		if !r.readonly {
			err := checkPush(ctx, dir, url, r.branch)
			if err != nil {
				r.setUnready(RepoCloned, err)
				return false
			}
		}

		r.setReady()
		// Treat every transition to ready as a refresh, so
		// that any listeners can respond in the same way.
		r.refreshed()
		return true

	case RepoReady:
		return false
	}

	return false
}

// Ready tries to advance the cloning process along as far as
// possible, and returns an error if it is not able to get to a ready
// state.
func (r *Repo) Ready(ctx context.Context) error {
	for r.step(ctx) {
		// keep going!
	}
	_, err := r.Status()
	return err
}

// Start begins synchronising the repo by cloning it, then fetching
// the required tags and so on.
func (r *Repo) Start(shutdown <-chan struct{}, done *sync.WaitGroup) error {
	defer done.Done()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
		advanced := r.step(ctx)
		cancel()

		if advanced {
			continue
		}

		status, _ := r.Status()
		if status == RepoReady {
			if err := r.refreshLoop(shutdown); err != nil {
				r.setUnready(RepoNew, err)
				continue // with new status, skipping timer
			}
		} else if status == RepoNoConfig {
			return nil
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
			ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
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
	path, err := clone(ctx, working, r.dir, ref)
	if err != nil {
		os.RemoveAll(working)
	}
	return path, err
}
