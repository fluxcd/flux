package git

import (
	"context"
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

	exportHead, err := refRevision(ctx, export.dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if headMinusOne != exportHead {
		t.Errorf("exported %s, but head in export dir %s is %s", headMinusOne, export.dir, exportHead)
	}
}
