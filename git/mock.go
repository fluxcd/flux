package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Create and init a new mock repo for testing. This is sort of terrible using
// the actual filesystem. We should be using a git library, doing it in-memory,
// and not shelling out to git CLI, *but* kubeservice needs files on the
// filesystem. :(
func NewMockRepo() (string, error) {
	path, err := ioutil.TempDir(os.TempDir(), "flux-testrepo")
	if err != nil {
		return "", err
	}
	if err := gitCmd(path, "", "init").Run(); err != nil {
		os.RemoveAll(path)
		return "", errors.Wrap(err, "git init")
	}
	return path, nil
}

func AddFileToMockRepo(repoDir, file string, data []byte) error {
	if err := ioutil.WriteFile(filepath.Join(repoDir, file), []byte(data), 0600); err != nil {
		return err
	}
	if err := gitCmd(repoDir, "", "add", "-A").Run(); err != nil {
		return errors.Wrap(err, "git add -A")
	}
	return commit(repoDir, fmt.Sprintf("added file: %s", file))
}
