package git

import (
	"bufio"
	"bytes"
	"encoding/json"
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

func clone(workingDir, keyPath, repoURL, repoBranch string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	args := []string{"clone"}
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

// push the refs given to the upstream repo
func push(keyPath, workingDir, upstream string, refs []string) error {
	args := append([]string{"push", upstream}, refs...)
	if err := execGitCmd(workingDir, keyPath, nil, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push %s %s", upstream, refs))
	}
	return nil
}

// pull the specific ref from upstream. Usually this would
func pull(keyPath, workingDir, upstream, ref string) error {
	if err := execGitCmd(workingDir, keyPath, nil, "pull", "--ff-only", upstream, ref); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git pull --ff-only %s %s", upstream, ref))
	}
	return nil
}

func fetch(keyPath, workingDir, upstream, refspec string) error {
	if err := execGitCmd(workingDir, keyPath, nil, "fetch", "--tags", upstream, refspec); err != nil &&
		!strings.Contains(err.Error(), "Couldn't find remote ref") {
		return errors.Wrap(err, fmt.Sprintf("git fetch --tags %s %s", upstream, refspec))
	}
	return nil
}

func refExists(workingDir, ref string) (bool, error) {
	if err := execGitCmd(workingDir, "", nil, "rev-list", ref); err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get the full ref for a shorthand notes ref
func getNotesRef(workingDir, ref string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(workingDir, "", out, "notes", "--ref", ref, "get-ref"); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func addNote(workingDir, rev, notesRef string, note *Note) error {
	b, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return execGitCmd(workingDir, "", nil, "notes", "--ref", notesRef, "add", "-m", string(b), rev)
}

func getNote(workingDir, notesRef, rev string) (*Note, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(workingDir, "", out, "notes", "--ref", notesRef, "show", rev); err != nil {
		return nil, err
	}
	var note Note
	if err := json.NewDecoder(out).Decode(&note); err != nil {
		return nil, err
	}
	return &note, nil
}

// Get the commit hash for a reference
func refRevision(path, ref string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, "", out, "rev-list", "--max-count", "1", ref); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func revlist(path, ref string) ([]string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, "", out, "rev-list", ref); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

func splitList(s string) []string {
	outStr := strings.TrimSpace(s)
	if outStr == "" {
		return []string{}
	}
	return strings.Split(outStr, "\n")
}

// Move the tag to the ref given and push that tag upstream
func moveTagAndPush(path, key, tag, ref, msg, upstream string) error {
	if err := execGitCmd(path, "", nil, "tag", "--force", "-a", "-m", msg, tag, ref); err != nil {
		return errors.Wrap(err, "moving tag "+tag)
	}
	if err := execGitCmd(path, key, nil, "push", "--force", upstream, "tag", tag); err != nil {
		return errors.Wrap(err, "pushing tag to origin")
	}
	return nil
}

func changedFiles(path, subPath, ref string) ([]string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, "", out, "diff", "--name-only", ref, "--", subPath); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

func execGitCmd(dir, keyPath string, out io.Writer, args ...string) error {
	//	println("git", strings.Join(args, " "))
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
	base := `GIT_SSH_COMMAND=ssh -o LogLevel=error`
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
