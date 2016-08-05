package automator

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weaveworks/fluxy/platform/kubernetes"
)

const gitEnvTmpl = `GIT_SSH_COMMAND=ssh -i %q -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`

func gitClone(repoKey, repoURL string) (path string, err error) {
	dst, err := ioutil.TempDir(os.TempDir(), "fluxy-automator-gitclone")
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "clone", repoURL, dst)
	cmd.Env = []string{fmt.Sprintf(gitEnvTmpl, repoKey)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dst)
		return "", fmt.Errorf("git clone: %v", err)
	}

	return dst, nil
}

func findFileFor(basePath, repoPath, imageStr string) (res string, err error) {
	filepath.Walk(filepath.Join(basePath, repoPath), func(tgt string, info os.FileInfo, err error) error {
		if !info.IsDir() && fileContains(tgt, imageStr) {
			res = tgt
			return errors.New("found; stopping")
		}
		return nil
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

func gitCommitAndPush(repoKey, workingDir, commitMessage string) error {
	for _, c := range [][]string{
		{"git", "-c", "user.name=Weave Flux", "-c", "user.email=support@weave.works", "commit", "-a", "-m", commitMessage},
		{"git", "push", "origin", "master"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = workingDir
		cmd.Env = []string{fmt.Sprintf(gitEnvTmpl, repoKey)}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %v", strings.Join(c, " "), err)
		}
	}
	return nil
}
