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

func config(workingDir, user, email string) error {
	for k, v := range map[string]string{
		"user.name":  user,
		"user.email": email,
	} {
		if err := execGitCmd(workingDir, "", nil, "config", k, v); err != nil {
			return errors.Wrap(err, "setting git config")
		}
	}
	return nil
}

// Do a shallow clone of the repo. We only need the files, and not the
// history. A shallow clone is marginally quicker, and takes less
// space, than a full clone.
func clone(workingDir, keyPath, repoURL, repoBranch string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	// --single-branch is also useful, but is implied by --depth=1
	args := []string{"clone", "--depth=1"}
	if repoBranch != "" {
		args = append(args, "--branch", repoBranch)
	}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(workingDir, keyPath, nil, args...); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func commit(workingDir, commitMessage string) error {
	if err := execGitCmd(
		workingDir, "", nil,
		"commit",
		"--no-verify", "-a", "-m", commitMessage,
	); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

func push(keyPath, repoBranch, workingDir string) error {
	if err := execGitCmd(workingDir, keyPath, nil, "push", "origin", repoBranch); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push origin %s", repoBranch))
	}
	return nil
}

func pull(keyPath, repoBranch, workingDir string) error {
	if err := execGitCmd(workingDir, keyPath, nil, "pull", "--ff-only", "origin", repoBranch); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git pull --ff-only origin %s", repoBranch))
	}
	return nil
}

// Get the commit hash for HEAD
func headRevision(path string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, "", out, "rev-list", "--max-count", "1", "HEAD"); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func revlist(path, ref1, ref2 string) ([]string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, "", out, "rev-list", fmt.Sprintf("%s..%s", ref1, ref2)); err != nil {
		return nil, err
	}
	return strings.Split(out.String(), "\n"), nil
}

// Move the tag to the ref given and push that tag to origin
func moveTagAndPush(path, key, tag, ref, msg string) error {
	if err := execGitCmd(path, "", nil, "tag", "--force", "-a", "-m", msg, tag, ref); err != nil {
		return errors.Wrap(err, "moving tag "+tag)
	}
	if err := execGitCmd(path, key, nil, "push", "origin", "tag", tag); err != nil {
		return errors.Wrap(err, "pushing tag to origin")
	}
	return nil
}

func execGitCmd(dir, keyPath string, out io.Writer, args ...string) error {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(keyPath)
	c.Stdout = ioutil.Discard
	if out != nil {
		c.Stdout = out
	}
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
	return execGitCmd(workingDir, "", nil, "diff", "--quiet", "--", subdir) != nil
}

func narrowKeyPerms(keyPath string) error {
	return os.Chmod(keyPath, 0400)
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
