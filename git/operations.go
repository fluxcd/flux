package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Do a shallow clone of the repo. We only need the files, and not the
// history. A shallow clone is marginally quicker, and takes less
// space, than a full clone.
func clone(workingDir, keyData, repoURL, repoBranch string) (path string, err error) {
	keyPath, err := writeKey(keyData)
	if err != nil {
		return "", err
	}
	defer os.Remove(keyPath)
	repoPath := filepath.Join(workingDir, "repo")
	// --single-branch is also useful, but is implied by --depth=1
	args := []string{"clone", "--depth=1"}
	if repoBranch != "" {
		args = append(args, "--branch", repoBranch)
	}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(workingDir, keyPath, args...); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func commit(workingDir, commitMessage string) error {
	if err := execGitCmd(
		workingDir, "",
		"-c", "user.name=Weave Flux", "-c", "user.email=support@weave.works",
		"commit",
		"--no-verify", "-a", "-m", commitMessage,
	); err != nil {
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
	if err := execGitCmd(workingDir, keyPath, "push", "origin", repoBranch); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push origin %s", repoBranch))
	}
	return nil
}

func execGitCmd(dir, keyPath string, args ...string) error {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(keyPath)
	c.Stdout = ioutil.Discard
	errOut := &bytes.Buffer{}
	c.Stderr = errOut
	err := c.Run()
	if err != nil {
		msg := findFatalMessage(errOut)
		if msg != "" {
			err = errors.New(msg)
		}
	}
	return err
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
	// `--quiet` means "exit with 1 if there are changes"
	return execGitCmd(workingDir, "", "diff", "--quiet", "--", subdir) != nil
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

func findFatalMessage(output io.Reader) string {
	sc := bufio.NewScanner(output)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "fatal:") {
			return sc.Text()
		}
	}
	return ""
}
