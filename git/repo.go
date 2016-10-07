package git

import (
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

	// The file containing the private key with permissions to clone and push to
	// the config repo.
	Key string

	// The path within the config repo where files are stored.
	Path string
}

func (r Repo) Clone() (path string, keyFile string, err error) {
	workingDir, err := ioutil.TempDir(os.TempDir(), "fluxy-gitclone")
	if err != nil {
		return "", "", err
	}

	keyFile, err = copyKey(workingDir, r.Key)
	if err != nil {
		return "", "", err
	}

	repoDir, err := clone(workingDir, keyFile, r.URL, r.Branch)
	return repoDir, keyFile, err
}

func (r Repo) CommitAndPush(path, keyFile, commitMessage string) (string, error) {
	if !check(path, r.Path) {
		return "no changes made to files", nil
	}
	if err := commit(path, commitMessage); err != nil {
		return "", err
	}
	return "", push(keyFile, r.Branch, path)
}
