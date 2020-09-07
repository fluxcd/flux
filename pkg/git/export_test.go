package git

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
)

func TestExportAtRevision(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := createRepo(newDir, []string{"config"})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(Remote{URL: newDir}, ReadOnly)
	defer repo.Clean()
	if err := repo.Ready(ctx); err != nil {
		t.Fatal(err)
	}

	headMinusOne, err := repo.Revision(ctx, "HEAD^1")
	if err != nil {
		t.Fatal(err)
	}

	export, err := repo.Export(ctx, headMinusOne)
	if err != nil {
		t.Fatal(err)
	}
	defer export.Clean()

	exportHead, err := refRevision(ctx, export.dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if headMinusOne != exportHead {
		t.Errorf("exported %s, but head in export dir %s is %s", headMinusOne, export.dir, exportHead)
	}
}

func TestExportFailsCheckout(t *testing.T) {
	newDir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := createRepo(newDir, []string{"config"})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(Remote{URL: newDir}, ReadOnly)
	defer repo.Clean()
	if err := repo.Ready(ctx); err != nil {
		t.Fatal(err)
	}

	findWorkingDirs := func() map[string]bool {
		entries, err := ioutil.ReadDir(os.TempDir())
		if err != nil {
			t.Fatalf("ioutil.ReadDir(%q)=%v, want no error", os.TempDir(), err)
		}

		found := make(map[string]bool, len(entries))
		for _, e := range entries {
			// Makes an assumption about the location a repo export will happen.
			if strings.HasPrefix(e.Name(), "flux-working") {
				found[e.Name()] = true
			}
		}
		return found
	}

	prior := findWorkingDirs()

	// Try to check out a revision that does not exist:
	export, err := repo.Export(ctx, "0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("want repo.Export(\"0000000000000000000000000000000000000000\") to fail, succeeded instead")
	}
	if export != nil {
		t.Fatalf("repo.Export(\"0000000000000000000000000000000000000000\")=%+v, want nill", export)
	}

	for d := range findWorkingDirs() {
		if !prior[d] {
			t.Errorf("Found %q in %q - failed to cleanup when returning an error", d, os.TempDir())
		}
	}
}
