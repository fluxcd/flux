package git

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// Get a local clone of the upstream repo, and use the config given.
func (r Repo) Clone(c Config) (*Checkout, error) {
	if r.URL == "" {
		return nil, NoRepoError
	}

	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
	if err != nil {
		return nil, err
	}

	// ssh will balk if it's asked to use a key file with loose
	// permissions. However, this could be mounted as a Kubernetes
	// secret -- and it's not possible to mount a secret with a
	// particular mode in Kubernetes < 1.6. So, for old Kubernetes, we
	// need it mounted RW, and we'll narrow the permissions.
	if r.Key != "" {
		stat, err := os.Stat(r.Key)
		if err != nil {
			return nil, err
		}
		if stat.Mode()&os.ModePerm != 0400 {
			if err := narrowKeyPerms(r.Key); err != nil {
				return nil, err
			}
		}
	}

	repoDir, err := clone(workingDir, r.Key, r.URL, r.Branch)
	if err != nil {
		return nil, CloningError(r.URL, err)
	}

	if err := config(repoDir, c.UserName, c.UserEmail); err != nil {
		return nil, err
	}

	notesRef, err := getNotesRef(repoDir, c.NotesRef)
	if err != nil {
		return nil, err
	}

	// this fetches and updates the local ref, so we'll see notes
	if err := fetch(r.Key, repoDir, r.URL, notesRef+":"+notesRef); err != nil &&
		!strings.Contains(err.Error(), "Couldn't find remote ref") {
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
func (c *Checkout) WorkingClone() (*Checkout, error) {
	c.Lock()
	defer c.Unlock()
	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-working")
	if err != nil {
		return nil, err
	}

	repoDir, err := clone(workingDir, "", c.Dir, c.repo.Branch)
	if err != nil {
		return nil, err
	}

	if err := config(repoDir, c.UserName, c.UserEmail); err != nil {
		return nil, err
	}

	// this fetches and updates the local ref, so we'll see notes
	if err := fetch("", repoDir, c.Dir, c.realNotesRef+":"+c.realNotesRef); err != nil &&
		!strings.Contains(err.Error(), "Couldn't find remote ref") {
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
func (c *Checkout) CommitAndPush(commitMessage string, note *Note) error {
	c.Lock()
	defer c.Unlock()
	if !check(c.Dir, c.repo.Path) {
		return ErrNoChanges
	}
	if err := commit(c.Dir, commitMessage); err != nil {
		return err
	}

	if note != nil {
		rev, err := headRevision(c.Dir)
		if err != nil {
			return err
		}
		if err := addNote(c.Dir, rev, c.realNotesRef, note); err != nil {
			return err
		}
	}

	refs := []string{c.repo.Branch}
	ok, err := refExists(c.Dir, c.realNotesRef)
	if ok {
		refs = append(refs, c.realNotesRef)
	} else if err != nil {
		return err
	}

	if err := push(c.repo.Key, c.Dir, c.repo.URL, refs); err != nil {
		return PushError(c.repo.URL, err)
	}
	return nil
}

// GetNote gets a note for the revision specified, or "" if there is no such note.
func (c *Checkout) GetNote(rev string) (*Note, error) {
	c.RLock()
	defer c.RUnlock()
	return getNote(c.Dir, c.realNotesRef, rev)
}

// Pull fetches the latest commits on the branch we're using, and the latest notes
func (c *Checkout) Pull() error {
	c.Lock()
	defer c.Unlock()
	if err := pull(c.repo.Key, c.Dir, c.repo.URL, c.repo.Branch); err != nil {
		return err
	}
	for _, ref := range []string{
		c.realNotesRef + ":" + c.realNotesRef,
		c.SyncTag,
	} {
		// this fetches and updates the local ref, so we'll see the new
		// notes; but it's possible that the upstream doesn't have this
		// ref.
		if err := fetch(c.repo.Key, c.Dir, c.repo.URL, ref); err != nil &&
			!strings.Contains(err.Error(), "Couldn't find remote ref") {
			return err
		}
	}
	return nil
}

func (c *Checkout) HeadRevision() (string, error) {
	c.RLock()
	defer c.RUnlock()
	return headRevision(c.Dir)
}

func (c *Checkout) RevisionsBetween(ref1, ref2 string) ([]string, error) {
	c.RLock()
	defer c.RUnlock()
	return revlist(c.Dir, ref1+".."+ref2)
}

func (c *Checkout) RevisionsBefore(ref string) ([]string, error) {
	c.RLock()
	defer c.RUnlock()
	return revlist(c.Dir, ref)
}

func (c *Checkout) MoveTagAndPush(ref, msg string) error {
	c.Lock()
	defer c.Unlock()
	return moveTagAndPush(c.Dir, c.repo.Key, c.SyncTag, ref, msg, c.repo.URL)
}

// ChangedFiles does a git diff listing changed files
func (c *Checkout) ChangedFiles(ref string) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	list, err := changedFiles(c.Dir, c.repo.Path, ref)
	if err == nil {
		for i, file := range list {
			list[i] = filepath.Join(c.Dir, file)
		}
	}
	return list, err
}
