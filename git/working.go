package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrReadOnly = errors.New("cannot make a working clone of a read-only git repo")
)

// Config holds some values we use when working in the working clone of
// a repo.
type Config struct {
	Branch           string   // branch we're syncing to
	Paths            []string // paths within the repo containing files we care about
	SyncTag          string
	NotesRef         string
	UserName         string
	UserEmail        string
	SigningKey       string
	SetAuthor        bool
	SkipMessage      string
}

// Checkout is a local working clone of the remote repo. It is
// intended to be used for one-off "transactions", e.g,. committing
// changes then pushing upstream. It has no locking.
type Checkout struct {
	dir          string
	config       Config
	upstream     Remote
	realNotesRef string // cache the notes ref, since we use it to push as well
}

type Commit struct {
	Signature Signature
	Revision  string
	Message   string
}

// CommitAction - struct holding commit information
type CommitAction struct {
	Author     string
	Message    string
	SigningKey string
}

// TagAction - struct holding tag information
type TagAction struct {
	Revision   string
	Message    string
	SigningKey string
}

// Clone returns a local working clone of the sync'ed `*Repo`, using
// the config given.
func (r *Repo) Clone(ctx context.Context, conf Config) (*Checkout, error) {
	if r.readonly {
		return nil, ErrReadOnly
	}

	upstream := r.Origin()
	repoDir, err := r.workingClone(ctx, conf.Branch)
	if err != nil {
		return nil, err
	}

	if err := config(ctx, repoDir, conf.UserName, conf.UserEmail); err != nil {
		os.RemoveAll(repoDir)
		return nil, err
	}

	// We'll need the notes ref for pushing it, so make sure we have
	// it. This assumes we're syncing it (otherwise we'll likely get conflicts)
	realNotesRef, err := getNotesRef(ctx, repoDir, conf.NotesRef)
	if err != nil {
		os.RemoveAll(repoDir)
		return nil, err
	}

	r.mu.RLock()
	if err := fetch(ctx, repoDir, r.dir, realNotesRef+":"+realNotesRef); err != nil {
		os.RemoveAll(repoDir)
		r.mu.RUnlock()
		return nil, err
	}
	r.mu.RUnlock()

	return &Checkout{
		dir:          repoDir,
		upstream:     upstream,
		realNotesRef: realNotesRef,
		config:       conf,
	}, nil
}

// Clean a Checkout up (remove the clone)
func (c *Checkout) Clean() {
	if c.dir != "" {
		os.RemoveAll(c.dir)
	}
}

// Dir returns the path to the repo
func (c *Checkout) Dir() string {
	return c.dir
}

// ManifestDirs returns the paths to the manifests files. It ensures
// that at least one path is returned, so that it can be used with
// `Manifest.LoadManifests`.
func (c *Checkout) ManifestDirs() []string {
	if len(c.config.Paths) == 0 {
		return []string{c.dir}
	}

	paths := make([]string, len(c.config.Paths), len(c.config.Paths))
	for i, p := range c.config.Paths {
		paths[i] = filepath.Join(c.dir, p)
	}
	return paths
}

// CommitAndPush commits changes made in this checkout, along with any
// extra data as a note, and pushes the commit and note to the remote repo.
func (c *Checkout) CommitAndPush(ctx context.Context, commitAction CommitAction, note interface{}) error {
	if !check(ctx, c.dir, c.config.Paths) {
		return ErrNoChanges
	}

	commitAction.Message += c.config.SkipMessage
	if commitAction.SigningKey == "" {
		commitAction.SigningKey = c.config.SigningKey
	}

	if err := commit(ctx, c.dir, commitAction); err != nil {
		return err
	}

	if note != nil {
		rev, err := c.HeadRevision(ctx)
		if err != nil {
			return err
		}
		if err := addNote(ctx, c.dir, rev, c.config.NotesRef, note); err != nil {
			return err
		}
	}

	refs := []string{c.config.Branch}
	ok, err := refExists(ctx, c.dir, c.realNotesRef)
	if ok {
		refs = append(refs, c.realNotesRef)
	} else if err != nil {
		return err
	}

	if err := push(ctx, c.dir, c.upstream.URL, refs); err != nil {
		return PushError(c.upstream.URL, err)
	}
	return nil
}

// GetNote gets a note for the revision specified, or nil if there is no such note.
func (c *Checkout) GetNote(ctx context.Context, rev string, note interface{}) (bool, error) {
	return getNote(ctx, c.dir, c.realNotesRef, rev, note)
}

func (c *Checkout) HeadRevision(ctx context.Context) (string, error) {
	return refRevision(ctx, c.dir, "HEAD")
}

func (c *Checkout) SyncRevision(ctx context.Context) (string, error) {
	return refRevision(ctx, c.dir, "tags/"+c.config.SyncTag)
}

func (c *Checkout) MoveSyncTagAndPush(ctx context.Context, tagAction TagAction) error {
	if tagAction.SigningKey == "" {
		tagAction.SigningKey = c.config.SigningKey
	}
	return moveTagAndPush(ctx, c.dir, c.config.SyncTag, c.upstream.URL, tagAction)
}

func (c *Checkout) VerifySyncTag(ctx context.Context) (string, error) {
	return verifyTag(ctx, c.dir, c.config.SyncTag)
}

// ChangedFiles does a git diff listing changed files
func (c *Checkout) ChangedFiles(ctx context.Context, ref string) ([]string, error) {
	list, err := changed(ctx, c.dir, ref, c.config.Paths)
	if err == nil {
		for i, file := range list {
			list[i] = filepath.Join(c.dir, file)
		}
	}
	return list, err
}

func (c *Checkout) NoteRevList(ctx context.Context) (map[string]struct{}, error) {
	return noteRevList(ctx, c.dir, c.realNotesRef)
}

func (c *Checkout) Checkout(ctx context.Context, rev string) error {
	return checkout(ctx, c.dir, rev)
}
