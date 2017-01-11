package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

func clone(stderr io.Writer, workingDir, keyData, repoURL, repoBranch string) (path string, err error) {
	keyPath, err := writeKey(keyData)
	if err != nil {
		return "", err
	}
	defer os.Remove(keyPath)
	repoPath := filepath.Join(workingDir, "repo")
	args := []string{"clone"}
	if repoBranch != "" {
		args = append(args, "--branch", repoBranch)
	}
	args = append(args, repoURL, repoPath)
	if err := gitCmd(stderr, workingDir, keyPath, args...).Run(); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func commit(workingDir, commitMessage string) error {
	if err := gitCmd(
		nil, workingDir, "",
		"-c", "user.name=Weave Flux", "-c", "user.email=support@weave.works",
		"commit",
		"--no-verify", "-a", "-m", commitMessage,
	).Run(); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

func push(keyData, repoBranch, workingDir string) error {
	keyPath, err := writeKey(keyData)
	if err != nil {
		return err
	}
	defer os.Remove(keyPath)
	if err := gitCmd(nil, workingDir, keyPath, "push", "origin", repoBranch).Run(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push origin %s", repoBranch))
	}
	return nil
}

func gitCmd(stderr io.Writer, dir, keyPath string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(keyPath)
	c.Stdout = ioutil.Discard
	c.Stderr = ioutil.Discard
	if stderr != nil {
		c.Stderr = stderr
	}
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
	diff := gitCmd(nil, workingDir, "", "diff", "--quiet", "--", subdir)
	// `--quiet` means "exit with 1 if there are changes"
	return diff.Run() != nil
}

func writeKey(keyData string) (string, error) {
	f, err := ioutil.TempFile("", "flux-key")
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := ioutil.WriteFile(f.Name(), []byte(keyData), 0400); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
