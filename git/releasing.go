package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// check for changes
func check(workingDir, subdir string) bool {
	diff := cmd(workingDir, "", "diff", "--quiet", "--", subdir)
	// `--quiet` means "exit with 1 if there are changes"
	return diff.Run() != nil
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
