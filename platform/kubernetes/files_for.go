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
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, "./"+filepath.Base(file)) // due to bug (?) in kubeservice
		cmd.Dir = filepath.Dir(file)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		fmt.Fprintf(os.Stderr, "Args: %v\n", cmd.Args)
		fmt.Fprintf(os.Stderr, "Dir: %s\n", cmd.Dir)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, stdout.String()+"\n")
			fmt.Fprintf(os.Stderr, stderr.String()+"\n")
			fmt.Fprintf(os.Stderr, "kubeservice FAILED: %v\n", err)
			continue
		}
		out := strings.TrimSpace(stdout.String())
		fmt.Fprintf(os.Stderr, "kubeservice SUCCESS: %s -> %s\n", file, out)
		if out == tgt { // kubeservice output is "namespace/service", same as ServiceID
			winners = append(winners, file)
		}
	}

	if len(winners) <= 0 {
		return nil, fmt.Errorf("no file found for namespace %s service %s", namespace, service)
	}
	return winners, nil
}
