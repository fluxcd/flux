package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

func clone(workingDir, repoKey, repoURL string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	if err := gitCmd("", repoKey, "clone", repoURL, repoPath).Run(); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func commit(workingDir, commitMessage string) error {
	if err := gitCmd(
		workingDir, "",
		"-c", "user.name=Weave Flux", "-c", "user.email=support@weave.works",
		"commit",
		"--no-verify", "-a", "-m", commitMessage,
	).Run(); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

func push(repoKey, workingDir string) error {
	if err := gitCmd(workingDir, repoKey, "push", "origin", "master").Run(); err != nil {
		return errors.Wrap(err, "git push origin master")
	}
	return nil
}

func gitCmd(dir, repoKey string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(repoKey)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func env(repoKey string) []string {
	base := `GIT_SSH_COMMAND=ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`
	if repoKey == "" {
		return []string{base}
	}
	return []string{fmt.Sprintf("%s -i %q", base, repoKey), "GIT_TERMINAL_PROMPT=0"}
}

// check returns true if there are changes locally.
func check(workingDir, subdir string) bool {
	diff := gitCmd(workingDir, "", "diff", "--quiet", "--", subdir)
	// `--quiet` means "exit with 1 if there are changes"
	return diff.Run() != nil
}

func copyKey(working, key string) (string, error) {
	keyPath := filepath.Join(working, "id-rsa")
	f, err := ioutil.ReadFile(key)
	if err == nil {
		err = ioutil.WriteFile(keyPath, f, 0400)
	}
	return keyPath, err
}
