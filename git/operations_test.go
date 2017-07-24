package git

import (
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"io/ioutil"
	"os/exec"
	"path"
	"testing"
)

func TestChangedFiles_SlashPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := "/test/dir"

	err := createRepo(newDir, nestedDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = changedFiles(newDir, nestedDir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles_UnslashPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := "test/dir"

	err := createRepo(newDir, nestedDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = changedFiles(newDir, nestedDir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles_NoPath(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	nestedDir := ""

	err := createRepo(newDir, nestedDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = changedFiles(newDir, nestedDir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
}

func createRepo(dir string, nestedDir string) error {
	fullPath := path.Join(dir, nestedDir)
	var err error
	if err = execCommand("git", "-C", dir, "init"); err != nil {
		return err
	}
	if err := execCommand("mkdir", "-p", fullPath); err != nil {
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
	return nil
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stderr = ioutil.Discard
	c.Stdout = ioutil.Discard
	return c.Run()
}
