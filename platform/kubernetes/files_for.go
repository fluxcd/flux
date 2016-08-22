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
		if _, err := os.Stat("./kubeservice"); err == nil {
			return "./kubeservice", nil
		}
		if bin, err := exec.LookPath("kubeservice"); err == nil {
			return bin, nil
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
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, "./"+filepath.Base(file)) // due to bug (?) in kubeservice
		cmd.Dir = filepath.Dir(file)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			continue
		}
		out := strings.TrimSpace(stdout.String())
		if out == tgt { // kubeservice output is "namespace/service", same as ServiceID
			winners = append(winners, file)
		}
	}

	if len(winners) <= 0 {
		return nil, fmt.Errorf("no file found for namespace %s service %s", namespace, service)
	}
	return winners, nil
}
