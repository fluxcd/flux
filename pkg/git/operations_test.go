package git

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/stretchr/testify/assert"
)

const (
	testNoteRef = "flux-sync"
)

var (
	noteIdCounter = 1
)

type Note struct {
	ID string
}

func TestListNotes_2Notes(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	err := createRepo(newDir, []string{"another"})
	if err != nil {
		t.Fatal(err)
	}

	idHEAD_1, err := testNote(newDir, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	idHEAD, err := testNote(newDir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	notes, err := noteRevList(context.Background(), newDir, testNoteRef)
	if err != nil {
		t.Fatal(err)
	}

	// Now check that these commits actually have a note
	if len(notes) != 2 {
		t.Fatal("expected two notes")
	}
	for rev := range notes {
		var note Note
		ok, err := getNote(context.Background(), newDir, testNoteRef, rev, &note)
		if err != nil {
			t.Error(err)
		}
		if !ok {
			t.Error("note not found for commit:", rev)
		}
		if note.ID != idHEAD_1 && note.ID != idHEAD {
			t.Error("Note contents not expected:", note.ID)
		}
	}
}

func TestListNotes_0Notes(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	err := createRepo(newDir, []string{"another"})
	if err != nil {
		t.Fatal(err)
	}

	notes, err := noteRevList(context.Background(), newDir, testNoteRef)
	if err != nil {
		t.Fatal(err)
	}

	if len(notes) != 0 {
		t.Fatal("expected two notes")
	}
}

func testNote(dir, rev string) (string, error) {
	id := fmt.Sprintf("%v", noteIdCounter)
	noteIdCounter += 1
	err := addNote(context.Background(), dir, rev, testNoteRef, &Note{ID: id})
	return id, err
}

func TestChangedFiles_SlashPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := "/test/dir"

	err := createRepo(newDir, []string{nestedDir})
	if err != nil {
		t.Fatal(err)
	}

	_, err = changed(context.Background(), newDir, "HEAD", []string{nestedDir})
	if err == nil {
		t.Fatal("Should have errored")
	}
}

func TestChangedFiles_UnslashPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := "test/dir"

	err := createRepo(newDir, []string{nestedDir})
	if err != nil {
		t.Fatal(err)
	}

	_, err = changed(context.Background(), newDir, "HEAD", []string{nestedDir})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles_NoPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := ""

	err := createRepo(newDir, []string{nestedDir})
	if err != nil {
		t.Fatal(err)
	}

	_, err = changed(context.Background(), newDir, "HEAD", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles_LeadingSpace(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	err := createRepo(newDir, []string{})
	if err != nil {
		t.Fatal(err)
	}

	filename := " space.yaml"

	if err = updateDirAndCommit(newDir, "", map[string]string{filename: "foo"}); err != nil {
		t.Fatal(err)
	}

	files, err := changed(context.Background(), newDir, "HEAD~1", []string{})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatal("expected 1 changed file")
	}

	if actualFilename := files[0]; actualFilename != filename {
		t.Fatalf("expected changed filename to equal: '%s', got '%s'", filename, actualFilename)
	}
}

func TestOnelinelog_NoGitpath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	subdirs := []string{"dev", "prod"}
	err := createRepo(newDir, subdirs)
	if err != nil {
		t.Fatal(err)
	}

	if err = updateDirAndCommit(newDir, "dev", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "prod", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}

	commits, err := onelinelog(context.Background(), newDir, "HEAD~2..HEAD", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 2 {
		t.Fatal(err)
	}
}

func TestOnelinelog_NoGitpath_Merged(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	subdirs := []string{"dev", "prod"}
	err := createRepo(newDir, subdirs)
	if err != nil {
		t.Fatal(err)
	}

	branch := "tmp"
	if err = execCommand("git", "-C", newDir, "checkout", "-b", branch); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "dev", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "prod", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", newDir, "checkout", "master"); err != nil {
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", newDir, "merge", "--no-ff", branch); err != nil {
		t.Fatal(err)
	}

	commits, err := onelinelog(context.Background(), newDir, "HEAD", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 6 {
		t.Fatal(commits)
	}

	commits, err = onelinelog(context.Background(), newDir, "HEAD", nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 4 {
		t.Fatal(commits)
	}
}

func TestOnelinelog_WithGitpath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	subdirs := []string{"dev", "prod"}

	err := createRepo(newDir, subdirs)
	if err != nil {
		t.Fatal(err)
	}

	if err = updateDirAndCommit(newDir, "dev", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "prod", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}

	commits, err := onelinelog(context.Background(), newDir, "HEAD~2..HEAD", []string{"dev"}, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 1 {
		t.Fatal(err)
	}
}

func TestOnelinelog_WithGitpath_Merged(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	subdirs := []string{"dev", "prod"}
	err := createRepo(newDir, subdirs)
	if err != nil {
		t.Fatal(err)
	}

	branch := "tmp"
	if err = execCommand("git", "-C", newDir, "checkout", "-b", branch); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "dev", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = updateDirAndCommit(newDir, "prod", testfiles.FilesUpdated); err != nil {
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", newDir, "checkout", "master"); err != nil {
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", newDir, "merge", "--no-ff", branch); err != nil {
		t.Fatal(err)
	}

	// show the 2 update commits as well as init commit
	commits, err := onelinelog(context.Background(), newDir, "HEAD", []string{"prod"}, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 2 {
		t.Fatal(err)
	}

	// show the merge commit as well as init commit
	commits, err = onelinelog(context.Background(), newDir, "HEAD", []string{"prod"}, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 2 {
		t.Fatal(err)
	}
}

func TestCheckPush(t *testing.T) {
	upstreamDir, upstreamCleanup := testfiles.TempDir(t)
	defer upstreamCleanup()
	if err := createRepo(upstreamDir, []string{"config"}); err != nil {
		t.Fatal(err)
	}

	cloneDir, cloneCleanup := testfiles.TempDir(t)
	defer cloneCleanup()

	working, err := clone(context.Background(), cloneDir, upstreamDir, "master")
	if err != nil {
		t.Fatal(err)
	}
	err = checkPush(context.Background(), working, upstreamDir, "")
	if err != nil {
		t.Fatal(err)
	}
}

// ---

func createRepo(dir string, subdirs []string) error {
	var (
		err      error
		fullPath string
	)

	if err = execCommand("git", "-C", dir, "init"); err != nil {
		return err
	}
	if err := config(context.Background(), dir, "operations_test_user", "example@example.com"); err != nil {
		return err
	}

	for _, subdir := range subdirs {
		fullPath = path.Join(dir, subdir)
		if err = execCommand("mkdir", "-p", fullPath); err != nil {
			return err
		}

		if err = testfiles.WriteTestFiles(fullPath); err != nil {
			return err
		}
		if err = execCommand("git", "-C", dir, "add", "--all"); err != nil {
			return err
		}
		if err = execCommand("git", "-C", dir, "commit", "-m", "'Initial revision'"); err != nil {
			return err
		}
	}
	if err = execCommand("git", "-C", dir, "commit", "--allow-empty", "-m", "'Second revision'"); err != nil {
		return err
	}

	return nil
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stderr = ioutil.Discard
	c.Stdout = ioutil.Discard
	return c.Run()
}

// Replaces/creates a file
func updateFile(path string, files map[string]string) error {
	for file, content := range files {
		path := filepath.Join(path, file)
		if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
			return err
		}
	}
	return nil
}

func updateDirAndCommit(dir, subdir string, filesUpdated map[string]string) error {
	path := filepath.Join(dir, subdir)
	if err := updateFile(path, filesUpdated); err != nil {
		return err
	}
	if err := execCommand("git", "-C", path, "add", "--all"); err != nil {
		return err
	}
	if err := execCommand("git", "-C", path, "commit", "-m", "'Update 1'"); err != nil {
		return err
	}
	return nil
}

func TestTraceGitCommand(t *testing.T) {
	type input struct {
		args   []string
		config gitCmdConfig
		out    string
		err    string
	}
	examples := []struct {
		name     string
		input    input
		expected string
		actual   string
	}{
		{
			name: "git clone",
			input: input{
				args: []string{
					"clone",
					"--branch",
					"master",
					"/tmp/flux-gitclone239583443",
					"/tmp/flux-working628880789",
				},
				config: gitCmdConfig{
					dir: "/tmp/flux-working628880789",
				},
			},
			expected: `TRACE: command="git clone --branch master /tmp/flux-gitclone239583443 /tmp/flux-working628880789" out="" dir="/tmp/flux-working628880789" env=""`,
		},
		{
			name: "git rev-list",
			input: input{
				args: []string{
					"rev-list",
					"--max-count",
					"1",
					"flux-sync",
					"--",
				},
				out: "b9d6a543acf8085ff6bed23fac17f8dc71bfcb66",
				config: gitCmdConfig{
					dir: "/tmp/flux-gitclone239583443",
				},
			},
			expected: `TRACE: command="git rev-list --max-count 1 flux-sync --" out="b9d6a543acf8085ff6bed23fac17f8dc71bfcb66" dir="/tmp/flux-gitclone239583443" env=""`,
		},
		{
			name: "git config email",
			input: input{
				args: []string{
					"config",
					"user.email",
					"support@weave.works",
				},
				config: gitCmdConfig{
					dir: "/tmp/flux-working056923691",
				},
			},
			expected: `TRACE: command="git config user.email support@weave.works" out="" dir="/tmp/flux-working056923691" env=""`,
		},
		{
			name: "git notes",
			input: input{
				args: []string{
					"notes",
					"--ref",
					"flux",
					"get-ref",
				},
				config: gitCmdConfig{
					dir: "/tmp/flux-working647148942",
				},
				out: "refs/notes/flux",
			},
			expected: `TRACE: command="git notes --ref flux get-ref" out="refs/notes/flux" dir="/tmp/flux-working647148942" env=""`,
		},
	}
	for _, example := range examples {
		actual := traceGitCommand(
			example.input.args,
			example.input.config,
			example.input.out,
		)
		assert.Equal(t, example.expected, actual)
	}
}

// TestMutexBuffer tests that the threadsafe buffer used to capture
// stdout and stderr does not give rise to races or deadlocks. In
// particular, this test guards against reverting to a situation in
// which copying into the buffer from two goroutines can deadlock it,
// if one of them uses `ReadFrom`.
func TestMutexBuffer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out := &bytes.Buffer{}
	err := execGitCmd(ctx, []string{"log", "--oneline"}, gitCmdConfig{out: out})
	if err != nil {
		t.Fatal(err)
	}
}
