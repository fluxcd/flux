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

// DefinedServices returns a list of all defined services, and their relevant
// resource definition files in path (or in any subdirectory). It presumes
// kubeservice is available in the PWD or PATH.
func DefinedServices(path string) (filenames map[flux.ServiceID][]string, err error) {
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
	filepath.Walk(path, func(target string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
			files = append(files, target)
		}
		return nil
	})

	services := map[flux.ServiceID][]string{}
	for _, file := range files {
		var stdout bytes.Buffer
		cmd := exec.Command(bin, "./"+filepath.Base(file)) // due to bug (?) in kubeservice
		cmd.Dir = filepath.Dir(file)
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			continue
		}
		for _, out := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
			id := flux.ServiceID(out)
			services[id] = append(services[id], file)
		}
	}

	return services, nil
}
