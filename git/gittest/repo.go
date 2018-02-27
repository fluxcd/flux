package gittest

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"context"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
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

	return git.NewRepo(git.Remote{
		URL: "file://" + gitDir,
	}), cleanup
}

func Checkout(t *testing.T) (*git.Checkout, func()) {
	repo, cleanup := Repo(t)
	shutdown, wg := make(chan struct{}), &sync.WaitGroup{}
	wg.Add(1)
	go repo.Start(shutdown, wg)
	WaitForRepoReady(repo, t)

	config := git.Config{
		Branch:    "master",
		UserName:  "example",
		UserEmail: "example@example.com",
		SyncTag:   "flux-test",
		NotesRef:  "fluxtest",
	}
	co, err := repo.Clone(context.Background(), config)
	if err != nil {
		close(shutdown)
		cleanup()
		t.Fatal(err)
	}
	return co, func() {
		close(shutdown)
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

func WaitForRepoReady(r *git.Repo, t *testing.T) {
	retries := 5
	for {
		s, err := r.Status()
		if err != nil {
			t.Fatal(err)
		}
		if s == flux.RepoReady {
			return
		}
		if retries == 0 {
			t.Fatalf("repo was not ready after 5 seconds")
			return
		}
		retries--
		time.Sleep(100 * time.Millisecond)
	}
}
