package flux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func filesFor(path string, id ServiceID) (filenames []string, err error) {
	bin, err := func() (string, error) {
		if _, err := os.Stat("./kubeservice"); err == nil {
			return "./kubeservice", nil
		}
		bin, err := exec.LookPath("kubeservice")
		if err == nil {
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
		if out == string(id) { // kubeservice output is "namespace/service", same as ServiceID
			winners = append(winners, file)
		}
	}

	if len(winners) <= 0 {
		return nil, fmt.Errorf("no file found for service %s", id)
	}
	return winners, nil
}
