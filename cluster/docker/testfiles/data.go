package testfiles

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TempDir(t *testing.T) (string, func()) {
	newDir, err := ioutil.TempDir(os.TempDir(), "flux-test")
	if err != nil {
		t.Fatal("failed to create temp directory")
	}

	cleanup := func() {
		if strings.HasPrefix(newDir, os.TempDir()) {
			if err = exec.Command("rm", "-rf", newDir).Run(); err == nil {
				println("Deleted " + newDir)
			} else {
				println("Failed to delete " + newDir)
			}
		} else {
			println("Refusing to delete " + newDir)
		}
	}
	return newDir, cleanup
}
func WriteTestFiles(dir string) error {
	for name, content := range Files {
		path := filepath.Join(dir, name)
		if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
			return err
		}
	}
	return nil
}

var Files = map[string]string{
	"docker-compose-front-end.yml": `version: "3"
services:
  front-end:
    environment:
    - reschedule=on-node-failure
    image: index.docker.io/weaveworksdemos/front-end:blue-buttons
    ports:
    - 80:8079
`,
	"docker-compose-carts.yaml": `version: "3"
services:
  carts:
    environment:
    - reschedule=on-node-failure
    - JAVA_OPTS=-Xms64m -Xmx128m -XX:PermSize=32m -XX:MaxPermSize=64m -XX:+UseG1GC
      -Djava.security.egd=file:/dev/urandom
    image: index.docker.io/weaveworksdemos/carts:master-4a0d0780
`,
}
