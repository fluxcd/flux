package git

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/weaveworks/flux/ssh"
)

func config(workingDir, user, email string) error {
	for k, v := range map[string]string{
		"user.name":  user,
		"user.email": email,
	} {
		if err := execGitCmd(workingDir, nil, nil, "config", k, v); err != nil {
			return errors.Wrap(err, "setting git config")
		}
	}
	return nil
}

func clone(workingDir string, keyRing ssh.KeyRing, repoURL, repoBranch string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	args := []string{"clone"}
	if repoBranch != "" {
		args = append(args, "--branch", repoBranch)
	}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(workingDir, keyRing, nil, args...); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func commit(workingDir, commitMessage string) error {
	if err := execGitCmd(
		workingDir, nil, nil,
		"commit",
		"--no-verify", "-a", "-m", commitMessage,
	); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

// push the refs given to the upstream repo
func push(keyRing ssh.KeyRing, workingDir, upstream string, refs []string) error {
	args := append([]string{"push", upstream}, refs...)
	if err := execGitCmd(workingDir, keyRing, nil, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push %s %s", upstream, refs))
	}
	return nil
}

// pull the specific ref from upstream
func pull(keyRing ssh.KeyRing, workingDir, upstream, ref string) error {
	if err := execGitCmd(workingDir, keyRing, nil, "pull", "--ff-only", upstream, ref); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git pull --ff-only %s %s", upstream, ref))
	}
	return nil
}

func fetch(keyRing ssh.KeyRing, workingDir, upstream, refspec string) error {
	if err := execGitCmd(workingDir, keyRing, nil, "fetch", "--tags", upstream, refspec); err != nil &&
		!strings.Contains(err.Error(), "Couldn't find remote ref") {
		return errors.Wrap(err, fmt.Sprintf("git fetch --tags %s %s", upstream, refspec))
	}
	return nil
}

func refExists(workingDir, ref string) (bool, error) {
	if err := execGitCmd(workingDir, nil, nil, "rev-list", ref); err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get the full ref for a shorthand notes ref.
func getNotesRef(workingDir, ref string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(workingDir, nil, out, "notes", "--ref", ref, "get-ref"); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func addNote(workingDir, rev, notesRef string, note *Note) error {
	b, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return execGitCmd(workingDir, nil, nil, "notes", "--ref", notesRef, "add", "-m", string(b), rev)
}

// NB return values (*Note, nil), (nil, error), (nil, nil)
func getNote(workingDir, notesRef, rev string) (*Note, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(workingDir, nil, out, "notes", "--ref", notesRef, "show", rev); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no note found for object") {
			return nil, nil
		}
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
	if err := execGitCmd(path, nil, out, "rev-list", "--max-count", "1", ref); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func revlist(path, ref string) ([]string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, nil, out, "rev-list", ref); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

// Return the revisions and one-line log commit messages
func onelinelog(path, refspec string) ([]Commit, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(path, nil, out, "log", "--oneline", "--no-abbrev-commit", refspec); err != nil {
		return nil, err
	}
	return splitLog(out.String())
}

func splitLog(s string) ([]Commit, error) {
	lines := splitList(s)
	commits := make([]Commit, len(lines))
	for i, m := range lines {
		revAndMessage := strings.SplitN(m, " ", 2)
		commits[i].Revision = revAndMessage[0]
		commits[i].Message = revAndMessage[1]
	}
	return commits, nil
}

func splitList(s string) []string {
	outStr := strings.TrimSpace(s)
	if outStr == "" {
		return []string{}
	}
	return strings.Split(outStr, "\n")
}

// Move the tag to the ref given and push that tag upstream
func moveTagAndPush(path string, keyRing ssh.KeyRing, tag, ref, msg, upstream string) error {
	if err := execGitCmd(path, nil, nil, "tag", "--force", "-a", "-m", msg, tag, ref); err != nil {
		return errors.Wrap(err, "moving tag "+tag)
	}
	if err := execGitCmd(path, keyRing, nil, "push", "--force", upstream, "tag", tag); err != nil {
		return errors.Wrap(err, "pushing tag to origin")
	}
	return nil
}

func changedFiles(path, subPath, ref string) ([]string, error) {
	// Remove leading slash if present. diff doesn't work when using github style root paths.
	if len(subPath) > 0 && subPath[0] == '/' {
		return []string{}, errors.New("git subdirectory should not have leading forward slash")
	}
	out := &bytes.Buffer{}
	// This uses --diff-filter to only look at changes for file _in
	// the working dir_; i.e, we do not report on things that no
	// longer appear.
	if err := execGitCmd(path, nil, out, "diff", "--name-only", "--diff-filter=ACMRT", ref, "--", subPath); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

func execGitCmd(dir string, keyRing ssh.KeyRing, out io.Writer, args ...string) error {
	//	println("git", strings.Join(args, " "))
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(keyRing)
	c.Stdout = ioutil.Discard
	if out != nil {
		c.Stdout = out
	}
	errOut := &bytes.Buffer{}
	c.Stderr = errOut
	err := c.Run()
	if err != nil {
		msg := findErrorMessage(errOut)
		if msg != "" {
			err = errors.New(msg)
		}
	}
	return err
}

func env(keyRing ssh.KeyRing) []string {
	base := `GIT_SSH_COMMAND=ssh -o LogLevel=error`
	if keyRing == nil {
		return []string{base}
	}
	_, privateKeyPath := keyRing.KeyPair()
	return []string{fmt.Sprintf("%s -i %q", base, privateKeyPath), "GIT_TERMINAL_PROMPT=0"}
}

// check returns true if there are changes locally.
func check(workingDir, subdir string) bool {
	// `--quiet` means "exit with 1 if there are changes"
	return execGitCmd(workingDir, nil, nil, "diff", "--quiet", "--", subdir) != nil
}

func findErrorMessage(output io.Reader) string {
	sc := bufio.NewScanner(output)
	for sc.Scan() {
		switch {
		case strings.HasPrefix(sc.Text(), "fatal: "):
			return sc.Text()
		case strings.HasPrefix(sc.Text(), "error:"):
			return strings.Trim(sc.Text(), "error: ")
		}
	}
	return ""
}
