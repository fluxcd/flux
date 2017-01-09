package git

import (
	"io"
	"io/ioutil"
	"os"
)

// Repo represents a remote git repo
type Repo struct {
	// The URL to the config repo that holds the resource definition files. For
	// example, "https://github.com/myorg/conf.git", "git@foo.com:myorg/conf".
	URL string

	// The branch of the config repo that holds the resource definition files.
	Branch string

	// The private key (e.g., the contents of an id_rsa file) with
	// permissions to clone and push to the config repo.
	Key string

	// The path within the config repo where files are stored.
	Path string
}

func (r Repo) Clone(stderr io.Writer) (path string, err error) {
	workingDir, err := ioutil.TempDir(os.TempDir(), "flux-gitclone")
	if err != nil {
		return "", err
	}

	repoDir, err := clone(stderr, workingDir, r.Key, r.URL, r.Branch)
	return repoDir, err
}

func (r Repo) CommitAndPush(path, commitMessage string) (string, error) {
	if !check(path, r.Path) {
		return "no changes made to files", nil
	}
	if err := commit(path, commitMessage); err != nil {
		return "", err
	}
	return "", push(r.Key, r.Branch, path)
}
