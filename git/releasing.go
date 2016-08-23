package git

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Release clones, updates the config, deploys the release, commits, and pushes
// the result.
func (r Repo) Release(
	logf func(format string, args ...interface{}),
	platform *kubernetes.Cluster,
	namespace string,
	service string,
	candidate registry.Image,
) error {
	// Check out latest version of config repo.
	logf("fetching config repo")

	working, err := ioutil.TempDir(os.TempDir(), "fluxy-gitclone")
	if err != nil {
		return err
	}
	defer func(d string) {
		os.RemoveAll(d)
	}(working)

	// If the private key is from a mounted k8s secret, it will have
	// the wrong permissions; copy it to the working dir and give it
	// the right permissions
	keyPath, err := copyKey(working, r.Key)
	if err != nil {
		return err
	}

	configPath, err := clone(working, keyPath, r.URL)
	if err != nil {
		return fmt.Errorf("clone of config repo failed: %v", err)
	}

	// Find the relevant resource definition file.
	file, err := findFileFor(configPath, r.Path, candidate.Repository())
	if err != nil {
		return fmt.Errorf("couldn't find a resource definition file: %v", err)
	}

	// Special case: will this actually result in an update?
	if fileContains(file, candidate.String()) {
		return fmt.Errorf("%s already set to %s; no release necessary", filepath.Base(file), candidate.String())
	}

	// Mutate the file so it points to the right image.
	// TODO(pb): should validate file contents are what we expect.
	if err := configUpdate(file, candidate.String()); err != nil {
		return fmt.Errorf("config update failed: %v", err)
	}

	// Commit the mutated file.
	if err := commit(configPath, "Deployment of "+candidate.String()); err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}

	// Make the release.
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("couldn't read the resource definition file: %v", err)
	}
	logf("starting release...")
	err = platform.Release(namespace, service, buf)
	if err != nil {
		return fmt.Errorf("release failed: %v", err)
	}
	logf("release complete")

	// Push the new commit.
	if err := push(keyPath, configPath); err != nil {
		return fmt.Errorf("push failed: %v", err)
	}
	logf("committed and pushed the resource definition file %s", file)

	logf("service release succeeded")
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

func clone(workingDir, repoKey, repoURL string) (path string, err error) {
	repoPath := filepath.Join(workingDir, "repo")
	if err := cmd("", repoKey, "clone", repoURL, repoPath).Run(); err != nil {
		return "", fmt.Errorf("git clone: %v", err)
	}
	return repoPath, nil
}

func findFileFor(basePath, repoPath, imageStr string) (res string, err error) {
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
		res = tgt
		return errors.New("found; stopping")
	})
	if res == "" {
		return "", errors.New("no matching file found")
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

func copyKey(working, key string) (string, error) {
	keyPath := filepath.Join(working, "id-rsa")
	f, err := ioutil.ReadFile(key)
	if err == nil {
		err = ioutil.WriteFile(keyPath, f, 0400)
	}
	return keyPath, err
}
