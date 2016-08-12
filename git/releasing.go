package git

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Release clones, updates the config, deploys the release, commits, and pushes
// the result.
func (r Repo) Release(
	logf func(namespace, service, format string, args ...interface{}),
	platform *kubernetes.Cluster,
	namespace string,
	serviceNames []string,
	candidate registry.Image,
	updatePeriod time.Duration,
) error {
	// Check out latest version of config repo.
	for _, s := range serviceNames {
		logf(namespace, s, "fetching config repo")
	}
	configPath, err := clone(r.Key, r.URL)
	if err != nil {
		return fmt.Errorf("clone of config repo failed: %v", err)
	}
	defer os.RemoveAll(configPath)

	// Find the relevant resource definition file.
	// TODO: need to make sure these belong to the specified service.
	files, err := findFilesFor(configPath, r.Path, candidate.Repository())
	if err != nil {
		return fmt.Errorf("couldn't find a resource definition file: %v", err)
	}

	updatedFiles := map[string]string{}
	for _, file := range files {
		// TODO: This should actually check all containers, as there may be
		// multiple versions of the same image in use.
		// Special case: will this actually result in an update?
		if fileContains(file, candidate.String()) {
			continue
		}

		// Mutate the file so it points to the right image.
		// TODO(pb): should validate file contents are what we expect.
		if err := configUpdate(file, candidate.String()); err != nil {
			return fmt.Errorf("config update failed: %v", err)
		}

		updatedFiles[file] = serviceNameFromFile(file)
	}

	if len(updatedFiles) == 0 {
		return fmt.Errorf("%s already set; no release necessary", candidate.String())
	}

	// Commit the mutated file.
	if err := commit(configPath, "Deployment of "+candidate.String()); err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}

	// Release each changed service
	// TODO: This should be each service, not each file, as they may not be 1-1
	for file, serviceName := range updatedFiles {
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("couldn't read the resource definition file %s: %v", file, err)
		}
		logf(namespace, serviceName, "starting release...")
		err = platform.Release(namespace, serviceName, buf, updatePeriod)
		if err != nil {
			return fmt.Errorf("release failed: %v", err)
		}
		logf(namespace, serviceName, "release complete")
	}

	// Push the new commit.
	if err := push(r.Key, configPath); err != nil {
		return fmt.Errorf("push failed: %v", err)
	}
	for file, serviceName := range updatedFiles {
		logf(namespace, serviceName, "committed and pushed the resource definition file %s", file)
		logf(namespace, serviceName, "service release succeeded")
	}

	return nil
}

func cmd(dir, repoKey string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Env = env(repoKey)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func env(repoKey string) []string {
	base := `GIT_SSH_COMMAND=ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`
	if repoKey == "" {
		return []string{base}
	}
	return []string{fmt.Sprintf("%s -i %q", base, repoKey)}
}

func clone(repoKey, repoURL string) (path string, err error) {
	dst, err := ioutil.TempDir(os.TempDir(), "fluxy-gitclone")
	if err != nil {
		return "", err
	}

	if err := cmd("", repoKey, "clone", repoURL, dst).Run(); err != nil {
		os.RemoveAll(dst)
		return "", fmt.Errorf("git clone: %v", err)
	}

	return dst, nil
}

func findFilesFor(basePath, repoPath, imageStr string) (res []string, err error) {
	filepath.Walk(filepath.Join(basePath, repoPath), func(tgt string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(tgt)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		if !fileContains(tgt, imageStr) {
			return nil
		}
		res = append(res, tgt)
		return nil
	})
	if len(res) == 0 {
		return nil, errors.New("no matching file found")
	}
	return res, nil
}

func fileContains(filename string, s string) bool {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return false
	}
	if strings.Contains(string(buf), s) {
		return true
	}
	return false
}

func serviceNameFromFile(filename string) (string, error) {
	ioutil.ReadFile(filename)
	yaml.Parse()
	filepath.Walk(filepath.Join(basePath, repoPath), func(tgt string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(tgt)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		if !fileContains(tgt, imageStr) {
			return nil
		}
		res = append(res, tgt)
		return nil
	})
	if len(res) == 0 {
		return nil, errors.New("no matching file found")
	}
	return res, nil
}

func configUpdate(file string, newImage string) error {
	fi, err := os.Stat(file)
	if err != nil {
		return err
	}
	def, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	newdef, err := kubernetes.UpdatePodController(def, newImage, ioutil.Discard)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(file, newdef, fi.Mode()); err != nil {
		return err
	}
	return nil
}

func commit(workingDir, commitMessage string) error {
	for _, c := range [][]string{
		{"-c", "user.name=Weave Flux", "-c", "user.email=support@weave.works", "commit", "--no-verify", "-a", "-m", commitMessage},
	} {
		if err := cmd(workingDir, "", c...).Run(); err != nil {
			return fmt.Errorf("%s: %v", strings.Join(c, " "), err)
		}
	}
	return nil
}

func push(repoKey, workingDir string) error {
	for _, c := range [][]string{
		{"push", "origin", "master"},
	} {
		if err := cmd(workingDir, repoKey, c...).Run(); err != nil {
			return fmt.Errorf("%s: %v", strings.Join(c, " "), err)
		}
	}
	return nil
}
