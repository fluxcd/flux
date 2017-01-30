package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/instance"
)

func TestCloneCommitAndPush(t *testing.T) {
	r, cleanup := setupRepo(t)
	defer cleanup()
	inst := &instance.Instance{Repo: r}
	ctx := NewReleaseContext(inst)
	defer ctx.Clean()

	if err := ctx.CloneRepo(); err != nil {
		t.Fatal(err)
	}

	out, err := ctx.CommitAndPush("No changes!")
	if err != nil {
		t.Error(err)
	}
	if out == "" {
		t.Errorf("Expected no-op message back from git")
	}

	// change a file and try again
	for name, _ := range files {
		if err = execCommand("rm", filepath.Join(ctx.WorkingDir, name)); err != nil {
			t.Fatal(err)
		}
		break
	}
	out, err = ctx.CommitAndPush("Removed file")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("Expected no warning from CommitAndPush but got: %q", out)
	}
}

func TestLockedServices(t *testing.T) {
	conf := instance.Config{
		Services: map[flux.ServiceID]instance.ServiceConfig{
			flux.ServiceID("service1"): instance.ServiceConfig{
				Locked: true,
			},
			flux.ServiceID("service2"): instance.ServiceConfig{
				Locked:    true,
				Automated: true,
			},
			flux.ServiceID("service3"): instance.ServiceConfig{
				Automated: true,
			},
		},
	}

	locked := LockedServices(conf)
	if !locked.Contains(flux.ServiceID("service1")) {
		t.Error("service1 locked in config but not reported as locked")
	}
	if !locked.Contains(flux.ServiceID("service2")) {
		t.Error("service2 locked in config but not reported as locked")
	}
	if locked.Contains(flux.ServiceID("service3")) {
		t.Error("service3 not locked but reported as locked")
	}
}

func setupRepo(t *testing.T) (git.Repo, func()) {
	newDir, err := ioutil.TempDir(os.TempDir(), "flux-testrepo")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if strings.HasPrefix(newDir, os.TempDir()) {
			execCommand("rm", "-rf", newDir)
			println("Deleted " + newDir)
		} else {
			println("Refusing to delete " + newDir)
		}
	}

	filesDir := filepath.Join(newDir, "files")
	gitDir := filepath.Join(newDir, "git")
	if err := execCommand("mkdir", filesDir); err != nil {
		t.Fatal(err)
	}

	if err = execCommand("git", "-C", filesDir, "init"); err != nil {
		cleanup()
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(filesDir, name)
		ioutil.WriteFile(path, []byte(content), 0666)
		if err = execCommand("git", "-C", filesDir, "add", name); err != nil {
			cleanup()
			t.Fatal(err)
		}
	}
	if err = execCommand("git", "-C", filesDir, "commit", "-m", "'Initial revision'"); err != nil {
		cleanup()
		t.Fatal(err)
	}

	if err = execCommand("git", "clone", "--bare", filesDir, gitDir); err != nil {
		t.Fatal(err)
	}

	return git.Repo{
		URL:    gitDir,
		Branch: "master",
	}, cleanup
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	fmt.Printf("exec: %s %s\n", cmd, strings.Join(args, " "))
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	return c.Run()
}

// ----- DATA

var files = map[string]string{
	"helloworld-deploy.yaml": `apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: helloworld
spec:
  minReadySeconds: 1
  replicas: 5
  template:
    metadata:
      labels:
        name: helloworld
    spec:
      containers:
      - name: helloworld
        image: quay.io/weaveworks/helloworld:master-a000001
        args:
        - -msg=Ahoy
        ports:
        - containerPort: 80
      - name: sidecar
        image: quay.io/weaveworks/sidecar:master-a000002
        args:
        - -addr=:8080
        ports:
        - containerPort: 8080
`,
	"helloworld-svc.yaml": `apiVersion: v1
kind: Service
metadata:
  name: helloworld
spec:
  ports:
    - port: 80
  selector:
    name: helloworld
`,
}
