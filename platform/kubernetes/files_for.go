package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FilesFor returns the resource definition files in path (or any subdirectory)
// that are responsible for driving the given namespace/service. It presumes
// kubeservice is available in the PWD or PATH.
func FilesFor(path, namespace, service string) (filenames []string, err error) {
	bin, err := func() (string, error) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		localBin := filepath.Join(cwd, "kubeservice")
		if _, err := os.Stat(localBin); err == nil {
			return localBin, nil
		}
		if pathBin, err := exec.LookPath("kubeservice"); err == nil {
			return pathBin, nil
		}
		return "", errors.New("kubeservice not found")
	}()
	if err != nil {
		return nil, err
	}

	var candidates []string
	filepath.Walk(path, func(target string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
			candidates = append(candidates, target)
		}
		return nil
	})

	tgt := fmt.Sprintf("%s/%s", namespace, service)
	var winners []string
	for _, file := range candidates {
		var stdout bytes.Buffer
		cmd := exec.Command(bin, "./"+filepath.Base(file)) // due to bug (?) in kubeservice
		cmd.Dir = filepath.Dir(file)
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			continue
		}
		for _, out := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
			if out == tgt { // kubeservice output is "namespace/service", same as ServiceID
				winners = append(winners, file)
				break
			}
		}
	}

	return winners, nil
}
