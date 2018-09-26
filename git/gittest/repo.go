package gittest

import (
	"context"
	"io/ioutil"
	"io"
	"os/exec"
	"path/filepath"
	"testing"
	"bytes"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
)

// GPGKey creates a new, temporary GPG home directory and a public/private key
// pair. It returns the GPG home directory, the ID of the created key, and a
// cleanup function to be called after the caller is finished with this key.
// Since GPG uses /dev/random, this may block while waiting for entropy to
// become available.
func GPGKey(t *testing.T) (string, string, func()) {
	newDir, cleanup := testfiles.TempDir(t)

	cmd := exec.Command("gpg", "--homedir", newDir, "--batch", "--gen-key")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	io.WriteString(stdin, "Key-Type: DSA\n")
	io.WriteString(stdin, "Key-Length: 1024\n")
	io.WriteString(stdin, "Key-Usage: sign\n")
	io.WriteString(stdin, "Name-Real: Weave Flux\n")
	io.WriteString(stdin, "Name-Email: flux@weave.works\n")
	io.WriteString(stdin, "%no-protection\n")
	stdin.Close()

	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	gpgCmd := exec.Command("gpg", "--homedir", newDir, "--list-keys", "--with-colons")
	grepCmd := exec.Command("grep", "^fpr")
	cutCmd := exec.Command("cut", "-d:", "-f10")

	grepIn, gpgOut := io.Pipe()
	cutIn, grepOut := io.Pipe()
	var cutOut bytes.Buffer

	gpgCmd.Stdout = gpgOut
	grepCmd.Stdin, grepCmd.Stdout = grepIn, grepOut
	cutCmd.Stdin, cutCmd.Stdout = cutIn, &cutOut

	gpgCmd.Start()
	grepCmd.Start()
	cutCmd.Start()

	if err := gpgCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	gpgOut.Close()

	if err := grepCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	grepOut.Close()

	if err := cutCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	fingerprint := strings.TrimSpace(cutOut.String())
	return newDir, fingerprint, cleanup
}

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
	})
	return mirror, func() {
		mirror.Clean()
		cleanup()
	}
}

// Workloads is a shortcut to getting the names of the workloads (NB
// not all resources, just the workloads) represented in the test
// files.
func Workloads() (res []flux.ResourceID) {
	for k, _ := range testfiles.ServiceMap("") {
		res = append(res, k)
	}
	return res
}

// CheckoutWithConfig makes a standard repo, clones it, and returns
// the clone, the original repo, and a cleanup function.
func CheckoutWithConfig(t *testing.T, config git.Config) (*git.Checkout, *git.Repo, func()) {
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
	}
}

var TestConfig git.Config = git.Config{
	Branch:    "master",
	UserName:  "example",
	UserEmail: "example@example.com",
	SyncTag:   "flux-test",
	NotesRef:  "fluxtest",
}

// Checkout makes a standard repo, clones it, and returns the clone
// with a cleanup function.
func Checkout(t *testing.T) (*git.Checkout, func()) {
	checkout, _, cleanup := CheckoutWithConfig(t, TestConfig)
	return checkout, cleanup
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stderr = ioutil.Discard
	c.Stdout = ioutil.Discard
	return c.Run()
}
