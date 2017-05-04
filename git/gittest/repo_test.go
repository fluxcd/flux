package gittest

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
)

func TestCheckout(t *testing.T) {
	repo, cleanup := Repo(t)
	defer cleanup()

	params := git.Config{
		UserName:  "example",
		UserEmail: "example@example.com",
		SyncTag:   "flux-test",
		NotesRef:  "fluxtest",
	}
	checkout, err := repo.Clone(params)
	if err != nil {
		t.Fatal(err)
	}
	defer checkout.Clean()

	// Make a working clone and push changes back; then make sure they
	// are visible in the original repo
	working, err := checkout.WorkingClone()
	if err != nil {
		t.Fatal(err)
	}
	defer working.Clean()

	changedFile := ""
	for file, _ := range testfiles.Files {
		path := filepath.Join(working.ManifestDir(), file)
		if err := ioutil.WriteFile(path, []byte("CHANGED"), 0666); err != nil {
			t.Fatal(err)
		}
		changedFile = file
		break
	}
	if err := working.CommitAndPush("Changed file", "With note"); err != nil {
		t.Fatal(err)
	}

	check := func(c git.Checkout) {
		contents, err := ioutil.ReadFile(filepath.Join(c.ManifestDir(), changedFile))
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != "CHANGED" {
			t.Error("contents in checkout are not what we committed")
		}
		rev, err := c.HeadRevision()
		if err != nil {
			t.Fatal(err)
		}
		note, err := c.GetNote(rev)
		if err != nil {
			t.Error(err)
		}
		if strings.TrimSpace(note) != "With note" {
			t.Error("note is not what we supplied when committing: " + note)
		}
	}

	// Do we see the changes if we pull into the original checkout?
	if err := checkout.Pull(); err != nil {
		t.Fatal(err)
	}
	check(checkout)

	// Do we see the changes if we clone again?
	anotherCheckout, err := repo.Clone(params)
	if err != nil {
		t.Fatal(err)
	}
	defer anotherCheckout.Clean()
	check(checkout)
}
