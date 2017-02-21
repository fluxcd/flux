package kubernetes

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weaveworks/flux"
)

// FindDefinedServices finds all the services defined under the
// directory given, and returns a map of service IDs (from its
// specified namespace and name) to the paths of resource definition
// files.
func FindDefinedServices(path string) (map[flux.ServiceID][]string, error) {
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

	var files []string
	filepath.Walk(path, func(target string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
			files = append(files, target)
		}
		return nil
	})

	services := map[flux.ServiceID][]string{}
	for _, file := range files {
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, "./"+filepath.Base(file)) // due to bug (?) in kubeservice
		cmd.Dir = filepath.Dir(file)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			continue
		}
		for _, out := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
			if out != "" {
				id := flux.ServiceID(out)
				services[id] = append(services[id], file)
			}
		}
	}
	return services, nil
}
