package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform/kubernetes/testfiles"
)

func TestCloneCommitAndPush(t *testing.T) {
	r, cleanup := gittest.Repo(t)
	defer cleanup()
	inst := &instance.Instance{Repo: r}
	ctx := NewReleaseContext(inst)
	defer ctx.Clean()

	if err := ctx.CloneRepo(); err != nil {
		t.Fatal(err)
	}

	err := ctx.CommitAndPush("No changes!")
	if err != git.ErrNoChanges {
		t.Errorf("expected ErrNoChanges, got %s", err)
	}

	// change a file and try again
	for name, _ := range testfiles.Files {
		if err = execCommand("rm", filepath.Join(ctx.WorkingDir, name)); err != nil {
			t.Fatal(err)
		}
		break
	}
	err = ctx.CommitAndPush("Removed file")
	if err != nil {
		t.Fatal(err)
	}
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	fmt.Printf("exec: %s %s\n", cmd, strings.Join(args, " "))
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	return c.Run()
}
