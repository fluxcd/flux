package chartsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func updateDependencies(chartDir string) error {
	var hasLockFile bool

	// We are going to use `helm dep build`, which tries to update the
	// dependencies in charts/ by looking at the file
	// `requirements.lock` in the chart directory. If the lockfile
	// does not match what is specified in requirements.yaml, it will
	// error out.
	//
	// If that file doesn't exist, `helm dep build` will fall back on
	// `helm dep update`, which populates the charts/ directory _and_
	// creates the lockfile. So that it will have the same behaviour
	// the next time it attempts a release, remove the lockfile if it
	// was created by helm.
	lockfilePath := filepath.Join(chartDir, "requirements.lock")
	info, err := os.Stat(lockfilePath)
	hasLockFile = (err == nil && !info.IsDir())
	if !hasLockFile {
		defer os.Remove(lockfilePath)
	}

	cmd := exec.Command("helm", "repo", "update")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not update repo: %s", string(out))
	}

	cmd = exec.Command("helm", "dep", "build", ".")
	cmd.Dir = chartDir

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not update dependencies in %s: %s", chartDir, string(out))
	}

	return nil
}
