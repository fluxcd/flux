package gittest

import (
	"context"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/resource"
)

// Repo creates a new clone-able git repo, pre-populated with some kubernetes
// files and a few commits. Also returns a cleanup func to clean up after.
func Repo(t *testing.T) (*git.Repo, func()) {
	newDir, cleanup := testfiles.TempDir(t)

	filesDir := filepath.Join(newDir, "files")
	gitDir := filepath.Join(newDir, "git")
	if err := execCommand("mkdir", filesDir); err != nil {
		t.Fatal(err)
	}

	var err error
	if err = execCommand("git", "-C", filesDir, "init"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", filesDir, "config", "--local", "user.email", "example@example.com"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", filesDir, "config", "--local", "user.name", "example"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err = testfiles.WriteTestFiles(filesDir); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", filesDir, "add", "--all"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err = execCommand("git", "-C", filesDir, "commit", "-m", "'Initial revision'"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	if err = execCommand("git", "clone", "--bare", filesDir, gitDir); err != nil {
		t.Fatal(err)
	}

	mirror := git.NewRepo(git.Remote{
		URL: "file://" + gitDir,
	}, git.Branch("master"))
	return mirror, func() {
		mirror.Clean()
		cleanup()
	}
}

// Workloads is a shortcut to getting the names of the workloads (NB
// not all resources, just the workloads) represented in the test
// files.
func Workloads() (res []resource.ID) {
	for k, _ := range testfiles.WorkloadMap("") {
		res = append(res, k)
	}
	return res
}

// CheckoutWithConfig makes a standard repo, clones it, and returns
// the clone, the original repo, and a cleanup function.
func CheckoutWithConfig(t *testing.T, config git.Config, syncTag string) (*git.Checkout, *git.Repo, func()) {
	// Add files to the repo with the same name as the git branch and the sync tag.
	// This is to make sure that git commands don't have ambiguity problems between revisions and files.
	testfiles.Files[config.Branch] = "Filename doctored to create a conflict with the git branch name"
	testfiles.Files[syncTag] = "Filename doctored to create a conflict with the git sync tag"
	repo, cleanup := Repo(t)
	if err := repo.Ready(context.Background()); err != nil {
		cleanup()
		t.Fatal(err)
	}

	co, err := repo.Clone(context.Background(), config)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	return co, repo, func() {
		co.Clean()
		cleanup()
		delete(testfiles.Files, config.Branch)
		delete(testfiles.Files, syncTag)
	}
}

var TestConfig git.Config = git.Config{
	Branch:    "master",
	UserName:  "example",
	UserEmail: "example@example.com",
	NotesRef:  "fluxtest",
}

var testSyncTag = "sync"

// Checkout makes a standard repo, clones it, and returns the clone
// with a cleanup function.
func Checkout(t *testing.T) (*git.Checkout, func()) {
	checkout, _, cleanup := CheckoutWithConfig(t, TestConfig, testSyncTag)
	return checkout, cleanup
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stderr = ioutil.Discard
	c.Stdout = ioutil.Discard
	return c.Run()
}
