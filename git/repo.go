package git

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	ErrNoChanges = errors.New("no changes made in repo")
)

// Repo represents a (remote) git repo.
type Repo struct {
	// The URL to the config repo that holds the resource definition files. For
	// example, "https://github.com/myorg/conf.git", "git@foo.com:myorg/conf".
	URL string

	// The branch of the config repo that holds the resource definition files.
	Branch string

	// Path to a private key (e.g., an id_rsa file) with
	// permissions to clone and push to the config repo.
	Key string

	// The path within the config repo where files are stored.
	Path string
}

// Checkout is a local clone of the remote repo.
type Checkout struct {
	repo Repo
	Dir  string
	Config
}

// Config holds some values we use when working in the local copy of
// the repo
type Config struct {
	SyncTag   string
	NotesRef  string
	UserName  string
	UserEmail string
}

// Get a local clone of the upstream repo, and use the config given.
func (r Repo) Clone(c Config) (Checkout, error) {
	if r.URL == "" {
		return Checkout{}, NoRepoError
	}

	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
	if err != nil {
		return Checkout{}, err
	}

	// Hack, while it's not possible to mount a secret with a
	// particular mode in Kubernetes
	// FIXME this fails if the key is mounted as read-only; but we
	// cannot proceed if we can't do it.
	if err := narrowKeyPerms(r.Key); err != nil {
		return Checkout{}, err
	}

	repoDir, err := clone(workingDir, r.Key, r.URL, r.Branch)
	if err != nil {
		return Checkout{}, CloningError(r.URL, err)
	}

	if err := config(repoDir, c.UserName, c.UserEmail); err != nil {
		return Checkout{}, err
	}

	return Checkout{
		repo:   r,
		Dir:    repoDir,
		Config: c,
	}, nil
}

// WorkingClone makes a(nother) clone of the repository to use for
// e.g., rewriting files, so we can keep a pristine clone for reading
// out of.
func (c Checkout) WorkingClone() (Checkout, error) {
	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-working")
	if err != nil {
		return Checkout{}, err
	}

	repoDir, err := clone(workingDir, "", c.Dir, c.repo.Branch)
	if err != nil {
		return Checkout{}, err
	}

	return Checkout{
		repo:   c.repo,
		Dir:    repoDir,
		Config: c.Config,
	}, nil
}

// Clean a Checkout up (remove the clone)
func (c Checkout) Clean() {
	if c.Dir != "" {
		os.RemoveAll(c.Dir)
	}
}

// ManifestDir returns a path to where the files are
func (c Checkout) ManifestDir() string {
	return filepath.Join(c.Dir, c.repo.Path)
}

// CommitAndPush commits changes made in this checkout, along with any
// extra data as a note, and pushes the commit and note to the remote repo.
func (c Checkout) CommitAndPush(commitMessage, note string) error {
	if !check(c.Dir, c.repo.Path) {
		return ErrNoChanges
	}
	if err := commit(c.Dir, commitMessage); err != nil {
		return err
	}

	if note != "" {
		rev, err := headRevision(c.Dir)
		if err != nil {
			return err
		}
		if err := addNote(c.Dir, rev, c.NotesRef, note); err != nil {
			return err
		}
	}

	if err := push(c.repo.Key, c.Dir, c.repo.URL, c.repo.Branch, c.NotesRef); err != nil {
		return PushError(c.repo.URL, err)
	}
	return nil
}

// Pull fetches the latest commits on the branch we're using, and the latest notes
func (c Checkout) Pull() error {
	if err := pull(c.repo.Key, c.Dir, c.repo.URL, c.repo.Branch); err != nil {
		return err
	}
	notesRef, err := getNotesRef(c.Dir, c.NotesRef)
	if err != nil {
		return err
	}
	// this fetches and updates the local ref, so we'll see the new notes
	return fetch(c.repo.Key, c.Dir, c.repo.URL, notesRef+":"+notesRef)
}

func (c Checkout) HeadRevision() (string, error) {
	return headRevision(c.Dir)
}

func (c Checkout) RevisionsBetween(ref1, ref2 string) ([]string, error) {
	return revlist(c.Dir, ref1, ref2)
}

func (c Checkout) MoveTagAndPush(ref, msg string) error {
	return moveTagAndPush(c.Dir, c.repo.Key, c.SyncTag, ref, msg, c.repo.URL)
}
