package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

const (
	testNoteRef = "flux-sync"
)

var (
	noteIdCounter = 1
)

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
	for n := range notes {
		note, err := getNote(context.Background(), newDir, testNoteRef, n)
		if err != nil {
			t.Fatal(err)
		}
		if note == nil {
			t.Fatal("note is nil")
		}
		if note.JobID != idHEAD_1 && note.JobID != idHEAD {
			t.Fatal("Note id didn't match expected", note.JobID)
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

func testNote(dir, rev string) (job.ID, error) {
	id := job.ID(fmt.Sprintf("%v", noteIdCounter))
	noteIdCounter += 1
	err := addNote(context.Background(), dir, rev, testNoteRef, &Note{
		id,
		update.Spec{
			update.Auto,
			update.Cause{
				"message",
				"user",
			},
			update.Automated{},
		},
		update.Result{},
	})
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

	_, err = changedFiles(context.Background(), newDir, nestedDir, "HEAD")
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

	_, err = changedFiles(context.Background(), newDir, nestedDir, "HEAD")
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

	_, err = changedFiles(context.Background(), newDir, nestedDir, "HEAD")
	if err != nil {
		t.Fatal(err)
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

	commits, err := onelinelog(context.Background(), newDir, "HEAD~2..HEAD", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 2 {
		t.Fatal(err)
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

	commits, err := onelinelog(context.Background(), newDir, "HEAD~2..HEAD", "dev")
	if err != nil {
		t.Fatal(err)
	}

	if len(commits) != 1 {
		t.Fatal(err)
	}
}

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
