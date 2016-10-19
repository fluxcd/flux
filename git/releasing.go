package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

func clone(workingDir, keyPath, repoURL, repoBranch string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	if err := gitCmd("", keyPath, "clone", "--branch", repoBranch, repoURL, repoPath).Run(); err != nil {
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

func push(keyPath, repoBranch, workingDir string) error {
	if err := gitCmd(workingDir, keyPath, "push", "origin", repoBranch).Run(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push origin %s", repoBranch))
	}
	return nil
}

func gitCmd(dir, keyPath string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(keyPath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func env(keyPath string) []string {
	base := `GIT_SSH_COMMAND=ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`
	if keyPath == "" {
		return []string{base}
	}
	return []string{fmt.Sprintf("%s -i %q", base, keyPath), "GIT_TERMINAL_PROMPT=0"}
}

// check returns true if there are changes locally.
func check(workingDir, subdir string) bool {
	diff := gitCmd(workingDir, "", "diff", "--quiet", "--", subdir)
	// `--quiet` means "exit with 1 if there are changes"
	return diff.Run() != nil
}

func writeKey(workingDir, keyData string) (string, error) {
	keyPath := filepath.Join(workingDir, "id-rsa")
	err := ioutil.WriteFile(keyPath, []byte(keyData), 0400)
	return keyPath, err
}
