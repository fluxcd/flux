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

// If true, every git invocation will be echoed to stdout (with the exception of those added to `exemptedTraceCommands`)
const trace = false

// Whilst debugging or developing, you may wish to filter certain git commands out of the logs when tracing is on.
var exemptedTraceCommands = []string{
	// To filter out a certain git subcommand add it here, e.g.:
	// "config",
}

// Env vars that are allowed to be inherited from the os
var allowedEnvVars = []string{"http_proxy", "https_proxy", "no_proxy", "HOME", "GNUPGHOME"}

type gitCmdConfig struct {
	dir string
	env []string
	out io.Writer
}

func config(ctx context.Context, workingDir, user, email string) error {
	for k, v := range map[string]string{
		"user.name":  user,
		"user.email": email,
	} {
		args := []string{"config", k, v}
		if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
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
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return "", errors.Wrap(err, "git clone")
	}
	return repoPath, nil
}

func mirror(ctx context.Context, workingDir, repoURL string) (path string, err error) {
	repoPath := workingDir
	args := []string{"clone", "--mirror"}
	args = append(args, repoURL, repoPath)
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return "", errors.Wrap(err, "git clone --mirror")
	}
	return repoPath, nil
}

func checkout(ctx context.Context, workingDir, ref string) error {
	args := []string{"checkout", ref, "--"}
	return execGitCmd(ctx, args, gitCmdConfig{dir: workingDir})
}

// checkPush sanity-checks that we can write to the upstream repo
// (being able to `clone` is an adequate check that we can read the
// upstream).
func checkPush(ctx context.Context, workingDir, upstream, branch string) error {
	// --force just in case we fetched the tag from upstream when cloning
	args := []string{"tag", "--force", CheckPushTag}
	if branch != "" {
		args = append(args, branch)
	}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return errors.Wrap(err, "tag for write check")
	}
	args = []string{"push", "--force", upstream, "tag", CheckPushTag}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return errors.Wrap(err, "attempt to push tag")
	}
	args = []string{"push", "--delete", upstream, "tag", CheckPushTag}
	return execGitCmd(ctx, args, gitCmdConfig{dir: workingDir})
}

func commit(ctx context.Context, workingDir string, commitAction CommitAction) error {
	args := []string{"commit", "--no-verify", "-a", "-m", commitAction.Message}
	var env []string
	if commitAction.Author != "" {
		args = append(args, "--author", commitAction.Author)
	}
	if commitAction.SigningKey != "" {
		args = append(args, fmt.Sprintf("--gpg-sign=%s", commitAction.SigningKey))
	}
	args = append(args, "--")
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, env: env}); err != nil {
		return errors.Wrap(err, "git commit")
	}
	return nil
}

// push the refs given to the upstream repo
func push(ctx context.Context, workingDir, upstream string, refs []string) error {
	args := append([]string{"push", upstream}, refs...)
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("git push %s %s", upstream, refs))
	}
	return nil
}

// fetch updates refs from the upstream.
func fetch(ctx context.Context, workingDir, upstream string, refspec ...string) error {
	args := append([]string{"fetch", "--tags", upstream}, refspec...)
	// In git <=2.20 the error started with an uppercase, in 2.21 this
	// was changed to be consistent with all other die() and error()
	// messages, cast to lowercase to support both versions.
	// Ref: https://github.com/git/git/commit/0b9c3afdbfb62936337efc52b4007a446939b96b
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil &&
		!strings.Contains(strings.ToLower(err.Error()), "couldn't find remote ref") {
		return errors.Wrap(err, fmt.Sprintf("git fetch --tags %s %s", upstream, refspec))
	}
	return nil
}

func refExists(ctx context.Context, workingDir, ref string) (bool, error) {
	args := []string{"rev-list", ref, "--"}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		if strings.Contains(err.Error(), "bad revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get the full ref for a shorthand notes ref.
func getNotesRef(ctx context.Context, workingDir, ref string) (string, error) {
	out := &bytes.Buffer{}
	args := []string{"notes", "--ref", ref, "get-ref"}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func addNote(ctx context.Context, workingDir, rev, notesRef string, note interface{}) error {
	b, err := json.Marshal(note)
	if err != nil {
		return err
	}
	args := []string{"notes", "--ref", notesRef, "add", "-m", string(b), rev}
	return execGitCmd(ctx, args, gitCmdConfig{dir: workingDir})
}

func getNote(ctx context.Context, workingDir, notesRef, rev string, note interface{}) (ok bool, err error) {
	out := &bytes.Buffer{}
	args := []string{"notes", "--ref", notesRef, "show", rev}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
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
	args := []string{"notes", "--ref", notesRef, "list"}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
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
func refRevision(ctx context.Context, workingDir, ref string) (string, error) {
	out := &bytes.Buffer{}
	args := []string{"rev-list", "--max-count", "1", ref, "--"}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// Return the revisions and one-line log commit messages
func onelinelog(ctx context.Context, workingDir, refspec string, subdirs []string) ([]Commit, error) {
	out := &bytes.Buffer{}
	args := []string{"log", "--pretty=format:%GK|%G?|%H|%s", refspec}
	args = append(args, "--")
	if len(subdirs) > 0 {
		args = append(args, subdirs...)
	}

	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
		return nil, err
	}

	return splitLog(out.String())
}

func splitLog(s string) ([]Commit, error) {
	lines := splitList(s)
	commits := make([]Commit, len(lines))
	for i, m := range lines {
		parts := strings.SplitN(m, "|", 4)
		commits[i].Signature = Signature{
			Key:    parts[0],
			Status: parts[1],
		}
		commits[i].Revision = parts[2]
		commits[i].Message = parts[3]
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
func moveTagAndPush(ctx context.Context, workingDir, tag, upstream string, tagAction TagAction) error {
	args := []string{"tag", "--force", "-a", "-m", tagAction.Message}
	var env []string
	if tagAction.SigningKey != "" {
		args = append(args, fmt.Sprintf("--local-user=%s", tagAction.SigningKey))
	}
	args = append(args, tag, tagAction.Revision)
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, env: env}); err != nil {
		return errors.Wrap(err, "moving tag "+tag)
	}
	args = []string{"push", "--force", upstream, "tag", tag}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return errors.Wrap(err, "pushing tag to origin")
	}
	return nil
}

// Verify tag signature and return the revision it points to
func verifyTag(ctx context.Context, workingDir, tag string) (string, error) {
	out := &bytes.Buffer{}
	args := []string{"verify-tag", "--format", "%(object)", tag}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
		return "", errors.Wrap(err, "verifying tag "+tag)
	}
	return strings.TrimSpace(out.String()), nil
}

// Verify commit signature
func verifyCommit(ctx context.Context, workingDir, commit string) error {
	args := []string{"verify-commit", commit}
	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}); err != nil {
		return fmt.Errorf("failed to verify commit %s", commit)
	}
	return nil
}

func changed(ctx context.Context, workingDir, ref string, subPaths []string) ([]string, error) {
	out := &bytes.Buffer{}
	// This uses --diff-filter to only look at changes for file _in
	// the working dir_; i.e, we do not report on things that no
	// longer appear.
	args := []string{"diff", "--name-only", "--diff-filter=ACMRT", ref}
	args = append(args, "--")
	if len(subPaths) > 0 {
		args = append(args, subPaths...)
	}

	if err := execGitCmd(ctx, args, gitCmdConfig{dir: workingDir, out: out}); err != nil {
		return nil, err
	}
	return splitList(out.String()), nil
}

// traceGitCommand returns a log line that can be useful when debugging and developing git activity
func traceGitCommand(args []string, config gitCmdConfig, stdout string, stderr string) string {
	for _, exemptedCommand := range exemptedTraceCommands {
		if exemptedCommand == args[0] {
			return ""
		}
	}

	prepare := func(input string) string {
		output := strings.Trim(input, "\x00")
		output = strings.TrimSuffix(output, "\n")
		output = strings.Replace(output, "\n", "\\n", -1)
		return output
	}

	command := `git ` + strings.Join(args, " ")
	out := prepare(stdout)
	err := prepare(stderr)

	return fmt.Sprintf(
		"TRACE: command=%q out=%q err=%q dir=%q env=%q",
		command,
		out,
		err,
		config.dir,
		strings.Join(config.env, ","),
	)
}

// execGitCmd runs a `git` command with the supplied arguments.
func execGitCmd(ctx context.Context, args []string, config gitCmdConfig) error {
	c := exec.CommandContext(ctx, "git", args...)

	if config.dir != "" {
		c.Dir = config.dir
	}
	c.Env = append(env(), config.env...)
	c.Stdout = ioutil.Discard
	if config.out != nil {
		c.Stdout = config.out
	}
	errOut := &bytes.Buffer{}
	c.Stderr = errOut

	traceStdout := &bytes.Buffer{}
	traceStderr := &bytes.Buffer{}
	if trace {
		c.Stdout = io.MultiWriter(c.Stdout, traceStdout)
		c.Stderr = io.MultiWriter(c.Stderr, traceStderr)
	}

	err := c.Run()
	if err != nil {
		msg := findErrorMessage(errOut)
		if msg != "" {
			err = errors.New(msg)
		}
	}

	if trace {
		if traceCommand := traceGitCommand(args, config, traceStdout.String(), traceStderr.String()); traceCommand != "" {
			println(traceCommand)
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
	args = append(args, "--")
	if len(subdirs) > 0 {
		args = append(args, subdirs...)
	}
	return execGitCmd(ctx, args, gitCmdConfig{dir: workingDir}) != nil
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
