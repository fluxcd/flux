package gittest

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"context"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

func TestCheckout(t *testing.T) {
	repo, cleanup := Repo(t)
	defer cleanup()

	ctx := context.Background()

	params := git.Config{
		UserName:  "example",
		UserEmail: "example@example.com",
		SyncTag:   "flux-test",
		NotesRef:  "fluxtest",
	}
	checkout, err := repo.Clone(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	defer checkout.Clean()

	// We don't expect any notes in the clone, yet. Make sure we get
	// no note, rather than an error.
	head, err := checkout.HeadRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	note, err := checkout.GetNote(ctx, head)
	if err != nil {
		t.Error(err)
	}
	if note != nil {
		t.Errorf("Expected no note on head revision; got %#v", note)
	}

	// Make a working clone and push changes back; then make sure they
	// are visible in the original repo
	working, err := checkout.WorkingClone(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer working.Clean()

	changedFile := ""
	for file, _ := range testfiles.Files {
		path := filepath.Join(working.ManifestDir(), file)
		if err := ioutil.WriteFile(path, []byte("FIRST CHANGE"), 0666); err != nil {
			t.Fatal(err)
		}
		changedFile = file
		break
	}
	if err := working.CommitAndPush(ctx, "Changed file", nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(working.ManifestDir(), changedFile)
	if err := ioutil.WriteFile(path, []byte("SECOND CHANGE"), 0666); err != nil {
		t.Fatal(err)
	}
	// An example note with some of the fields filled in, so we can test
	// serialization a bit.
	expectedNote := git.Note{
		JobID: job.ID("jobID1234"),
		Spec: update.Spec{
			Type: update.Images,
			Spec: update.ReleaseSpec{},
		},
		Result: update.Result{
			flux.ServiceID("service1"): update.ServiceResult{
				Status: update.ReleaseStatusFailed,
				Error:  "failed the frobulator",
			},
		},
	}
	if err := working.CommitAndPush(ctx, "Changed file again", &expectedNote); err != nil {
		t.Fatal(err)
	}

	check := func(c *git.Checkout) {
		contents, err := ioutil.ReadFile(filepath.Join(c.ManifestDir(), changedFile))
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != "SECOND CHANGE" {
			t.Error("contents in checkout are not what we committed")
		}
		rev, err := c.HeadRevision(ctx)
		if err != nil {
			t.Fatal(err)
		}
		note, err := c.GetNote(ctx, rev)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(*note, expectedNote) {
			t.Errorf("note is not what we supplied when committing: %#v", note)
		}
	}

	// Do we see the changes if we pull into the original checkout?
	if err := checkout.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	check(checkout)

	// Do we see the changes if we clone again?
	anotherCheckout, err := repo.Clone(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	defer anotherCheckout.Clean()
	check(checkout)
}
