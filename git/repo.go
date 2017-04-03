package git

import (
	"errors"
	"io/ioutil"
	"os"
)

var (
	ErrNoChanges = errors.New("no changes made in repo")
)

// Repo represents a remote git repo
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

func (r Repo) Clone() (path string, err error) {
	if r.URL == "" {
		return "", NoRepoError
	}

	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
	if err != nil {
		return "", err
	}

	// Hack, while it's not possible to mount a secret with a
	// particular mode in Kubernetes
	if err := narrowKeyPerms(r.Key); err != nil {
		return "", err
	}

	repoDir, err := clone(workingDir, r.Key, r.URL, r.Branch)
	if err != nil {
		return "", CloningError(r.URL, err)
	}
	return repoDir, nil
}

func (r Repo) CommitAndPush(path, commitMessage string) error {
	if !check(path, r.Path) {
		return ErrNoChanges
	}
	if err := commit(path, commitMessage); err != nil {
		return err
	}
	if err := push(r.Key, r.Branch, path); err != nil {
		return PushError(r.URL, err)
	}
	return nil
}

func (r Repo) Pull(path string) error {
	return pull(r.Key, r.Branch, path)
}
