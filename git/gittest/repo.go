package gittest

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
)

// Repo creates a new clone-able git repo, pre-populated with some kubernetes
// files and a few commits. Also returns a cleanup func to clean up after.
func Repo(t *testing.T) (git.Repo, func()) {
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

	return git.Repo{
		GitRemoteConfig: flux.GitRemoteConfig{
			URL:    gitDir,
			Branch: "master",
		},
	}, cleanup
}

func Checkout(t *testing.T) (*git.Checkout, func()) {
	repo, cleanup := Repo(t)
	config := git.Config{
		UserName:  "example",
		UserEmail: "example@example.com",
		SyncTag:   "flux-test",
		NotesRef:  "fluxtest",
	}
	co, err := repo.Clone(config)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	return co, func() {
		co.Clean()
		cleanup()
	}
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stderr = ioutil.Discard
	c.Stdout = ioutil.Discard
	return c.Run()
}
