package git

import (
	"context"
	"sync"
	"time"
)

// Maintains several git mirrors as a set, with a mechanism for
// signalling when some have had changes.
//
// The advantage of it being a set is that you can add to it
// idempotently; if you need a repo to be mirrored, add it, and it
// will either already be mirrored, or that will be started.
type Mirrors struct {
	reposMu sync.Mutex
	repos   map[string]mirroringState

	changesMu sync.Mutex
	changes   chan map[string]struct{}

	wg *sync.WaitGroup
}

func NewMirrors() *Mirrors {
	return &Mirrors{
		repos:   make(map[string]mirroringState),
		changes: make(chan map[string]struct{}, 1),
		wg:      &sync.WaitGroup{},
	}
}

// Changes gets a channel upon which notifications of which repos have
// changed will be sent.
func (m *Mirrors) Changes() <-chan map[string]struct{} {
	return m.changes
}

func (m *Mirrors) signalChange(name string) {
	// So we don't try to write from two goroutines at once. This
	// procedure assumes writers will always go through the lock.
	m.changesMu.Lock()
	defer m.changesMu.Unlock()
	select {
	case c := <-m.changes:
		c[name] = struct{}{}
		m.changes <- c
	default:
		c := map[string]struct{}{}
		c[name] = struct{}{}
		m.changes <- c
	}
}

// Mirror instructs the Mirrors to track a particular repo; if there
// is already a repo with the name given, nothing is done. Otherwise,
// the repo given will be mirrored, and changes signalled on the
// channel obtained with `Changes()`.  The return value indicates
// whether the repo was already present (`true` if so, `false` otherwise).
func (m *Mirrors) Mirror(name string, remote Remote, options ...Option) bool {
	m.reposMu.Lock()
	defer m.reposMu.Unlock()

	_, ok := m.repos[name]
	if !ok {
		repo := NewRepo(remote, options...)
		stop := make(chan struct{})
		mir := mirroringState{stop: stop, repo: repo}
		m.repos[name] = mir
		// Forward any notifications from the repo
		go func() {
			for {
				select {
				case <-mir.repo.C:
					m.signalChange(name)
				case <-stop:
					return
				}
			}
		}()

		m.wg.Add(1)               // the wait group only waits for the refresh loop; the forwarding loop will exit when we close `stop`
		go repo.Start(stop, m.wg) // TODO(michael) is it safe to use the wait group dynamically like this?
	}
	return ok
}

// Get returns the named repo or nil, and a bool indicating whether
// the repo is being mirrored.
func (m *Mirrors) Get(name string) (*Repo, bool) {
	m.reposMu.Lock()
	defer m.reposMu.Unlock()
	r, ok := m.repos[name]
	if ok {
		return r.repo, true
	}
	return nil, false
}

// StopAllAndWait stops all the repos refreshing, and waits for them
// to indicate they've done so.
func (m *Mirrors) StopAllAndWait() {
	m.reposMu.Lock()
	for k, state := range m.repos {
		close(state.stop)
		state.repo.Clean()
		delete(m.repos, k)
	}
	m.reposMu.Unlock()
	m.wg.Wait()
}

// StopOne stops the repo given by `remote`, and cleans up after
// it (i.e., removes filesystem traces), if it is being tracked.
func (m *Mirrors) StopOne(name string) {
	m.reposMu.Lock()
	if state, ok := m.repos[name]; ok {
		close(state.stop)
		state.repo.Clean()
		delete(m.repos, name)
	}
	m.reposMu.Unlock()
}

// RefreshAll instructs all the repos to refresh, this means
// fetching updated refs, and associated objects. The given
// timeout is the timeout per mirror and _not_ the timeout
// for the whole operation. It returns a collection of
// eventual errors it encountered.
func (m *Mirrors) RefreshAll(timeout time.Duration) []error {
	m.reposMu.Lock()
	defer m.reposMu.Unlock()

	var errs []error
	for _, state := range m.repos {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := state.repo.Refresh(ctx); err != nil {
			errs = append(errs, err)
		}
		cancel()
	}
	return errs
}

// ---

type mirroringState struct {
	stop chan struct{}
	repo *Repo
}
