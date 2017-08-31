package git

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"context"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/ssh"
	"time"
)

const (
	DefaultCloneTimeout = 2 * time.Minute
)

var (
	ErrNoChanges = errors.New("no changes made in repo")
)

// Repo represents a (remote) git repo.
type Repo struct {
	flux.GitRemoteConfig
	KeyRing ssh.KeyRing
}

// Checkout is a local clone of the remote repo.
type Checkout struct {
	repo Repo
	Dir  string
	Config
	realNotesRef string
	sync.RWMutex
}

// Config holds some values we use when working in the local copy of
// the repo
type Config struct {
	SyncTag   string
	NotesRef  string
	UserName  string
	UserEmail string
}

type Commit struct {
	Revision string
	Message  string
}

// Get a local clone of the upstream repo, and use the config given.
func (r Repo) Clone(ctx context.Context, c Config) (*Checkout, error) {
	if r.URL == "" {
		return nil, NoRepoError
	}

	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
	if err != nil {
		return nil, err
	}

	repoDir, err := clone(ctx, workingDir, r.KeyRing, r.URL, r.Branch)
	if err != nil {
		return nil, CloningError(r.URL, err)
	}

	if err := config(ctx, repoDir, c.UserName, c.UserEmail); err != nil {
		return nil, err
	}

	notesRef, err := getNotesRef(ctx, repoDir, c.NotesRef)
	if err != nil {
		return nil, err
	}

	// this fetches and updates the local ref, so we'll see notes
	if err := fetch(ctx, r.KeyRing, repoDir, r.URL, notesRef+":"+notesRef); err != nil {
		return nil, err
	}

	return &Checkout{
		repo:         r,
		Dir:          repoDir,
		Config:       c,
		realNotesRef: notesRef,
	}, nil
}

// WorkingClone makes a(nother) clone of the repository to use for
// e.g., rewriting files, so we can keep a pristine clone for reading
// out of.
func (c *Checkout) WorkingClone(ctx context.Context) (*Checkout, error) {
	c.Lock()
	defer c.Unlock()
	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-working")
	if err != nil {
		return nil, err
	}

	repoDir, err := clone(ctx, workingDir, nil, c.Dir, c.repo.Branch)
	if err != nil {
		return nil, err
	}

	if err := config(ctx, repoDir, c.UserName, c.UserEmail); err != nil {
		return nil, err
	}

	// this fetches and updates the local ref, so we'll see notes
	if err := fetch(ctx, nil, repoDir, c.Dir, c.realNotesRef+":"+c.realNotesRef); err != nil {
		return nil, err
	}

	return &Checkout{
		repo:         c.repo,
		Dir:          repoDir,
		Config:       c.Config,
		realNotesRef: c.realNotesRef,
	}, nil
}

// Clean a Checkout up (remove the clone)
func (c *Checkout) Clean() {
	if c.Dir != "" {
		os.RemoveAll(c.Dir)
	}
}

// ManifestDir returns a path to where the files are
func (c *Checkout) ManifestDir() string {
	return filepath.Join(c.Dir, c.repo.Path)
}

// CommitAndPush commits changes made in this checkout, along with any
// extra data as a note, and pushes the commit and note to the remote repo.
func (c *Checkout) CommitAndPush(ctx context.Context, commitMessage string, note *Note) error {
	c.Lock()
	defer c.Unlock()
	if !check(ctx, c.Dir, c.repo.Path) {
		return ErrNoChanges
	}
	if err := commit(ctx, c.Dir, commitMessage); err != nil {
		return err
	}

	if note != nil {
		rev, err := refRevision(ctx, c.Dir, "HEAD")
		if err != nil {
			return err
		}
		if err := addNote(ctx, c.Dir, rev, c.realNotesRef, note); err != nil {
			return err
		}
	}

	refs := []string{c.repo.Branch}
	ok, err := refExists(ctx, c.Dir, c.realNotesRef)
	if ok {
		refs = append(refs, c.realNotesRef)
	} else if err != nil {
		return err
	}

	if err := push(ctx, c.repo.KeyRing, c.Dir, c.repo.URL, refs); err != nil {
		return PushError(c.repo.URL, err)
	}
	return nil
}

// GetNote gets a note for the revision specified, or nil if there is no such note.
func (c *Checkout) GetNote(ctx context.Context, rev string) (*Note, error) {
	c.RLock()
	defer c.RUnlock()
	return getNote(ctx, c.Dir, c.realNotesRef, rev)
}

// Pull fetches the latest commits on the branch we're using, and the latest notes
func (c *Checkout) Pull(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()
	if err := pull(ctx, c.repo.KeyRing, c.Dir, c.repo.URL, c.repo.Branch); err != nil {
		return err
	}
	for _, ref := range []string{
		c.realNotesRef + ":" + c.realNotesRef,
		c.SyncTag,
	} {
		// this fetches and updates the local ref, so we'll see the new
		// notes; but it's possible that the upstream doesn't have this
		// ref.
		if err := fetch(ctx, c.repo.KeyRing, c.Dir, c.repo.URL, ref); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checkout) HeadRevision(ctx context.Context) (string, error) {
	c.RLock()
	defer c.RUnlock()
	return refRevision(ctx, c.Dir, "HEAD")
}

func (c *Checkout) TagRevision(ctx context.Context, tag string) (string, error) {
	c.RLock()
	defer c.RUnlock()
	return refRevision(ctx, c.Dir, tag)
}

func (c *Checkout) CommitsBetween(ctx context.Context, ref1, ref2 string) ([]Commit, error) {
	c.RLock()
	defer c.RUnlock()
	return onelinelog(ctx, c.Dir, ref1+".."+ref2)
}

func (c *Checkout) CommitsBefore(ctx context.Context, ref string) ([]Commit, error) {
	c.RLock()
	defer c.RUnlock()
	return onelinelog(ctx, c.Dir, ref)
}

func (c *Checkout) MoveTagAndPush(ctx context.Context, ref, msg string) error {
	c.Lock()
	defer c.Unlock()
	return moveTagAndPush(ctx, c.Dir, c.repo.KeyRing, c.SyncTag, ref, msg, c.repo.URL)
}

// ChangedFiles does a git diff listing changed files
func (c *Checkout) ChangedFiles(ctx context.Context, ref string) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	list, err := changedFiles(ctx, c.Dir, c.repo.Path, ref)
	if err == nil {
		for i, file := range list {
			list[i] = filepath.Join(c.Dir, file)
		}
	}
	return list, err
}

func (c *Checkout) NoteRevList(ctx context.Context) (map[string]struct{}, error) {
	c.Lock()
	defer c.Unlock()
	return noteRevList(ctx, c.Dir, c.realNotesRef)
}
