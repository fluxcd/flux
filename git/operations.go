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
	"strings"

	"context"

	"github.com/pkg/errors"
)

// If true, every git invocation will be echoed to stdout
const trace = false

// Env vars that are allowed to be inherited from the os
var allowedEnvVars = []string{"http_proxy", "https_proxy", "no_proxy", "HOME"}

func config(ctx context.Context, workingDir, user, email string) error {
	for k, v := range map[string]string{
		"user.name":  user,
		"user.email": email,
	} {
		if err := execGitCmd(ctx, workingDir, nil, "config", k, v); err != nil {
			return errors.Wrap(err, "setting git config")
		}
	}
	return nil
}

func clone(ctx context.Context, workingDir, repoURL, repoBranch string) (path string, err error) {
	repoPath := workingDir
	args := []string{"clone"}
	if repoBranch != "" {
		args = append(args, "--branch", repoBranch)
	}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(ctx, workingDir, nil, args...); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func mirror(ctx context.Context, workingDir, repoURL string) (path string, err error) {
	repoPath := workingDir
	args := []string{"clone", "--mirror"}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(ctx, workingDir, nil, args...); err != nil {
		return "", errors.Wrap(err, "git clone --mirror")
	}
	return repoPath, nil
}

func checkout(ctx context.Context, workingDir, ref string) error {
	return execGitCmd(ctx, workingDir, nil, "checkout", ref)
}

// checkPush sanity-checks that we can write to the upstream repo
// (being able to `clone` is an adequate check that we can read the
// upstream).
func checkPush(ctx context.Context, workingDir, upstream string) error {
	// --force just in case we fetched the tag from upstream when cloning
	if err := execGitCmd(ctx, workingDir, nil, "tag", "--force", CheckPushTag); err != nil {
		return errors.Wrap(err, "tag for write check")
	}
	if err := execGitCmd(ctx, workingDir, nil, "push", "--force", upstream, "tag", CheckPushTag); err != nil {
		return errors.Wrap(err, "attempt to push tag")
	}
	return execGitCmd(ctx, workingDir, nil, "push", "--delete", upstream, "tag", CheckPushTag)
}

func commit(ctx context.Context, workingDir string, commitAction CommitAction) error {
	commitAuthor := commitAction.Author
	if commitAuthor != "" {
		if err := execGitCmd(ctx,
			workingDir, nil,
			"commit",
			"--no-verify", "-a", "--author", commitAuthor, "-m", commitAction.Message,
		); err != nil {
			return errors.Wrap(err, "git commit")
		}
		return nil
	}
	if err := execGitCmd(ctx,
		workingDir, nil,
		"commit",
		"--no-verify", "-a", "-m", commitAction.Message,
	); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

// push the refs given to the upstream repo
func push(ctx context.Context, workingDir, upstream string, refs []string) error {
	args := append([]string{"push", upstream}, refs...)
	if err := execGitCmd(ctx, workingDir, nil, args...); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push %s %s", upstream, refs))
	}
	return nil
}

// fetch updates refs from the upstream.
func fetch(ctx context.Context, workingDir, upstream string, refspec ...string) error {
	args := append([]string{"fetch", "--tags", upstream}, refspec...)
	if err := execGitCmd(ctx, workingDir, nil, args...); err != nil &&
		!strings.Contains(err.Error(), "Couldn't find remote ref") {
		return errors.Wrap(err, fmt.Sprintf("git fetch --tags %s %s", upstream, refspec))
	}
	return nil
}

func refExists(ctx context.Context, workingDir, ref string) (bool, error) {
	if err := execGitCmd(ctx, workingDir, nil, "rev-list", ref); err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get the full ref for a shorthand notes ref.
func getNotesRef(ctx context.Context, workingDir, ref string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(ctx, workingDir, out, "notes", "--ref", ref, "get-ref"); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func addNote(ctx context.Context, workingDir, rev, notesRef string, note interface{}) error {
	b, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return execGitCmd(ctx, workingDir, nil, "notes", "--ref", notesRef, "add", "-m", string(b), rev)
}

func getNote(ctx context.Context, workingDir, notesRef, rev string, note interface{}) (ok bool, err error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(ctx, workingDir, out, "notes", "--ref", notesRef, "show", rev); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no note found for object") {
			return false, nil
		}
		return false, err
	}
	if err := json.NewDecoder(out).Decode(note); err != nil {
		return false, err
	}
	return true, nil
}

// Get all revisions with a note (NB: DO NOT RELY ON THE ORDERING)
// It appears to be ordered by ascending git object ref, not by time.
// Return a map to make it easier to do "if in" type queries.
func noteRevList(ctx context.Context, workingDir, notesRef string) (map[string]struct{}, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(ctx, workingDir, out, "notes", "--ref", notesRef, "list"); err != nil {
		return nil, err
	}
	noteList := splitList(out.String())
	result := make(map[string]struct{}, len(noteList))
	for _, l := range noteList {
		split := strings.Fields(l)
		if len(split) > 0 {
			result[split[1]] = struct{}{} // First field contains the object ref (commit id in our case)
		}
	}
	return result, nil
}

// Get the commit hash for a reference
func refRevision(ctx context.Context, path, ref string) (string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(ctx, path, out, "rev-list", "--max-count", "1", ref); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func revlist(ctx context.Context, path, ref string) ([]string, error) {
	out := &bytes.Buffer{}
	if err := execGitCmd(ctx, path, out, "rev-list", ref); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

// Return the revisions and one-line log commit messages
func onelinelog(ctx context.Context, path, refspec string, subdirs []string) ([]Commit, error) {
	out := &bytes.Buffer{}
	args := []string{"log", "--oneline", "--no-abbrev-commit", refspec}
	if len(subdirs) > 0 {
		args = append(args, "--")
		args = append(args, subdirs...)
	}

	if err := execGitCmd(ctx, path, out, args...); err != nil {
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
func moveTagAndPush(ctx context.Context, path string, tag, ref, msg, upstream string) error {
	if err := execGitCmd(ctx, path, nil, "tag", "--force", "-a", "-m", msg, tag, ref); err != nil {
		return errors.Wrap(err, "moving tag "+tag)
	}
	if err := execGitCmd(ctx, path, nil, "push", "--force", upstream, "tag", tag); err != nil {
		return errors.Wrap(err, "pushing tag to origin")
	}
	return nil
}

func changed(ctx context.Context, path, ref string, subPaths []string) ([]string, error) {
	out := &bytes.Buffer{}
	// This uses --diff-filter to only look at changes for file _in
	// the working dir_; i.e, we do not report on things that no
	// longer appear.
	args := []string{"diff", "--name-only", "--diff-filter=ACMRT", ref}
	if len(subPaths) > 0 {
		args = append(args, "--")
		args = append(args, subPaths...)
	}

	if err := execGitCmd(ctx, path, out, args...); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

func execGitCmd(ctx context.Context, dir string, out io.Writer, args ...string) error {
	if trace {
		print("TRACE: git")
		for _, arg := range args {
			print(` "`, arg, `"`)
		}
		println()
	}
	c := exec.CommandContext(ctx, "git", args...)

	if dir != "" {
		c.Dir = dir
	}
	c.Env = env()
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
	if ctx.Err() == context.DeadlineExceeded {
		return errors.Wrap(ctx.Err(), fmt.Sprintf("running git command: %s %v", "git", args))
	} else if ctx.Err() == context.Canceled {
		return errors.Wrap(ctx.Err(), fmt.Sprintf("context was unexpectedly cancelled when running git command: %s %v", "git", args))
	}
	return err
}

func env() []string {
	env := []string{"GIT_TERMINAL_PROMPT=0"}

	// include allowed env vars from os
	for _, k := range allowedEnvVars {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}

	return env
}

// check returns true if there are changes locally.
func check(ctx context.Context, workingDir string, subdirs []string) bool {
	// `--quiet` means "exit with 1 if there are changes"
	args := []string{"diff", "--quiet"}
	if len(subdirs) > 0 {
		args = append(args, "--")
		args = append(args, subdirs...)
	}
	return execGitCmd(ctx, workingDir, nil, args...) != nil
}

func findErrorMessage(output io.Reader) string {
	sc := bufio.NewScanner(output)
	for sc.Scan() {
		switch {
		case strings.HasPrefix(sc.Text(), "fatal: "):
			return sc.Text()
		case strings.HasPrefix(sc.Text(), "ERROR fatal: "): // Saw this error on ubuntu systems
			return sc.Text()
		case strings.HasPrefix(sc.Text(), "error:"):
			return strings.Trim(sc.Text(), "error: ")
		}
	}
	return ""
}
